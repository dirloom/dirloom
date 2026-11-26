package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"sync"
	"time"

	"github.com/dirloom/dirloom/internal/buildinfo"
	"github.com/dirloom/dirloom/internal/cli"
)

// result is one run of one scenario: a line of the raw results.
type result struct {
	Label      string `json:"label"`
	Scenario   string `json:"scenario"`
	Run        int    `json:"run"`
	Start      string `json:"start"`
	MemLimit   uint64 `json:"memLimitBytes,omitempty"`
	Env        string `json:"env,omitempty"`
	GoVersion  string `json:"goVersion"`
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	GOMAXPROCS int    `json:"gomaxprocs"`

	Exit int `json:"exit"`
	// OK is true when the exit status and the expected output match the
	// scenario: only such runs are measurements.
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// OOM is true when the process stopped because memory ran out.
	OOM bool `json:"oom,omitempty"`

	// Time: the workload in the process (cli.Execute), and the whole process
	// as the runner saw it.
	WallNs     int64 `json:"wallNs"`
	ProcWallNs int64 `json:"procWallNs"`
	UserNs     int64 `json:"userNs"`
	SystemNs   int64 `json:"systemNs"`

	// Go heap, sampled during the workload: the heap in use (live objects
	// and garbage not yet swept, HeapAlloc) and the heap marked live by the
	// last finished collection.
	HeapPeak     uint64 `json:"heapPeakBytes"`
	HeapLivePeak uint64 `json:"heapLivePeakBytes"`
	HeapBase     uint64 `json:"heapBaseBytes"`
	Samples      int    `json:"samples"`
	// Allocations of the workload, as go test -benchmem counts them.
	AllocBytes   uint64 `json:"allocBytes"`
	AllocObjects uint64 `json:"allocObjects"`
	GCCycles     uint32 `json:"gcCycles"`

	// Process: the peak working set (resident memory) and the peak private
	// commit of the dirloom process, and the peak commit of its job, which
	// also holds the Git processes it starts (Windows).
	PeakWorkingSet uint64 `json:"peakWorkingSetBytes"`
	PeakPrivate    uint64 `json:"peakPrivateBytes"`
	JobPeak        uint64 `json:"jobPeakCommitBytes,omitempty"`

	StdoutBytes int64        `json:"stdoutBytes"`
	StdoutHead  string       `json:"stdoutHead,omitempty"`
	StderrHead  string       `json:"stderrHead,omitempty"`
	StderrTail  string       `json:"stderrTail,omitempty"`
	Watch       *watchResult `json:"watch,omitempty"`
}

// watchResult is what a watch run adds.
type watchResult struct {
	StartupNs   int64   `json:"startupNs"`
	Bursts      int     `json:"bursts"`
	Converged   int     `json:"converged"`
	LatenciesNs []int64 `json:"latenciesNs"`
	// RetainedHeap is the heap after a forced collection: before the first
	// burst, then after each convergence.
	RetainedHeap []uint64 `json:"retainedHeapBytes"`
	OpsPerBurst  int      `json:"opsPerBurst"`
	Generations  int      `json:"generations"`
	Errors       int      `json:"errors"`
	Rescans      int      `json:"rescans"`
	Lines        int      `json:"lines"`
	Stopped      bool     `json:"stopped"`
}

// headTail keeps the first and the last bytes written to it.
type headTail struct {
	mu    sync.Mutex
	n     int64
	head  []byte
	tail  []byte
	limit int
}

func (w *headTail) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.n += int64(len(p))
	if room := w.limit - len(w.head); room > 0 {
		w.head = append(w.head, p[:min(room, len(p))]...)
	}
	w.tail = append(w.tail, p...)
	if len(w.tail) > w.limit {
		w.tail = append(w.tail[:0], w.tail[len(w.tail)-w.limit:]...)
	}
	return len(p), nil
}

func (w *headTail) snapshot() (n int64, head, tail string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n, string(w.head), string(w.tail)
}

// sampler reads the Go heap metrics at a fixed period without stopping the
// world.
type sampler struct {
	stop     chan struct{}
	done     chan struct{}
	heapPeak uint64
	livePeak uint64
	samples  int
}

const (
	metricHeapObjects = "/memory/classes/heap/objects:bytes"
	metricHeapLive    = "/gc/heap/live:bytes"
)

func startSampler(period time.Duration) *sampler {
	s := &sampler{stop: make(chan struct{}), done: make(chan struct{})}
	samples := []metrics.Sample{{Name: metricHeapObjects}, {Name: metricHeapLive}}
	read := func() {
		metrics.Read(samples)
		if v := samples[0].Value; v.Kind() == metrics.KindUint64 {
			s.heapPeak = max(s.heapPeak, v.Uint64())
		}
		if v := samples[1].Value; v.Kind() == metrics.KindUint64 {
			s.livePeak = max(s.livePeak, v.Uint64())
		}
		s.samples++
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(period)
		defer ticker.Stop()
		for {
			read()
			select {
			case <-s.stop:
				read()
				return
			case <-ticker.C:
			}
		}
	}()
	return s
}

func (s *sampler) finish() {
	close(s.stop)
	<-s.done
}

// runChild runs one scenario in this process and writes its result as JSON
// on stdout. The runner starts it with -wait, puts it in a job, then
// releases it through stdin.
func runChild(args []string) error {
	flags := flag.NewFlagSet("exec", flag.ContinueOnError)
	root := flags.String("fixtures", "", "directory of the fixtures")
	name := flags.String("scenario", "", "scenario to run")
	wait := flags.Bool("wait", false, "wait for a line on stdin before running")
	period := flags.Duration("sample", time.Millisecond, "heap sampling period")
	if err := flags.Parse(args); err != nil {
		return err
	}
	sc, err := findScenario(*name)
	if err != nil {
		return err
	}
	if *wait {
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
	}
	res := result{
		Scenario: sc.Name, GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		GOMAXPROCS: runtime.GOMAXPROCS(0),
	}
	stdout := &headTail{limit: 1024}
	stderr := &headTail{limit: 1024}
	args = sc.args(*root)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	s := startSampler(*period)
	start := time.Now()
	if sc.Watch != nil {
		params := *sc.Watch
		// A shorter watch, to check the scenario quickly; never for a
		// baseline.
		if text := os.Getenv("PERFBASELINE_WATCH_DURATION"); text != "" {
			if params.Duration, err = time.ParseDuration(text); err != nil {
				return err
			}
		}
		res.Watch, res.Exit = driveWatch(args, params, filepath.Join(*root, "live-watch"), stdout, stderr)
	} else {
		res.Exit = cli.Execute(context.Background(), args, stdout, stderr, buildinfo.ResolvedVersion())
	}
	res.WallNs = time.Since(start).Nanoseconds()
	s.finish()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	res.HeapPeak, res.HeapLivePeak, res.HeapBase, res.Samples = s.heapPeak, s.livePeak, before.HeapAlloc, s.samples
	res.AllocBytes = after.TotalAlloc - before.TotalAlloc
	res.AllocObjects = after.Mallocs - before.Mallocs
	res.GCCycles = after.NumGC - before.NumGC
	if res.PeakWorkingSet, res.PeakPrivate, err = selfPeaks(); err != nil {
		res.Error = err.Error()
	}
	var head string
	res.StdoutBytes, head, _ = stdout.snapshot()
	res.StdoutHead = head
	_, _, res.StderrTail = stderr.snapshot()
	return json.NewEncoder(os.Stdout).Encode(res)
}

// driveWatch runs the watch command in this process, as cmd/dirloom does,
// and changes the watched tree in bursts. Each burst ends by creating a
// sentinel file; since every observation scans the whole tree, the first
// state.changed that adds the sentinel holds the whole burst, and its
// arrival is the burst's convergence.
func driveWatch(args []string, params watchParams, tree string, stdout, stderr *headTail) (*watchResult, int) {
	res := &watchResult{}
	bursts := filepath.Join(tree, "bursts")
	if err := resetBursts(bursts); err != nil {
		_, _ = fmt.Fprintf(stderr, "perfbaseline: %v\n", err)
		return res, -1
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	exit := make(chan int, 1)
	start := time.Now()
	go func() {
		code := cli.Execute(ctx, args, writer, stderr, buildinfo.ResolvedVersion())
		_ = writer.Close()
		exit <- code
	}()

	started := make(chan struct{})
	converged := make(chan time.Time, 1)
	var mu sync.Mutex
	sentinel := ""
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 1<<16), 1<<24)
		seen, first := false, true
		for scanner.Scan() {
			line := scanner.Bytes()
			_, _ = stdout.Write(line)
			_, _ = stdout.Write([]byte{'\n'})
			var event struct {
				Event string `json:"event"`
				Path  string `json:"path"`
			}
			if json.Unmarshal(line, &event) != nil {
				continue
			}
			res.Lines++
			switch event.Event {
			case "watch.started":
				if first {
					first = false
					close(started)
				}
			case "node.added":
				mu.Lock()
				if sentinel != "" && event.Path == sentinel {
					seen = true
				}
				mu.Unlock()
			case "state.changed":
				res.Generations++
				if seen {
					seen = false
					select {
					case converged <- time.Now():
					default: // the driver gave up on this burst
					}
				}
			case "watch.error":
				res.Errors++
			case "rescan.started":
				res.Rescans++
			case "watch.stopped":
				res.Stopped = true
			}
		}
		_, _ = io.Copy(io.Discard, reader)
	}()

	stopWith := func(code int) (*watchResult, int) {
		cancel()
		select {
		case code = <-exit:
		case <-time.After(30 * time.Second):
			_, _ = fmt.Fprintln(stderr, "perfbaseline: the watch did not stop within 30 s")
			code = -1
		}
		<-readerDone
		if err := resetBursts(bursts); err != nil {
			_, _ = fmt.Fprintf(stderr, "perfbaseline: %v\n", err)
		}
		return res, code
	}

	select {
	case <-started:
		res.StartupNs = time.Since(start).Nanoseconds()
	case code := <-exit:
		<-readerDone
		return res, code
	case <-time.After(2 * time.Minute):
		_, _ = fmt.Fprintln(stderr, "perfbaseline: no watch.started within 2 minutes")
		return stopWith(-1)
	}
	res.RetainedHeap = append(res.RetainedHeap, retainedHeap())

	first := time.Now()
	for k := 1; time.Since(first)+params.Interval <= params.Duration; k++ {
		time.Sleep(time.Until(first.Add(time.Duration(k) * params.Interval)))
		name := fmt.Sprintf("s%04d", k)
		mu.Lock()
		sentinel = "bursts/" + name
		mu.Unlock()
		ops, t0, err := burst(bursts, k, params.Files, name)
		res.Bursts++
		res.OpsPerBurst = ops
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "perfbaseline: burst %d: %v\n", k, err)
			return stopWith(-1)
		}
		select {
		case at := <-converged:
			res.Converged++
			res.LatenciesNs = append(res.LatenciesNs, at.Sub(t0).Nanoseconds())
		case code := <-exit:
			<-readerDone
			_, _ = fmt.Fprintf(stderr, "perfbaseline: the watch exited (%d) during burst %d\n", code, k)
			return res, code
		case <-time.After(60 * time.Second):
			_, _ = fmt.Fprintf(stderr, "perfbaseline: burst %d did not converge within 60 s\n", k)
			return stopWith(-1)
		}
		res.RetainedHeap = append(res.RetainedHeap, retainedHeap())
	}
	return stopWith(0)
}

func retainedHeap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// resetBursts empties the bursts directory, so every run starts from the
// same tree.
func resetBursts(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// burst k: create directory n<k> with files empty files; rename n<k-1>, the
// previous burst's directory, to m<k-1> (a moved subtree); delete m<k-2>
// with its files; replace the previous sentinel with s<k>. It returns the
// number of operations and the time the sentinel creation started.
func burst(dir string, k, files int, sentinel string) (int, time.Time, error) {
	ops := 0
	created := filepath.Join(dir, fmt.Sprintf("n%04d", k))
	if err := os.Mkdir(created, 0o750); err != nil {
		return ops, time.Time{}, err
	}
	ops++
	for f := 0; f < files; f++ {
		if err := os.WriteFile(filepath.Join(created, fmt.Sprintf("f%04d.txt", f)), nil, 0o600); err != nil {
			return ops, time.Time{}, err
		}
		ops++
	}
	if k >= 2 {
		if err := os.Rename(filepath.Join(dir, fmt.Sprintf("n%04d", k-1)), filepath.Join(dir, fmt.Sprintf("m%04d", k-1))); err != nil {
			return ops, time.Time{}, err
		}
		ops++
		if err := os.Remove(filepath.Join(dir, fmt.Sprintf("s%04d", k-1))); err != nil {
			return ops, time.Time{}, err
		}
		ops++
	}
	if k >= 3 {
		old := filepath.Join(dir, fmt.Sprintf("m%04d", k-2))
		if err := os.RemoveAll(old); err != nil {
			return ops, time.Time{}, err
		}
		ops += files + 1
	}
	t0 := time.Now()
	if err := os.WriteFile(filepath.Join(dir, sentinel), nil, 0o600); err != nil {
		return ops, t0, err
	}
	return ops + 1, t0, nil
}

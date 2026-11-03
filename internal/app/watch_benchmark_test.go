//go:build linux || windows || darwin

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/watch"
)

// The Structural Watch v1 benchmarks of plan §16, reported in
// docs/benchmarks/v0.4-a8.md. Each benchmark runs once and takes its own
// five samples where the plan asks for a median; the idle and content
// windows can be shortened through the environment. Times are not CI gates.
//
//	go test ./internal/app -run '^$' -bench BenchmarkWatch -benchtime=1x -timeout 60m

// benchWindow returns the duration in the environment variable name, or def.
func benchWindow(b *testing.B, name string, def time.Duration) time.Duration {
	b.Helper()
	value := os.Getenv(name)
	if value == "" {
		return def
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		b.Fatalf("%s: %v", name, err)
	}
	return d
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// reportSamples reports the median and the largest of samples.
func reportSamples(b *testing.B, name string, samples []time.Duration) {
	b.Helper()
	sorted := slices.Sorted(slices.Values(samples))
	b.ReportMetric(milliseconds(sorted[len(sorted)/2]), name+"-median-ms")
	b.ReportMetric(milliseconds(sorted[len(sorted)-1]), name+"-max-ms")
}

func benchRequest(root string) WatchRequest {
	return WatchRequest{Inspect: InspectRequest{Root: root, UseDefaultIgnores: true, UseGitIgnore: true}}
}

func (r *recordingObserver) observations() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func states(events []watch.Event) int {
	n := 0
	for _, ev := range events {
		if ev.Kind == watch.KindStateChanged {
			n++
		}
	}
	return n
}

// BenchmarkWatchSignalToState: one directory created in a 10k-entry tree,
// until its state.changed is published (native, default debounce).
func BenchmarkWatchSignalToState(b *testing.B) {
	root := b.TempDir()
	writeSyntheticTree(b, root, 10000)
	request := benchRequest(root)
	w := startWatch(b, request)
	w.log.waitState(b, fingerprintOf(b, request.Inspect))
	w.obs.waitObservations(b, 1) // the startup's verification
	var samples []time.Duration
	for i := 0; i < 5; i++ {
		before := states(w.log.recorded())
		start := time.Now()
		mkdirAll(b, filepath.Join(root, fmt.Sprintf("new-%d", i)))
		w.log.waitFor(b, "the generation", func(events []watch.Event) bool { return states(events) > before })
		samples = append(samples, time.Since(start))
		w.obs.waitObservations(b, 1) // the verification of the new registration
	}
	_ = w.interrupt(b)
	reportSamples(b, "latency", samples)
}

// BenchmarkWatchObservation: one reconciliation (a full observation), and a
// full rescan (the observation and its comparison with the last state).
func BenchmarkWatchObservation(b *testing.B) {
	for _, size := range []int{10_000, 100_000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			root := b.TempDir()
			writeSyntheticTree(b, root, size)
			observer := newWatchObserver(InspectRequest{Root: root, UseDefaultIgnores: true, UseGitIgnore: true})
			ctx := context.Background()
			last, err := observer.Observe(ctx)
			if err != nil {
				b.Fatal(err)
			}
			ref := comparison.SourceRef{Kind: "filesystem"}
			var observe, rescan []time.Duration
			for i := 0; i < 5; i++ {
				start := time.Now()
				obs, err := observer.Observe(ctx)
				if err != nil {
					b.Fatal(err)
				}
				observe = append(observe, time.Since(start))
				if _, err := comparison.CompareRecords(last.Records, obs.Records, ref, ref, comparison.Options{MoveDetection: comparison.MoveDetectionStructuralV1}); err != nil {
					b.Fatal(err)
				}
				rescan = append(rescan, time.Since(start))
				last = obs
			}
			b.ReportMetric(float64(len(last.Records)), "nodes")
			reportSamples(b, "observe", observe)
			reportSamples(b, "rescan", rescan)
		})
	}
}

// BenchmarkWatchContentWrites: content-only writes (append, truncate,
// rewrite, chmod, touch) to 100 files of a 10k-entry tree for a minute. No
// structural event may follow, and the observations they cause are counted
// (the reconciliation is pushed beyond the window).
func BenchmarkWatchContentWrites(b *testing.B) {
	window := benchWindow(b, "DIRLOOM_BENCH_CONTENT", time.Minute)
	root := b.TempDir()
	writeSyntheticTree(b, root, 10000)
	files, err := filepath.Glob(filepath.Join(root, "pkg-00*", "file-00*.go"))
	if err != nil || len(files) < 100 {
		b.Fatalf("%d files, %v", len(files), err)
	}
	files = files[:100]
	request := benchRequest(root)
	w := startWatchWith(b, request, func(c *watch.Config) { c.Reconcile = time.Hour })
	w.log.waitState(b, fingerprintOf(b, request.Inspect))
	w.obs.waitObservations(b, 1) // the startup's verification
	before := w.obs.observations()
	writes, deadline := 0, time.Now().Add(window)
	for time.Now().Before(deadline) {
		for _, f := range files {
			switch writes % 5 {
			case 0:
				fh, err := os.OpenFile(f, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					b.Fatal(err)
				}
				_, _ = fh.WriteString("line\n")
				_ = fh.Close()
			case 1:
				_ = os.Truncate(f, 0)
			case 2:
				_ = os.WriteFile(f, []byte("rewritten\n"), 0o644)
			case 3:
				_ = os.Chmod(f, 0o600)
				_ = os.Chmod(f, 0o644)
			default:
				now := time.Now()
				_ = os.Chtimes(f, now, now)
			}
			writes++
		}
	}
	// A sentinel directory: its observation, and the verification of its
	// registration, are the only ones expected.
	mkdirAll(b, filepath.Join(root, "sentinel"))
	w.log.waitState(b, fingerprintOf(b, request.Inspect))
	w.obs.waitObservations(b, 1)
	observations := w.obs.observations() - before - 2
	_ = w.interrupt(b)
	for _, ev := range w.log.recorded() {
		if ev.Kind.IsNode() && ev.Change.Path != "sentinel" {
			b.Fatalf("structural event %s %s after content writes", ev.Kind, ev.Change.Path)
		}
	}
	b.ReportMetric(float64(writes), "writes")
	b.ReportMetric(float64(observations), "observations")
}

// gateSink holds the batch from which it blocks until gate is closed, as a
// consumer that stops reading.
type gateSink struct {
	watch.Sink
	from    int
	gate    chan struct{}
	blocked chan struct{}

	mu sync.Mutex
	n  int
}

func (s *gateSink) Publish(events []watch.Event) error {
	s.mu.Lock()
	s.n++
	n := s.n
	s.mu.Unlock()
	if n == s.from {
		close(s.blocked)
		<-s.gate
	}
	return s.Sink.Publish(events)
}

// heapPeak samples the heap in use until stop is called and returns the
// largest value seen.
func heapPeak() (stop func() uint64) {
	var peak uint64
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			peak = max(peak, m.HeapInuse)
			select {
			case <-done:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()
	return func() uint64 {
		close(done)
		<-finished
		return peak
	}
}

func heapInUse() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapInuse
}

// BenchmarkWatchSlowConsumer: while the consumer does not read, a
// generation waits and the given number of entries are created and removed
// in a 10k-entry tree; then the consumer reads again. The peak heap of the
// catch-up is compared with the steady heap of the running watch.
func BenchmarkWatchSlowConsumer(b *testing.B) {
	for _, changes := range []int{1_000, 10_000} {
		b.Run(fmt.Sprint(changes), func(b *testing.B) {
			root := b.TempDir()
			writeSyntheticTree(b, root, 10000)
			request := benchRequest(root)
			sink := &gateSink{from: 2, gate: make(chan struct{}), blocked: make(chan struct{})}
			w := startWatchWith(b, request, func(c *watch.Config) { sink.Sink, c.Sink = c.Sink, sink })
			released := false
			b.Cleanup(func() {
				if !released {
					close(sink.gate)
				}
			})
			w.log.waitState(b, fingerprintOf(b, request.Inspect))
			w.obs.waitObservations(b, 1)
			steady := heapInUse()
			mkdirAll(b, filepath.Join(root, "first"))
			select {
			case <-sink.blocked:
			case <-time.After(watchWait):
				b.Fatal("the first generation never reached the consumer")
			}
			churn := filepath.Join(root, "churn")
			mkdirAll(b, churn)
			for i := 0; i < changes/2; i++ {
				writeFiles(b, filepath.Join(churn, fmt.Sprintf("f-%05d", i)))
			}
			if err := os.RemoveAll(churn); err != nil {
				b.Fatal(err)
			}
			final := fingerprintOf(b, request.Inspect)
			runtime.GC()
			stop := heapPeak()
			close(sink.gate)
			released = true
			w.log.waitState(b, final)
			peak := stop()
			_ = w.interrupt(b)
			b.ReportMetric(float64(steady)/(1<<20), "steady-MB")
			b.ReportMetric(float64(peak)/(1<<20), "peak-MB")
			b.ReportMetric(float64(peak)/float64(steady), "peak/steady")
		})
	}
}

// BenchmarkWatchStartup: from the start of a native watch of 10,000
// directories (10,100 with their parents) to its watch.started, which
// follows the registrations on Linux and macOS.
func BenchmarkWatchStartup(b *testing.B) {
	root := b.TempDir()
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			mkdirAll(b, filepath.Join(root, fmt.Sprintf("d%02d", i), fmt.Sprintf("s%02d", j)))
		}
	}
	request := benchRequest(root)
	var samples []time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		w := startWatch(b, request)
		w.log.waitFor(b, "watch.started", func(events []watch.Event) bool {
			return len(events) > 0 && events[0].Kind == watch.KindStarted
		})
		samples = append(samples, time.Since(start))
		if err := w.interrupt(b); err != nil {
			b.Fatal(err)
		}
	}
	reportSamples(b, "startup", samples)
}

// idleCPU runs a watch of a 10k-entry tree with nothing changing for
// window and reports the process CPU time it used.
func idleCPU(b *testing.B, request WatchRequest, window time.Duration) {
	b.Helper()
	w := startWatch(b, request)
	w.log.waitState(b, fingerprintOf(b, request.Inspect))
	w.obs.waitObservations(b, 1)
	observations, cpu, start := w.obs.observations(), processCPU(b), time.Now()
	<-time.After(window)
	used, wall := processCPU(b)-cpu, time.Since(start)
	observations = w.obs.observations() - observations
	_ = w.interrupt(b)
	b.ReportMetric(milliseconds(used), "cpu-ms")
	b.ReportMetric(100*float64(used)/float64(wall), "cpu-%")
	b.ReportMetric(float64(observations), "observations")
}

// BenchmarkWatchIdleNative: a native watch of a 10k-entry tree left idle
// for ten minutes; only the reconciliation observes.
func BenchmarkWatchIdleNative(b *testing.B) {
	root := b.TempDir()
	writeSyntheticTree(b, root, 10000)
	idleCPU(b, benchRequest(root), benchWindow(b, "DIRLOOM_BENCH_IDLE", 10*time.Minute))
}

// BenchmarkWatchIdlePoll: --poll 2s on an idle 10k-entry tree.
func BenchmarkWatchIdlePoll(b *testing.B) {
	root := b.TempDir()
	writeSyntheticTree(b, root, 10000)
	request := benchRequest(root)
	request.Poll = 2 * time.Second
	idleCPU(b, request, benchWindow(b, "DIRLOOM_BENCH_POLL", 2*time.Minute))
}

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// runRunner runs the selected scenarios, each run in a fresh child process
// of this same binary, and appends one JSON line per run to the results.
func runRunner(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	root := flags.String("fixtures", "", "directory of the fixtures")
	pattern := flags.String("scenarios", ".", "regular expression selecting scenario names")
	priority := flags.String("priority", "", "only scenarios of this priority (critical or high)")
	runs := flags.Int("runs", 5, "runs per scenario")
	limitText := flags.String("mem-limit", "", "memory limit of the measured process and its children, e.g. 8GiB (Windows job object)")
	out := flags.String("out", "", "JSON Lines file the results are appended to")
	label := flags.String("label", "baseline", "label of this campaign in the results")
	timeout := flags.Duration("timeout", 0, "time limit of one run, replacing the scenario's (default 20m)")
	env := flags.String("env", "", "comma-separated KEY=VALUE pairs added to the child environment, e.g. GOMEMLIMIT=6GiB")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *root == "" || *out == "" {
		return errors.New("run: -fixtures and -out are required")
	}
	// The child runs in the scenario's directory: it needs an absolute path.
	fixtureRoot, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	limit, err := parseBytes(*limitText)
	if err != nil {
		return err
	}
	if limit > 0 && !limitSupported() {
		return errors.New("run: -mem-limit needs a Windows job object on this platform")
	}
	selector, err := regexp.Compile(*pattern)
	if err != nil {
		return err
	}
	manifest, err := readManifest(fixtureRoot)
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	var extraEnv []string
	if *env != "" {
		extraEnv = strings.Split(*env, ",")
	}
	file, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304 -- the results file given on the command line.
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	for _, sc := range scenarios() {
		if !selector.MatchString(sc.Name) || (*priority != "" && sc.Priority != *priority) {
			continue
		}
		var missing []string
		for _, need := range sc.Needs {
			if _, ok := manifest[need]; !ok {
				missing = append(missing, need)
			}
		}
		if len(missing) > 0 {
			fmt.Fprintf(os.Stderr, "run: %s skipped: fixtures %v not generated\n", sc.Name, missing)
			continue
		}
		if *timeout > 0 {
			sc.Timeout = *timeout
		}
		for run := 1; run <= *runs; run++ {
			res := runOnce(exe, fixtureRoot, sc, run, limit, extraEnv)
			res.Label, res.MemLimit, res.Env = *label, limit, *env
			line, err := json.Marshal(res)
			if err != nil {
				return err
			}
			if _, err := file.Write(append(line, '\n')); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "run: %-24s %d/%d exit=%d ok=%t wall=%.2fs heap=%s commit=%s ws=%s job=%s%s\n",
				sc.Name, run, *runs, res.Exit, res.OK, float64(res.WallNs)/1e9,
				mib(res.HeapPeak), mib(res.PeakPrivate), mib(res.PeakWorkingSet), mib(res.JobPeak), errorSuffix(res))
		}
	}
	return nil
}

func errorSuffix(res result) string {
	switch {
	case res.OOM:
		return " OUT OF MEMORY"
	case res.Error != "":
		return " error: " + res.Error
	}
	return ""
}

func mib(b uint64) string { return fmt.Sprintf("%.0fMiB", float64(b)/(1<<20)) }

// runOnce starts the child, puts it in a fresh job before it runs anything,
// waits for it, and completes its result with what only the runner sees.
func runOnce(exe, root string, sc scenario, run int, limit uint64, extraEnv []string) result {
	res := result{Scenario: sc.Name, Run: run, Start: time.Now().UTC().Format(time.RFC3339)}
	cmd := exec.Command(exe, "exec", "-fixtures", root, "-scenario", sc.Name, "-wait") // #nosec G204 -- this binary, with a scenario of its own registry.
	cmd.Dir = sc.dir(root)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout bytes.Buffer
	stderr := &headTail{limit: 2048}
	cmd.Stdout, cmd.Stderr = &stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	j, err := newJob(limit)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer j.close()
	begin := time.Now()
	if err := cmd.Start(); err != nil {
		res.Error = err.Error()
		return res
	}
	process, err := openMeasured(cmd.Process.Pid)
	if err == nil {
		defer process.close()
		err = j.assign(process)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		res.Error = err.Error()
		return res
	}
	if _, err := stdin.Write([]byte("go\n")); err != nil {
		res.Error = err.Error()
	}
	_ = stdin.Close()

	timer := time.AfterFunc(sc.Timeout, func() {
		j.terminate()
		_ = cmd.Process.Kill()
	})
	// After "fatal error: out of memory" under a job memory limit, a Go
	// process on Windows printed its traceback and then did not exit (a
	// plain Go program does the same). The fatal error is the outcome:
	// end the job a few seconds after it appears.
	exited, watcherDone := make(chan struct{}), make(chan struct{})
	var fatalAt time.Duration
	go func() {
		defer close(watcherDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-exited:
				return
			case <-ticker.C:
			}
			if _, head, _ := stderr.snapshot(); strings.Contains(head, "fatal error:") {
				fatalAt = time.Since(begin)
				select {
				case <-exited:
				case <-time.After(5 * time.Second):
					j.terminate()
					_ = cmd.Process.Kill()
				}
				return
			}
		}
	}()
	waitErr := cmd.Wait()
	close(exited)
	<-watcherDone
	timedOut := !timer.Stop()
	res.ProcWallNs = time.Since(begin).Nanoseconds()
	if fatalAt > 0 {
		// The time to the failure, not to the end of the hang.
		res.ProcWallNs = fatalAt.Nanoseconds()
	}
	if state := cmd.ProcessState; state != nil {
		res.UserNs, res.SystemNs = state.UserTime().Nanoseconds(), state.SystemTime().Nanoseconds()
	}

	var child result
	if err := json.Unmarshal(lastLine(stdout.Bytes()), &child); err == nil && child.Scenario == sc.Name {
		child.Run, child.Start, child.UserNs, child.SystemNs, child.ProcWallNs = run, res.Start, res.UserNs, res.SystemNs, res.ProcWallNs
		res = child
	} else {
		res.Exit = -1
		if state := cmd.ProcessState; state != nil {
			res.Exit = state.ExitCode()
		}
		res.Error = "no result from the child"
		if waitErr != nil {
			res.Error += ": " + waitErr.Error()
		}
		_, res.StderrHead, res.StderrTail = stderr.snapshot()
		res.OOM = strings.Contains(res.StderrHead, "out of memory") || strings.Contains(res.StderrHead, "cannot allocate memory") ||
			strings.Contains(res.StderrHead, "VirtualAlloc")
	}
	if timedOut {
		res.Error = "timed out after " + sc.Timeout.String()
	}
	if ws, private, err := process.peaks(); err == nil && ws > 0 {
		// The counters read after the exit cover the whole life of the
		// process, a failed one included.
		res.PeakWorkingSet, res.PeakPrivate = ws, private
	}
	if jobPeak, _, err := j.peakCommit(); err == nil {
		res.JobPeak = jobPeak
	}
	res.OK = res.Error == "" && slices.Contains(sc.Exits, res.Exit) && strings.Contains(res.StdoutHead, sc.Expect)
	if res.Error == "" && !res.OK {
		res.Error = fmt.Sprintf("unexpected outcome: exit %d, want %v", res.Exit, sc.Exits)
		if sc.Expect != "" && !strings.Contains(res.StdoutHead, sc.Expect) {
			res.Error += fmt.Sprintf(", stdout without %s", sc.Expect)
		}
	}
	if sc.Watch != nil && res.Watch != nil && res.OK {
		if res.Watch.Errors > 0 || res.Watch.Converged != res.Watch.Bursts || !res.Watch.Stopped {
			res.OK = false
			res.Error = fmt.Sprintf("watch: %d of %d bursts converged, %d errors, stopped=%t",
				res.Watch.Converged, res.Watch.Bursts, res.Watch.Errors, res.Watch.Stopped)
		}
	}
	return res
}

func lastLine(b []byte) []byte {
	b = bytes.TrimRight(b, "\r\n")
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return b[i+1:]
	}
	return b
}

// parseBytes reads sizes such as 8GiB, 512MiB or a plain byte count.
func parseBytes(text string) (uint64, error) {
	if text == "" || text == "0" {
		return 0, nil
	}
	units := []struct {
		suffix string
		shift  uint
	}{{"GiB", 30}, {"MiB", 20}, {"KiB", 10}}
	for _, u := range units {
		if number, ok := strings.CutSuffix(text, u.suffix); ok {
			n, err := strconv.ParseUint(number, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("size %q: %w", text, err)
			}
			return n << u.shift, nil
		}
	}
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("size %q: %w", text, err)
	}
	return n, nil
}

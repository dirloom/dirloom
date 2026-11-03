package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/app"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/watch"
)

// watchWait bounds every wait of these tests for a real watch; it is a
// safety net, never a timing assumption.
const watchWait = 15 * time.Second

// syncBuffer is written by the engine's writer goroutine while a test reads
// it, and wakes the reader on each write.
type syncBuffer struct {
	mu      sync.Mutex
	b       bytes.Buffer
	changed chan struct{}
}

func newSyncBuffer() *syncBuffer { return &syncBuffer{changed: make(chan struct{}, 1)} }

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	n, err := s.b.Write(p)
	s.mu.Unlock()
	select {
	case s.changed <- struct{}{}:
	default:
	}
	return n, err
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// waitFor blocks until the buffer contains want.
func (s *syncBuffer) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.After(watchWait)
	for !strings.Contains(s.String(), want) {
		select {
		case <-s.changed:
		case <-deadline:
			t.Fatalf("no %q within %s; output:\n%s", want, watchWait, s.String())
		}
	}
}

// watchRun is a dirloom watch running in this process until interrupted.
type watchRun struct {
	stdout, stderr *syncBuffer
	cancel         context.CancelFunc
	done           chan struct{} // closed when the run ended, with code set
	code           int
}

func startWatchCLI(t *testing.T, args ...string) *watchRun {
	t.Helper()
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	r := &watchRun{stdout: newSyncBuffer(), stderr: newSyncBuffer(), cancel: cancel, done: make(chan struct{})}
	go func() {
		r.code = executeWithLoader(ctx, args, r.stdout, r.stderr, "v0.1.0-test", loader)
		close(r.done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.done:
		case <-time.After(watchWait):
		}
	})
	return r
}

// interrupt cancels the run as Ctrl+C does and returns its exit status.
func (r *watchRun) interrupt(t *testing.T) int {
	t.Helper()
	r.cancel()
	select {
	case <-r.done:
		return r.code
	case <-time.After(watchWait):
		t.Fatal("the watch did not end")
		return -1
	}
}

func TestWatchUsageErrorsExitTwo(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--format", "json"}, "use --format ndjson"},
		{[]string{"--format", "yaml"}, "ndjson"},
		{[]string{"--color", "always"}, "--color is not supported by watch"},
		{[]string{"--icons"}, "--icons is not supported by watch"},
		{[]string{"--theme", "midnight"}, "--theme is not supported by watch"},
		{[]string{"--output", "events.ndjson"}, "--output is not supported by watch"},
		{[]string{"-o", "events.ndjson"}, "--output is not supported by watch"},
		{[]string{"--copy"}, "--copy is not supported by watch"},
		{[]string{"git:HEAD"}, "not a git source operand"},
		{[]string{"snapshot:before.dlm.json"}, "not a snapshot source operand"},
		{[]string{"live:."}, "not a live source operand"},
		{[]string{"one", "two"}, "at most one directory"},
		{[]string{"--root", ".", "."}, "cannot be used together"},
		{[]string{"--poll", "1s", "--debounce", "50ms"}, "cannot be combined with --poll"},
		{[]string{"--debounce", "5ms"}, "--debounce must be between 10ms and 10s"},
		{[]string{"--debounce", "11s"}, "--debounce must be between 10ms and 10s"},
		{[]string{"--debounce", "0s"}, "--debounce must be between 10ms and 10s"},
		{[]string{"--debounce", "fast"}, "invalid argument"},
		{[]string{"--poll", "50ms"}, "--poll must be between 100ms and 1h0m0s"},
		{[]string{"--poll", "2h"}, "--poll must be between 100ms and 1h0m0s"},
		{[]string{"--poll", "0"}, "--poll must be between 100ms and 1h0m0s"},
		{[]string{"--depth", "-1"}, "depth"},
		{[]string{"--preset", "unknown"}, "unknown"},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			stdout, stderr, code := executeForTest(t, append([]string{"watch", "--no-config"}, tc.args...)...)
			if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") || !strings.Contains(stderr, tc.want) {
				t.Fatalf("watch %v = (stdout=%q, stderr=%q, code=%d), want exit 2 mentioning %q", tc.args, stdout, stderr, code, tc.want)
			}
		})
	}
	// A configuration that cannot be read (here a directory) is no exit 1.
	stdout, stderr, code := executeForTest(t, "watch", t.TempDir(), "--config", t.TempDir())
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
		t.Fatalf("unreadable configuration = (stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
}

func TestWatchNDJSONStreamEndsOnInterrupt(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"src/a.go", "package a\n"}})
	r := startWatchCLI(t, "watch", root, "--format", "ndjson", "--poll", "100ms", "--no-config")
	r.stdout.waitFor(t, `"event":"watch.started"`)
	writeFiles(t, root, [][2]string{{"docs/guide.md", "# Guide\n"}})
	fp, err := app.Fingerprint(context.Background(), app.InspectRequest{Root: root, UseDefaultIgnores: true, UseGitIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	r.stdout.waitFor(t, `"event":"state.changed","generation":`)
	r.stdout.waitFor(t, `"fingerprint":"`+fp.Fingerprint.String()+`","nodeCount":5`)
	if code := r.interrupt(t); code != 0 {
		t.Fatalf("exit status %d, want 0; stderr %q", code, r.stderr.String())
	}
	out := r.stdout.String()
	if r.stderr.String() != "" {
		t.Fatalf("stderr %q, want nothing in ndjson mode", r.stderr.String())
	}
	if !strings.HasSuffix(out, "\n") || strings.Contains(out, "\r") || strings.Contains(out, filepath.ToSlash(root)) || strings.Contains(out, root) {
		t.Fatalf("stdout is not a clean ndjson stream without the absolute root:\n%s", out)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, `{"schemaVersion":1,"event":"`) {
			t.Fatalf("line %q does not start with schemaVersion and event", line)
		}
	}
	if !strings.Contains(lines[0], `"backend":"poll"`) ||
		lines[len(lines)-1] != `{"schemaVersion":1,"event":"watch.stopped","generation":`+generationOf(t, lines[len(lines)-2])+`,"reason":"interrupted","fingerprint":"`+fp.Fingerprint.String()+`"}` {
		t.Fatalf("stream:\n%s", out)
	}
}

// generationOf extracts the generation of a state.changed line.
func generationOf(t *testing.T, line string) string {
	t.Helper()
	_, rest, ok := strings.Cut(line, `"generation":`)
	value, _, _ := strings.Cut(rest, ",")
	if !ok || !strings.Contains(line, `"event":"state.changed"`) {
		t.Fatalf("not a state.changed line: %q", line)
	}
	return value
}

func TestWatchTextReportsANativeGeneration(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"README.md", "hi\n"}})
	r := startWatchCLI(t, "watch", root, "--no-config")
	r.stdout.waitFor(t, "Watching structure (experimental, native): 2 nodes, dlm:v1:sha256:")
	if err := os.Mkdir(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.stdout.waitFor(t, "#1  + docs\n")
	r.stdout.waitFor(t, "#1  = 3 nodes, ")
	if code := r.interrupt(t); code != 0 {
		t.Fatalf("exit status %d, want 0; stderr %q", code, r.stderr.String())
	}
	if out := r.stdout.String(); !strings.HasSuffix(out, "\nStopped at #1.\n") || r.stderr.String() != "" || strings.Contains(out, "\x1b") {
		t.Fatalf("stdout %q, stderr %q", out, r.stderr.String())
	}
}

func TestWatchStartupFailuresExitFive(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFiles(t, dir, [][2]string{{"file", ""}})
	for _, tc := range []struct {
		root, message string
	}{
		{filepath.Join(dir, "missing"), "the watched root does not exist"},
		{file, "the watched root is not a directory"},
	} {
		stdout, stderr, code := executeForTest(t, "watch", tc.root, "--format", "ndjson", "--no-config")
		want := `{"schemaVersion":1,"event":"watch.error","fatal":true,"diagnostic":{"code":"watch_root_unavailable","message":"` + tc.message + `"}}` + "\n" +
			`{"schemaVersion":1,"event":"watch.stopped","reason":"error"}` + "\n"
		if code != 5 || stdout != want || stderr != "" {
			t.Fatalf("ndjson on %s = (stdout=%q, stderr=%q, code=%d)", filepath.Base(tc.root), stdout, stderr, code)
		}
		stdout, stderr, code = executeForTest(t, "watch", tc.root, "--no-config")
		if code != 5 || stdout != "Stopped before the first state.\n" || stderr != "Error: "+tc.message+" (watch_root_unavailable)\n" {
			t.Fatalf("text on %s = (stdout=%q, stderr=%q, code=%d)", filepath.Base(tc.root), stdout, stderr, code)
		}
	}
}

func TestWatchOutputFailureExitsFive(t *testing.T) {
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	var stderr bytes.Buffer
	code := executeWithLoader(context.Background(), []string{"watch", t.TempDir(), "--format", "ndjson", "--poll", "100ms", "--no-config"},
		failingWriter{}, &stderr, "v0.1.0-test", loader)
	if code != 5 || stderr.String() != "Error: write watch events: synthetic write failure\n" {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestWatchStatusMapsTheEndOfAWatch(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		code   int
		stderr string
	}{
		{"interrupted", nil, 0, ""},
		{"fatal", &watch.Failure{Diagnostic: watch.Diagnostic{Code: "watch_root_unavailable", Message: "gone"}}, 5, ""},
		{"internal", &watch.Failure{Diagnostic: watch.Diagnostic{Code: "internal_invariant_failure", Message: "x"}, Internal: true}, 6, ""},
		{"output", &watch.OutputError{Err: errors.New("broken pipe")}, 5, "Error: write watch events: broken pipe\n"},
		{"blocked", watch.ErrOutputBlocked, 5, "Error: " + watch.ErrOutputBlocked.Error() + "\n"},
		{"usage", &app.WatchUsageError{Message: "bad request"}, 2, "Error: bad request\n"},
		{"unexpected", errors.New("boom"), 6, "Error: internal error: boom\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := processStatus(watchStatus(tc.err), &stderr); code != tc.code || stderr.String() != tc.stderr {
				t.Fatalf("exit %d, stderr %q; want %d, %q", code, stderr.String(), tc.code, tc.stderr)
			}
		})
	}
}

func TestWatchIsACommandAndADirectoryNamedWatchIsStillInspected(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"watch/inside.txt", ""}})
	t.Chdir(root)
	stdout, stderr, code := executeForTest(t, "./watch")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "inside.txt") {
		t.Fatalf("dirloom ./watch = (stdout=%q, stderr=%q, code=%d)", stdout, stderr, code)
	}
}

func TestWatchHelpAndCompletion(t *testing.T) {
	stdout, _, code := executeForTest(t, "watch", "--help")
	if code != 0 || !strings.HasPrefix(stdout, "EXPERIMENTAL") {
		t.Fatalf("watch --help (code %d):\n%s", code, stdout)
	}
	for _, want := range []string{"--poll", "--debounce", "--no-moves", "--format", "ndjson", "state.changed", "watch.stopped",
		"dirloom watch --poll 2s", "kern.maxfilesperproc", "fs.inotify.max_user_watches"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("watch help missing %q", want)
		}
	}
	for _, hidden := range []string{"--color", "--icons", "--theme", "--copy", "--output"} {
		if strings.Contains(stdout, hidden) {
			t.Errorf("watch help shows the refused flag %s", hidden)
		}
	}
	root, _, _ := executeForTest(t, "--help")
	if !strings.Contains(root, "watch") || !strings.Contains(root, "EXPERIMENTAL") {
		t.Fatalf("root help does not list the experimental watch command:\n%s", root)
	}
	for _, tc := range []struct {
		args     []string
		want     []string
		excluded []string
	}{
		{[]string{"__complete", "watch", "--format", ""}, []string{"text\n", "ndjson\n"}, []string{"\njson\n"}},
		{[]string{"__complete", "watch", "--preset", ""}, []string{"docs", "none"}, nil},
		{[]string{"__complete", "watch", "--"}, []string{"--poll", "--debounce", "--no-moves"}, []string{"--color", "--copy"}},
		{[]string{"__complete", "watch", ""}, []string{":16"}, nil},
	} {
		stdout, _, code := executeForTest(t, tc.args...)
		if code != 0 {
			t.Fatalf("%v code=%d", tc.args, code)
		}
		for _, want := range tc.want {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v missing %q\n%s", tc.args, want, stdout)
			}
		}
		for _, excluded := range tc.excluded {
			if strings.Contains(stdout, excluded) {
				t.Errorf("%v offers %q\n%s", tc.args, excluded, stdout)
			}
		}
	}
}

// TestWatchUsesTheFingerprintCapture checks that watch resolves its capture
// exactly as fingerprint does: configuration, then flags.
func TestWatchUsesTheFingerprintCapture(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\nignore:\n  - generated\n")
	writeFiles(t, root, [][2]string{{"generated/out.go", "x"}, {"keep.go", "x"}})
	for _, tc := range []struct {
		flags   []string
		capture string
		moves   string
	}{
		{nil, `"capture":{"depth":null,"dirsOnly":false,"hidden":false,"useDefaultIgnores":true,"useGitignore":true,"ignore":["generated"]}`, "structural-v1"},
		{[]string{"--depth", "1", "--dirs-only", "--hidden", "--no-default-ignore", "--no-gitignore", "--no-moves", "--ignore", "*.tmp"},
			`"capture":{"depth":1,"dirsOnly":true,"hidden":true,"useDefaultIgnores":false,"useGitignore":false,"ignore":["generated","*.tmp"]}`, "none"},
	} {
		var captureFlags []string // the flags without --no-moves, which fingerprint does not take
		for _, flag := range tc.flags {
			if flag != "--no-moves" {
				captureFlags = append(captureFlags, flag)
			}
		}
		fingerprint, _, code := executeForTest(t, append([]string{"fingerprint", root}, captureFlags...)...)
		if code != 0 {
			t.Fatalf("fingerprint exit %d", code)
		}
		r := startWatchCLI(t, append([]string{"watch", root, "--format", "ndjson", "--poll", "100ms"}, tc.flags...)...)
		r.stdout.waitFor(t, `"event":"watch.started"`)
		if code := r.interrupt(t); code != 0 {
			t.Fatalf("exit %d; stderr %q", code, r.stderr.String())
		}
		started, _, _ := strings.Cut(r.stdout.String(), "\n")
		if !strings.Contains(started, tc.capture) || !strings.Contains(started, `"moveDetection":"`+tc.moves+`"`) ||
			!strings.Contains(started, `"fingerprint":"`+strings.TrimSpace(fingerprint)+`"`) {
			t.Fatalf("watch.started %s\nwant %s and the fingerprint %s", started, tc.capture, fingerprint)
		}
	}
}

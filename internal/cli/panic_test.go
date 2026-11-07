package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/gittest"
)

// panickingWriter keeps the first limit bytes written to it, then panics
// with value: a bug that strikes while a command writes its result.
type panickingWriter struct {
	limit int
	value any
	kept  bytes.Buffer
}

func (w *panickingWriter) Write(p []byte) (int, error) {
	if room := w.limit - w.kept.Len(); len(p) > room {
		w.kept.Write(p[:room])
		panic(w.value)
	}
	return w.kept.Write(p)
}

// indexWriter panics with a runtime error, as an out-of-range index does.
type indexWriter struct{}

func (indexWriter) Write(p []byte) (int, error) {
	i := len(p)
	return int(p[i]), nil
}

// executeWithStdout runs the CLI with stdout and without user configuration.
// The deadline ends a watch that the guard would fail to stop.
func executeWithStdout(t *testing.T, stdout io.Writer, args ...string) (string, int) {
	t.Helper()
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stderr bytes.Buffer
	code := executeWithLoader(ctx, args, stdout, &stderr, "v0.1.0-test", loader)
	return stderr.String(), code
}

// panicFixture is a small tree and a snapshot of it, stored outside it.
func panicFixture(t *testing.T) (root, snapshotPath string) {
	t.Helper()
	root = t.TempDir()
	writeFiles(t, root, [][2]string{{"README.md", "readme\n"}, {"src/main.go", "package main\n"}})
	stdout, stderr, code := executeForTest(t, "snapshot", root)
	if code != 0 || stderr != "" {
		t.Fatalf("snapshot=(%q, %d)", stderr, code)
	}
	snapshotPath = filepath.Join(t.TempDir(), "tree.dlm.json")
	if err := os.WriteFile(snapshotPath, []byte(stdout), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, snapshotPath
}

// A panic while a command writes its result is an internal error of the
// command: exit 1 in the first family, 6 in the 0-6 family. The diagnostic is
// one stderr line, without a stack trace (stderr is compared whole), and the
// guard adds nothing to stdout.
func TestPanicIsAnInternalErrorOfTheCommand(t *testing.T) {
	root, snap := panicFixture(t)
	const want = "Error: internal error: unexpected panic: synthetic bug\n"
	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		stderr string
	}{
		{"tree", []string{root}, 1, want},
		{"tree json", []string{root, "--format", "json"}, 1, want},
		{"fingerprint", []string{"fingerprint", root}, 1, want},
		{"snapshot", []string{"snapshot", root}, 1, want},
		{"config explain", []string{"config", "explain", root}, 1, want},
		{"preset explain", []string{"preset", "explain", "docs"}, 1, want},
		{"theme list", []string{"theme", "list"}, 1, want},
		{"completion", []string{"completion", "bash"}, 1, want},
		{"help", []string{"help", "verify"}, 1, want},
		{"verify", []string{"verify", snap, root}, 6, want},
		{"verify json", []string{"verify", snap, root, "--format", "json"}, 6, want},
		{"diff", []string{"diff", "snapshot:" + snap, "live:" + root}, 6, want},
		{"diff json", []string{"diff", "snapshot:" + snap, "live:" + root, "--format", "json"}, 6, want},
		// The events are written by the watch's own goroutine, which guards
		// itself: the run ends as after a failed write, but with exit 6.
		{"watch", []string{"watch", root, "--poll", "100ms"}, 6, "Error: internal error: write watch events: unexpected panic: synthetic bug\n"},
		{"watch ndjson", []string{"watch", root, "--format", "ndjson", "--poll", "100ms"}, 6, "Error: internal error: write watch events: unexpected panic: synthetic bug\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout := &panickingWriter{value: "synthetic bug"}
			stderr, code := executeWithStdout(t, stdout, tc.args...)
			if code != tc.code || stderr != tc.stderr || stdout.kept.Len() != 0 {
				t.Fatalf("exit %d, stderr %q, stdout %q; want exit %d, stderr %q, no stdout", code, stderr, stdout.kept.String(), tc.code, tc.stderr)
			}
		})
	}
}

func TestPanicInHistoryIsAnInternalError(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("README.md", "readme\n")
	repo.CommitAll("A")
	t.Chdir(repo.Dir)
	for _, args := range [][]string{{"history"}, {"history", "--format", "json"}} {
		stdout := &panickingWriter{value: "synthetic bug"}
		stderr, code := executeWithStdout(t, stdout, args...)
		if code != 6 || stderr != "Error: internal error: unexpected panic: synthetic bug\n" || stdout.kept.Len() != 0 {
			t.Fatalf("%v: exit %d, stderr %q, stdout %q", args, code, stderr, stdout.kept.String())
		}
	}
}

// The guard cannot complete an output that is already partly written: stdout
// keeps the truncated document, and the exit status says it is no success.
func TestPanicLeavesAPartialOutputAsAFailure(t *testing.T) {
	root, snap := panicFixture(t)
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"fingerprint json", []string{"fingerprint", root, "--format", "json"}, 1},
		{"verify json", []string{"verify", snap, root, "--format", "json"}, 6},
		{"diff json", []string{"diff", "snapshot:" + snap, "live:" + root, "--format", "json"}, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var complete bytes.Buffer
			if stderr, code := executeWithStdout(t, &complete, tc.args...); code != 0 || complete.Len() <= 10 {
				t.Fatalf("reference run: exit %d, stderr %q, stdout %q", code, stderr, complete.String())
			}
			stdout := &panickingWriter{limit: 10, value: "synthetic bug"}
			stderr, code := executeWithStdout(t, stdout, tc.args...)
			if code != tc.code || stderr != "Error: internal error: unexpected panic: synthetic bug\n" {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			if got := stdout.kept.String(); got != complete.String()[:10] || json.Valid(stdout.kept.Bytes()) {
				t.Fatalf("stdout %q, want the first 10 bytes of %q, not a valid document", got, complete.String())
			}
		})
	}
}

// The value of a runtime panic is the Go runtime's text, which changes
// between Go releases: only a fragment of it is checked.
func TestPanicValues(t *testing.T) {
	const prefix = "Error: internal error: unexpected panic: "
	root := t.TempDir()
	for _, tc := range []struct {
		name     string
		stdout   io.Writer
		fragment string
	}{
		{"error", &panickingWriter{value: errors.New("broken invariant")}, "broken invariant"},
		{"runtime error", indexWriter{}, "index out of range"},
		{"nil", &panickingWriter{value: nil}, "panic called with nil argument"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stderr, code := executeWithStdout(t, tc.stdout, root)
			if code != 1 || !strings.HasPrefix(stderr, prefix) || !strings.Contains(stderr, tc.fragment) ||
				strings.Count(stderr, "\n") != 1 || !strings.HasSuffix(stderr, "\n") {
				t.Fatalf("exit %d, stderr %q; want exit 1 and one line %q…%q…", code, stderr, prefix, tc.fragment)
			}
		})
	}
}

// Under GODEBUG=panicnil=1, panic(nil) recovers a nil value. It is still a
// panic, never a success.
func TestPanicNilIsNeverASuccess(t *testing.T) {
	t.Setenv("GODEBUG", "panicnil=1")
	stderr, code := executeWithStdout(t, &panickingWriter{value: nil}, t.TempDir())
	if code != 1 || stderr != "Error: internal error: unexpected panic: <nil>\n" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
}

// The tree output can fail with nothing written to stdout, here in the
// clipboard.
func TestPanicInTheClipboardIsAnInternalError(t *testing.T) {
	stdout, stderr, code := executeForTestWithClipboard(t, panickingClipboard{}, t.TempDir(), "--copy")
	if code != 1 || stdout != "" || stderr != "Error: internal error: unexpected panic: clipboard bug\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

type panickingClipboard struct{}

func (panickingClipboard) Write([]byte) error { panic("clipboard bug") }

// Every top-level command has a stated exit status for a broken invariant,
// so that a new command cannot fall into a family by accident.
func TestInternalExitStatusOfEveryCommand(t *testing.T) {
	root := newRootCommand(io.Discard, io.Discard, "v0.1.0-test", configuration.NewLoader())
	want := map[string]int{
		"config": 1, "preset": 1, "theme": 1, "fingerprint": 1, "snapshot": 1, "completion": 1,
		"verify": 6, "diff": 6, "history": 6, "watch": 6,
	}
	for _, command := range root.Commands() {
		code, ok := want[command.Name()]
		if !ok {
			t.Fatalf("command %q has no expected internal exit status", command.Name())
		}
		if got := internalExitStatus(root, []string{command.Name()}); got != code {
			t.Fatalf("%s: internal exit status %d, want %d", command.Name(), got, code)
		}
	}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{nil, 1},
		{[]string{"."}, 1},
		{[]string{"./verify"}, 1}, // a directory named verify
		{[]string{"--format", "json", "verify"}, 6},
		{[]string{"--no-config", "watch", "."}, 6},
		{[]string{"verify", "--format", "json", "a.dlm.json"}, 6},
		{[]string{"config", "explain", "."}, 1},
		{[]string{"theme", "list"}, 1},
		{[]string{"help", "verify"}, 1},
		{[]string{"unknown", "extra"}, 1},
	} {
		if got := internalExitStatus(root, tc.args); got != tc.code {
			t.Fatalf("%q: internal exit status %d, want %d", tc.args, got, tc.code)
		}
	}
}

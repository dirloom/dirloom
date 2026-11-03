package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dirloom/dirloom/internal/app"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/watch"
)

// spawnWatch runs dirloom as a child process whose stdout the test reads,
// or stops reading, through the returned pipe.
func spawnWatch(t *testing.T, args ...string) (*exec.Cmd, *os.File, *syncBuffer) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := newSyncBuffer()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWatchHelperProcess$") // #nosec G204 -- the test binary itself.
	cmd.Env = append(os.Environ(), watchHelperArgs+"="+strings.Join(args, "\n"))
	cmd.Stdout, cmd.Stderr = w, stderr
	ownProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		_ = r.Close()
	})
	return cmd, r, stderr
}

// readExactly reads n bytes from r, or one line when n is 0, one byte at a
// time so that nothing more is consumed from the pipe.
func readExactly(t *testing.T, r io.Reader, n int) []byte {
	t.Helper()
	done := make(chan []byte, 1)
	go func() {
		var out []byte
		b := make([]byte, 1)
		for n == 0 || len(out) < n {
			if _, err := r.Read(b); err != nil {
				break
			}
			out = append(out, b[0])
			if n == 0 && b[0] == '\n' {
				break
			}
		}
		done <- out
	}()
	select {
	case out := <-done:
		return out
	case <-time.After(watchWait):
		t.Fatal("nothing to read from the watch process")
		return nil
	}
}

func waitExit(t *testing.T, cmd *exec.Cmd) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(watchWait):
		t.Fatal("the watch process did not exit")
		return nil
	}
}

func exitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		return -1
	}
	return 0
}

// manyFiles creates n files in dir, with names long enough that their
// removal encodes to far more than a pipe holds.
func manyFiles(t *testing.T, dir string, n int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("a-file-with-a-rather-long-name-%05d.txt", i)))
		if err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
	}
}

// checkStream parses a whole stream and validates it.
func checkStream(t *testing.T, data []byte, complete bool) []watch.Event {
	t.Helper()
	events, _, err := watch.ParseNDJSON(data)
	if err != nil {
		t.Fatalf("ParseNDJSON: %v", err)
	}
	if err := watch.ValidateStream(events, complete); err != nil {
		t.Fatalf("invalid stream: %v", err)
	}
	return events
}

// TestWatchProcessBlockedOutputEndsAfterTheGracePeriod is F15: the consumer
// stops reading, a generation fills the pipe, then an interruption. The
// process ends within the output grace period with exit status 5 and a
// best-effort diagnostic, and the stream it leaves has no watch.stopped.
func TestWatchProcessBlockedOutputEndsAfterTheGracePeriod(t *testing.T) {
	root, aside := t.TempDir(), t.TempDir()
	manyFiles(t, filepath.Join(root, "many"), 3000)
	cmd, out, stderr := spawnWatch(t, "watch", root, "--format", "ndjson", "--poll", "100ms", "--no-config")
	stream := readExactly(t, out, 0)
	// Moved out at once, the 3,001 removals are one generation of about
	// 380 KB: far more than a pipe holds.
	if err := os.Rename(filepath.Join(root, "many"), filepath.Join(aside, "many")); err != nil {
		t.Fatal(err)
	}
	stream = append(stream, readExactly(t, out, 1)...) // the generation is being written
	interruptProcess(t, cmd)
	start := time.Now()
	err := waitExit(t, cmd)
	elapsed := time.Since(start)
	if code := exitCode(err); code != 5 || !strings.Contains(stderr.String(), "watch output blocked") {
		t.Fatalf("exit %d after %s, stderr %q", code, elapsed, stderr.String())
	}
	rest, _ := io.ReadAll(out)
	events := checkStream(t, append(stream, rest...), false)
	if last := events[len(events)-1]; last.Kind == watch.KindStopped {
		t.Fatal("watch.stopped written although the output was blocked")
	}
	t.Logf("ended %s after the interruption", elapsed.Round(time.Millisecond))
}

// TestWatchProcessClosedOutput: when the consumer closes stdout, the next
// write ends the process: SIGPIPE on Unix, exit status 5 with a diagnostic
// on Windows.
func TestWatchProcessClosedOutput(t *testing.T) {
	root := t.TempDir()
	cmd, out, stderr := spawnWatch(t, "watch", root, "--format", "ndjson", "--poll", "100ms", "--no-config")
	readExactly(t, out, 0)
	_ = out.Close()
	writeFiles(t, root, [][2]string{{"new", ""}})
	if err := waitExit(t, cmd); !endedByClosedOutput(err, stderr.String()) {
		t.Fatalf("exit: %v, stderr %q", err, stderr.String())
	}
}

// TestWatchProcessInterruptedWhileWritingALargeGeneration: the consumer
// keeps reading; the generation being written completes before
// watch.stopped, and the stream is valid.
func TestWatchProcessInterruptedWhileWritingALargeGeneration(t *testing.T) {
	root, aside := t.TempDir(), t.TempDir()
	manyFiles(t, filepath.Join(root, "many"), 3000)
	cmd, out, stderr := spawnWatch(t, "watch", root, "--format", "ndjson", "--poll", "100ms", "--no-config")
	stream := newSyncBuffer()
	copied := make(chan struct{})
	go func() { _, _ = io.Copy(stream, out); close(copied) }()
	stream.waitFor(t, `"event":"watch.started"`)
	if err := os.Rename(filepath.Join(root, "many"), filepath.Join(aside, "many")); err != nil {
		t.Fatal(err)
	}
	stream.waitFor(t, `"event":"node.removed"`)
	interruptProcess(t, cmd)
	if err := waitExit(t, cmd); err != nil {
		t.Fatalf("exit: %v, stderr %q", err, stderr.String())
	}
	<-copied
	events := checkStream(t, []byte(stream.String()), true)
	if last := events[len(events)-1]; last.Stop != watch.StopInterrupted || stderr.String() != "" {
		t.Fatalf("last event %+v, stderr %q", last, stderr.String())
	}
}

// TestWatchProcessSlowConsumer: the consumer stops reading while 10,000
// entries are created and removed, then resumes; the generations are
// coalesced and the stream converges to the final state.
func TestWatchProcessSlowConsumer(t *testing.T) {
	root := t.TempDir()
	cmd, out, stderr := spawnWatch(t, "watch", root, "--format", "ndjson", "--poll", "100ms", "--no-config")
	started := readExactly(t, out, 0)
	manyFiles(t, filepath.Join(root, "churn"), 10000)
	if err := os.RemoveAll(filepath.Join(root, "churn")); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, root, [][2]string{{"done/marker", ""}})
	final, err := app.Fingerprint(context.Background(), app.InspectRequest{Root: root, UseDefaultIgnores: true, UseGitIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	stream := newSyncBuffer()
	_, _ = stream.Write(started)
	copied := make(chan struct{})
	go func() { _, _ = io.Copy(stream, out); close(copied) }()
	stream.waitFor(t, `"event":"state.changed","generation":`)
	stream.waitFor(t, `"fingerprint":"`+final.Fingerprint.String()+`","nodeCount":3`)
	interruptProcess(t, cmd)
	if err := waitExit(t, cmd); err != nil {
		t.Fatalf("exit: %v, stderr %q", err, stderr.String())
	}
	<-copied
	events := checkStream(t, []byte(stream.String()), true)
	generations := 0
	for _, ev := range events {
		if ev.Kind == watch.KindStateChanged {
			generations++
		}
	}
	t.Logf("%d generations for 20,000 operations", generations)
}

// TestWatchPrivacy: with a marker in the path of the root's parent, neither
// the marker nor the home directory appears on stdout or stderr, through an
// observation error and the root's loss, in both formats.
func TestWatchPrivacy(t *testing.T) {
	home, _ := os.UserHomeDir()
	for _, format := range []string{"ndjson", "text"} {
		t.Run(format, func(t *testing.T) {
			const marker = "dirloom-privacy-marker"
			root := filepath.Join(t.TempDir(), marker, "root")
			writeFiles(t, root, [][2]string{{"caf\u00e9", ""}})
			r := startWatchCLI(t, "watch", root, "--format", format, "--poll", "100ms", "--no-config")
			r.stdout.waitFor(t, map[string]string{"ndjson": `"event":"watch.started"`, "text": "Watching structure"}[format])
			cause, cure := observationFailure(t, root)
			cause()
			r.stdout.waitFor(t, map[string]string{"ndjson": `"reason":"error"`, "text": "rescan (error)"}[format])
			cure()
			r.stdout.waitFor(t, map[string]string{"ndjson": `"event":"rescan.completed"`, "text": "rescan completed"}[format])
			if err := os.RemoveAll(filepath.Dir(root)); err != nil {
				t.Fatal(err)
			}
			select {
			case <-r.done:
			case <-time.After(watchWait):
				t.Fatal("the watch survived its root")
			}
			if r.code != 5 {
				t.Fatalf("exit %d", r.code)
			}
			output := r.stdout.String() + r.stderr.String()
			for _, secret := range []string{marker, home, strings.ReplaceAll(home, `\`, `\\`), os.TempDir()} {
				if secret != "" && strings.Contains(output, secret) {
					t.Fatalf("the output discloses %q:\n%s", secret, output)
				}
			}
		})
	}
}

// observationFailure returns how to make the next observation of root fail
// and how to end the failure: a Unicode normalization collision where the
// filesystem keeps both names, an unreadable directory on macOS.
func observationFailure(t *testing.T, root string) (cause, cure func()) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		decomposed := filepath.Join(root, "cafe\u0301")
		return func() { writeFiles(t, root, [][2]string{{"cafe\u0301", ""}}) },
			func() { _ = os.Remove(decomposed) }
	}
	if os.Geteuid() == 0 {
		t.Skip("permissions do not apply to root")
	}
	locked := filepath.Join(root, "locked")
	writeFiles(t, root, [][2]string{{"locked/inside", ""}})
	var once sync.Once
	cure = func() { once.Do(func() { _ = os.Chmod(locked, 0o755) }) }
	t.Cleanup(cure)
	return func() {
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, root, [][2]string{{"trigger/x", ""}})
	}, cure
}

// TestWatchOutputFailingAfterSomeBytes: the output accepts part of the
// stream, then fails. Exit status 5; what was written is a stream a
// consumer can use once it discards the partial last line.
func TestWatchOutputFailingAfterSomeBytes(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, [][2]string{{"a", ""}})
	loader := configuration.NewLoader(configuration.WithUserConfigDir(func() (string, error) {
		return "", errors.New("user configuration disabled in tests")
	}))
	out := &cutWriter{limit: 150}
	var stderr bytes.Buffer
	code := executeWithLoader(context.Background(), []string{"watch", root, "--format", "ndjson", "--poll", "100ms", "--no-config"},
		out, &stderr, "v0.1.0-test", loader)
	if code != 5 || !strings.HasPrefix(stderr.String(), "Error: write watch events: ") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	events, rest, err := watch.ParseNDJSON(out.Bytes())
	if err != nil || len(events) != 0 || len(rest) != 150 {
		t.Fatalf("a stream cut at 150 bytes: %d events, %d bytes left, %v", len(events), len(rest), err)
	}
}

// cutWriter accepts limit bytes, then fails, as a pipe that breaks in
// the middle of a write.
type cutWriter struct {
	bytes.Buffer
	limit int
}

func (w *cutWriter) Write(p []byte) (int, error) {
	room := w.limit - w.Len()
	if len(p) <= room {
		return w.Buffer.Write(p)
	}
	n, _ := w.Buffer.Write(p[:max(room, 0)])
	return n, errors.New("broken pipe")
}

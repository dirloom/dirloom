package cli

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"testing"
	"time"
)

// watchHelperArgs carries the dirloom arguments of a helper process,
// separated by line feeds.
const watchHelperArgs = "DIRLOOM_WATCH_HELPER_ARGS"

// TestWatchHelperProcess is not a test: the signal tests run the test binary
// through it as a dirloom process, with the signal handling of main.
func TestWatchHelperProcess(t *testing.T) {
	args := os.Getenv(watchHelperArgs)
	if args == "" {
		t.Skip("helper process for the watch signal tests")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := Execute(ctx, strings.Split(args, "\n"), os.Stdout, os.Stderr, "v0.1.0-test")
	stop()
	os.Exit(code)
}

// watchProcess is a dirloom watch running as a child process.
type watchProcess struct {
	cmd    *exec.Cmd
	lines  chan string
	stderr bytes.Buffer
}

func startWatchProcess(t *testing.T, root string) *watchProcess {
	t.Helper()
	p := &watchProcess{lines: make(chan string, 64)}
	p.cmd = exec.Command(os.Args[0], "-test.run=^TestWatchHelperProcess$") // #nosec G204 -- the test binary itself.
	p.cmd.Env = append(os.Environ(), watchHelperArgs+"="+strings.Join([]string{"watch", root, "--format", "ndjson", "--poll", "100ms", "--no-config"}, "\n"))
	p.cmd.Stderr = &p.stderr
	ownProcessGroup(p.cmd)
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.cmd.Process.Kill(); _ = p.cmd.Wait() })
	go func() {
		defer close(p.lines)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			p.lines <- scanner.Text()
		}
	}()
	if line := p.next(t); !strings.Contains(line, `"event":"watch.started"`) {
		t.Fatalf("first line %q", line)
	}
	return p
}

func (p *watchProcess) next(t *testing.T) string {
	t.Helper()
	select {
	case line, ok := <-p.lines:
		if !ok {
			t.Fatalf("the watch process ended early; stderr %q", p.stderr.String())
		}
		return line
	case <-time.After(watchWait):
		t.Fatal("no line from the watch process")
		return ""
	}
}

// endsInterrupted waits for the process to end and checks a controlled
// interruption: watch.stopped (interrupted) last, exit 0, nothing on stderr.
func (p *watchProcess) endsInterrupted(t *testing.T) {
	t.Helper()
	var last string
	for line := range p.lines {
		last = line
	}
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("exit: %v; stderr %q", err, p.stderr.String())
		}
	case <-time.After(watchWait):
		t.Fatal("the watch process did not exit")
	}
	if !strings.HasPrefix(last, `{"schemaVersion":1,"event":"watch.stopped","generation":0,"reason":"interrupted"`) || p.stderr.Len() != 0 {
		t.Fatalf("last line %q, stderr %q", last, p.stderr.String())
	}
}

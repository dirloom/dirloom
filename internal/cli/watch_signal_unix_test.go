//go:build unix

package cli

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func ownProcessGroup(*exec.Cmd) {}

// interruptProcess sends SIGINT, as Ctrl+C does.
func interruptProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
}

// endedByClosedOutput reports a process killed by SIGPIPE: Go's default
// for a write to a closed stdout.
func endedByClosedOutput(err error, _ string) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGPIPE
}

// TestWatchProcessEndsOnSignal checks the controlled interruption of a real
// process: SIGINT, and SIGTERM, which only watch handles.
func TestWatchProcessEndsOnSignal(t *testing.T) {
	for name, sig := range map[string]os.Signal{"SIGINT": os.Interrupt, "SIGTERM": syscall.SIGTERM} {
		t.Run(name, func(t *testing.T) {
			p := startWatchProcess(t, t.TempDir())
			if err := p.cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			p.endsInterrupted(t)
		})
	}
}

//go:build unix

package cli

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func ownProcessGroup(*exec.Cmd) {}

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

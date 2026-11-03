package cli

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// ownProcessGroup starts the child in its own process group, so that a
// console control event reaches it alone.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

// interruptProcess sends CTRL_BREAK_EVENT, which Go delivers as
// os.Interrupt.
func interruptProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	pid := uint32(cmd.Process.Pid) //nolint:gosec // G115: process identifiers are non-negative 32-bit values.
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, pid); err != nil {
		t.Fatal(err)
	}
}

// endedByClosedOutput reports exit status 5 with the diagnostic of a
// failed write: Windows has no SIGPIPE.
func endedByClosedOutput(err error, stderr string) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 5 && strings.HasPrefix(stderr, "Error: write watch events: ")
}

// TestWatchProcessEndsOnSignal checks the controlled interruption of a real
// process through CTRL_BREAK_EVENT, which Go delivers as os.Interrupt.
func TestWatchProcessEndsOnSignal(t *testing.T) {
	p := startWatchProcess(t, t.TempDir())
	interruptProcess(t, p.cmd)
	p.endsInterrupted(t)
}

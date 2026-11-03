package cli

import (
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// ownProcessGroup starts the child in its own process group, so that a
// console control event reaches it alone.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

// TestWatchProcessEndsOnSignal checks the controlled interruption of a real
// process through CTRL_BREAK_EVENT, which Go delivers as os.Interrupt.
func TestWatchProcessEndsOnSignal(t *testing.T) {
	p := startWatchProcess(t, t.TempDir())
	pid := uint32(p.cmd.Process.Pid) //nolint:gosec // G115: process identifiers are non-negative 32-bit values.
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, pid); err != nil {
		t.Fatal(err)
	}
	p.endsInterrupted(t)
}

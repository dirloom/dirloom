//go:build linux || darwin

package facts

import (
	"os"
	"os/exec"
)

func prepareGroup(*exec.Cmd) {}

func interruptChild(cmd *exec.Cmd) error { return cmd.Process.Signal(os.Interrupt) }

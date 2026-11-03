//go:build linux || darwin

package app

import (
	"syscall"
	"testing"
	"time"
)

// processCPU is the user and system CPU time this process has used.
func processCPU(tb testing.TB) time.Duration {
	tb.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		tb.Fatal(err)
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}

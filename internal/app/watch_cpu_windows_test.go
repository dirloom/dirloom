package app

import (
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// processCPU is the user and kernel CPU time this process has used.
func processCPU(tb testing.TB) time.Duration {
	tb.Helper()
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err != nil {
		tb.Fatal(err)
	}
	ticks := func(ft windows.Filetime) time.Duration { // 100 ns units
		return time.Duration(int64(ft.HighDateTime)<<32|int64(ft.LowDateTime)) * 100
	}
	return ticks(kernel) + ticks(user)
}

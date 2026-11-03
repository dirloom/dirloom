//go:build linux || darwin

package cli

import (
	"os"
	"runtime"
	"testing"
)

// handleSlack is what the process may keep open after a watch: nothing.
const handleSlack = 0

// openHandles counts this process's descriptors. Listing the descriptor
// directory works where stat-ing its entries does not (macOS).
func openHandles(t *testing.T) int {
	t.Helper()
	dir := "/proc/self/fd"
	if runtime.GOOS == "darwin" {
		dir = "/dev/fd"
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	names, err := f.Readdirnames(-1)
	if err != nil {
		t.Fatal(err)
	}
	return len(names)
}

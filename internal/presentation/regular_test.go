package presentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestThemeFileMustBeARegularFile: a theme path that names a device (the
// null device here, on every platform) is refused before it is read (F2 of
// the D7 audit of the v0.4.0-rc1 plan); its size of 0 used to pass the
// 1 MiB check.
func TestThemeFileMustBeARegularFile(t *testing.T) {
	_, err := loadFile(os.DevNull)
	if err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("theme %s = %v, want an invalid non-regular file", os.DevNull, err)
	}
}

// TestThemeReadStopsAfterTheLimit: the reader never reads more than one
// byte beyond its limit, whatever Stat said.
func TestThemeReadStopsAfterTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.yaml")
	if err := os.WriteFile(path, make([]byte, 3*maxThemeSize), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := readBounded(path, maxThemeSize)
	if err != nil || len(data) != maxThemeSize+1 {
		t.Fatalf("read %d bytes, %v; want %d", len(data), err, maxThemeSize+1)
	}
}

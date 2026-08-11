//go:build !windows

package output

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFilePreservesExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tree.txt")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("permissions = %o, want 600", got)
	}
}

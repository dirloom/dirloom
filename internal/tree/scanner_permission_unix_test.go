//go:build !windows

package tree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScannerPermissionErrorReturnsNoPartialTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission semantics are not meaningful as root")
	}
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(blocked, 0o700) }()

	scanner := NewScanner(ScanOptions{RootAbs: root, RootName: "root"})
	model, err := scanner.Scan(context.Background())
	if err == nil || model != nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("Scan() = (%#v, %v), want nil tree and permission error", model, err)
	}
}

//go:build !windows

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestThemeClassifyRejectsSpecialFilesystemEntry(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "events.pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := executeForTest(t, "theme", "classify", "events.pipe", "--root", root)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unsupported type") {
		t.Fatalf("special entry=(%q,%q,%d)", stdout, stderr, code)
	}
}

func TestThemeClassifyReportsPermissionFailureAsRuntimeError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can bypass directory permission checks")
	}
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "secret.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

	stdout, stderr, code := executeForTest(t, "theme", "classify", filepath.Join("blocked", "secret.go"), "--root", root)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "classification path") {
		t.Fatalf("permission failure=(%q,%q,%d)", stdout, stderr, code)
	}
}

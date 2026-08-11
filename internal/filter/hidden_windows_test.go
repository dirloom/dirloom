//go:build windows

package filter

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWindowsHiddenAttribute(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "hidden-by-attribute.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(pointer, syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.SetFileAttributes(pointer, syscall.FILE_ATTRIBUTE_NORMAL) }()

	entry, info := readEntry(t, root, "hidden-by-attribute.txt")
	if !isHidden(path, entry.Name(), info) {
		t.Fatal("FILE_ATTRIBUTE_HIDDEN was not recognized")
	}
}

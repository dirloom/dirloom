package output

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileCreatesAndReplaces(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tree.txt")
	if err := WriteFile(path, []byte("first\n")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("second\n")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "second\n" {
		t.Fatalf("content = %q", data)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".tree.txt.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files leaked: %#v", matches)
	}
}

func TestWriteFileDoesNotCreateParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "tree.txt")
	if err := WriteFile(path, nil); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestWriteFileRefusesSymlinkDestination(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	link := filepath.Join(root, "link.txt")
	if err := os.WriteFile(target, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.txt", link); err != nil {
		t.Skipf("symlinks unavailable on %s: %v", runtime.GOOS, err)
	}
	if err := WriteFile(link, []byte("replace")); err == nil {
		t.Fatal("symlink destination should be rejected")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "preserve" {
		t.Fatalf("symlink target was changed: %q", data)
	}
}

func TestWriteFileRefusesNonRegularDestination(t *testing.T) {
	root := t.TempDir()
	if err := WriteFile(root, nil); err == nil {
		t.Fatal("directory destination should be rejected")
	}
}

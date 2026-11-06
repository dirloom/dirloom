//go:build !windows

package source

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestFilesystemRefusesNamedPipe(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "x")
	if err := syscall.Mkfifo(filepath.Join(root, "events.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := observeFSErr(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
	var artErr *artifact.Error
	if !errors.As(err, &artErr) || artErr.Code != artifact.CodeUnsupportedKind || !strings.Contains(artErr.Message, "named pipe") {
		t.Fatalf("FIFO observation error = %v, want %s", err, artifact.CodeUnsupportedKind)
	}
	// The scan itself still lists the FIFO as a file, as tree renderings
	// always have.
	model, err := tree.NewScanner(tree.ScanOptions{RootAbs: root, RootName: "proj"}).Scan(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range model.Children {
		if child.Name == "events.pipe" && (child.Type != tree.NodeFile || child.Special != "named pipe") {
			t.Fatalf("FIFO node = %#v", child)
		}
	}
}

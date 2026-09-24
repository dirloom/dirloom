//go:build !windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/snapshot"
)

func TestSnapshotPOSIXBackslashFilenameFilesystem(t *testing.T) {
	root := t.TempDir()
	name := `a\b.txt`
	if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
		t.Skipf("backslash filename unavailable: %v", err)
	}
	result, err := Snapshot(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := snapshot.LoadBytes(result.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range loaded.Document.Artifact.Nodes {
		if node.Path == name && node.Kind == "file" {
			return
		}
	}
	t.Fatalf("POSIX backslash filename missing: %#v", loaded.Document.Artifact.Nodes)
}

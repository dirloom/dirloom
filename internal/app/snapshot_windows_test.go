//go:build windows

package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/snapshot"
)

func TestSnapshotWindowsJunctionRoundTrip(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "inside.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(root, "junction")
	cmd := exec.Command("cmd", "/c", "mklink", "/J", junction, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("junctions unavailable: %v\n%s", err, output)
	}
	result, err := Snapshot(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := snapshot.LoadBytes(result.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	var found *snapshot.NodeV1
	for i := range loaded.Document.Artifact.Nodes {
		if loaded.Document.Artifact.Nodes[i].Path == "junction" {
			found = &loaded.Document.Artifact.Nodes[i]
		}
	}
	if found == nil {
		t.Fatalf("junction missing: %#v", loaded.Document.Artifact.Nodes)
	}
	if found.Kind == "symlink" && (found.Target == nil || len(loaded.Artifact.Root.Children) == 0) {
		t.Fatal("symlink junction lost its target")
	}
	for _, node := range loaded.Document.Artifact.Nodes {
		if len(node.Path) > len("junction/") && node.Path[:len("junction/")] == "junction/" && found.Kind == "symlink" {
			t.Fatalf("symlink junction grew child %q", node.Path)
		}
	}
}

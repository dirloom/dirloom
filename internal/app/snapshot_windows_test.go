//go:build windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	makeJunction(t, filepath.Join(root, "junction"), target)
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
		node := &loaded.Document.Artifact.Nodes[i]
		if node.Path == "junction" {
			found = node
		}
		if strings.HasPrefix(node.Path, "junction/") {
			t.Fatalf("junction grew child %q", node.Path)
		}
	}
	if found == nil || found.Kind != "junction" || found.Target == nil || *found.Target != "target" {
		t.Fatalf("junction node = %+v, want kind junction with target %q", found, "target")
	}
}

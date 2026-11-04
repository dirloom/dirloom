//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/snapshot"
)

func TestVerifyWindowsPathSeparatorsAndRelocation(t *testing.T) {
	parent := t.TempDir()
	root := writeTree(t, filepath.Join(parent, "original"), [][2]string{{"src/main.go", "package main\n"}})
	snap := mustFilesystemSnapshot(t, root)
	slash := strings.ReplaceAll(root, `\`, `/`)
	assertSnapshotStatus(t, snap, slash, VerifyMatch)
	moved := filepath.Join(parent, "relocated")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	assertSnapshotStatus(t, snap, moved, VerifyMatch)
}

// TestVerifyWindowsJunctionRetarget checks that the junction target is part
// of identity: retargeting it mismatches, and restoring it matches again.
func TestVerifyWindowsJunctionRetarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "inside.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(root, "junction")
	makeJunction(t, junction, target)
	snap := mustFilesystemSnapshot(t, root)
	assertSnapshotStatus(t, snap, root, VerifyMatch)
	loaded, err := snapshot.LoadBytes(snap)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, node := range loaded.Document.Artifact.Nodes {
		if node.Path == "junction" {
			found = node.Kind == "junction" && node.Target != nil && *node.Target == "target"
		}
	}
	if !found {
		t.Fatalf("junction node = %+v, want kind junction with target %q", loaded.Document.Artifact.Nodes, "target")
	}

	retarget := func(to string) {
		t.Helper()
		if err := os.Remove(junction); err != nil {
			t.Fatal(err)
		}
		makeJunction(t, junction, to)
	}
	retarget(other)
	assertSnapshotStatus(t, snap, root, VerifyMismatch)
	retarget(target)
	assertSnapshotStatus(t, snap, root, VerifyMatch)
}

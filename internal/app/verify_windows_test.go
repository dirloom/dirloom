//go:build windows

package app

import (
	"os"
	"os/exec"
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

func TestVerifyWindowsJunctionInheritsScannerKind(t *testing.T) {
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
	snap := mustFilesystemSnapshot(t, root)
	assertSnapshotStatus(t, snap, root, VerifyMatch)
	loaded, err := snapshot.LoadBytes(snap)
	if err != nil {
		t.Fatal(err)
	}
	var kind string
	for _, node := range loaded.Document.Artifact.Nodes {
		if node.Path == "junction" {
			kind = node.Kind
		}
	}
	if kind == "" {
		t.Fatal("junction missing from snapshot")
	}
	switch kind {
	case "symlink", "junction":
		other := filepath.Join(root, "other")
		if err := os.Mkdir(other, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(junction); err != nil {
			t.Fatal(err)
		}
		relink := exec.Command("cmd", "/c", "mklink", "/J", junction, other)
		if output, err := relink.CombinedOutput(); err != nil {
			t.Fatalf("relink junction: %v\n%s", err, output)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	case "directory":
		if err := os.WriteFile(filepath.Join(target, "added.txt"), []byte("y"), 0o644); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	case "file":
		if err := os.Remove(junction); err != nil {
			t.Fatal(err)
		}
		assertSnapshotStatus(t, snap, root, VerifyMismatch)
	default:
		t.Fatalf("unexpected junction kind %q", kind)
	}
}

//go:build windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/snapshot"
)

// TestDiffWindowsJunctionFollowsScannerKind mirrors the verify junction test:
// the diff outcome depends on the kind the scanner assigns to junctions.
func TestDiffWindowsJunctionFollowsScannerKind(t *testing.T) {
	store := t.TempDir()
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
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, root))
	diff := mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if DiffStatusOf(diff) != DiffNoDifferences {
		t.Fatalf("unchanged junction = %+v", diff.Changes)
	}

	loaded, err := snapshot.LoadBytes(mustFilesystemSnapshot(t, root))
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

	request := func() DiffRequest {
		return DiffRequest{
			A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
			B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
		}
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
		diff = mustDiff(t, request())
		// The retargeted junction is a CHANGED entry; the new "other"
		// directory is an ADDED entry.
		if diff.Summary.Changed != 1 || diff.Summary.Added != 1 {
			t.Fatalf("retargeted junction = %+v", diff.Changes)
		}
		if diff.Changes[0].Path != "junction" || diff.Changes[0].Op != comparison.OpChanged {
			t.Fatalf("junction change = %+v", diff.Changes[0])
		}
	case "directory":
		if err := os.WriteFile(filepath.Join(target, "added.txt"), []byte("y"), 0o644); err != nil {
			t.Fatal(err)
		}
		diff = mustDiff(t, request())
		if DiffStatusOf(diff) != DiffDifferences {
			t.Fatalf("junction content = %+v", diff.Changes)
		}
	case "file":
		if err := os.Remove(junction); err != nil {
			t.Fatal(err)
		}
		diff = mustDiff(t, request())
		if DiffStatusOf(diff) != DiffDifferences {
			t.Fatalf("junction removal = %+v", diff.Changes)
		}
	default:
		t.Fatalf("unexpected junction kind %q", kind)
	}
}

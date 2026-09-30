//go:build unix

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/comparison"
)

func TestDiffUnixSymlinkTargetChange(t *testing.T) {
	store := t.TempDir()
	root := t.TempDir()
	if err := os.Symlink("target-a", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, root))
	diff := mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if DiffStatusOf(diff) != DiffNoDifferences {
		t.Fatalf("unchanged symlink = %+v", diff.Changes)
	}

	if err := os.Remove(filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target-b", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	diff = mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if len(diff.Changes) != 1 || diff.Changes[0].Op != comparison.OpChanged || diff.Changes[0].Path != "link" {
		t.Fatalf("target change = %+v", diff.Changes)
	}
	change := diff.Changes[0]
	if change.Before == nil || change.After == nil || change.Before.Target == nil || change.After.Target == nil {
		t.Fatalf("target presence = %+v", change)
	}
	if *change.Before.Target != "target-a" || *change.After.Target != "target-b" {
		t.Fatalf("targets = %q -> %q", *change.Before.Target, *change.After.Target)
	}
}

func TestDiffUnixBrokenSymlink(t *testing.T) {
	store := t.TempDir()
	root := t.TempDir()
	if err := os.Symlink("missing-target", filepath.Join(root, "broken")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, root))
	diff := mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if DiffStatusOf(diff) != DiffNoDifferences {
		t.Fatalf("broken symlink = %+v", diff.Changes)
	}
}

func TestDiffUnixKindFlips(t *testing.T) {
	store := t.TempDir()
	root := writeTree(t, t.TempDir(), [][2]string{{"node", "content"}})
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, root))
	if err := os.Remove(filepath.Join(root, "node")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(root, "node")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	diff := mustDiff(t, DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	})
	if len(diff.Changes) != 1 || diff.Changes[0].Op != comparison.OpChanged {
		t.Fatalf("kind flip = %+v", diff.Changes)
	}
	change := diff.Changes[0]
	if change.Before.Kind != "file" || change.After.Kind != "symlink" {
		t.Fatalf("kinds = %s -> %s", change.Before.Kind, change.After.Kind)
	}
	if change.Before.Target != nil || change.After.Target == nil || *change.After.Target != "elsewhere" {
		t.Fatalf("target presence = %+v", change)
	}
}

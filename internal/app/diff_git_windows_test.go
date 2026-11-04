//go:build windows

package app

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/gittest"
)

// TestDiffGitEquivalenceExceptionHiddenAttribute shows the last documented
// exception: a Windows hidden attribute hides a checked-out entry from a
// filesystem source, never from a Git source.
func TestDiffGitEquivalenceExceptionHiddenAttribute(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("a.txt", "a")
	repo.Write("visible.txt", "hidden by attribute only")
	repo.CommitAll("first")
	path, err := syscall.UTF16PtrFromString(filepath.Join(repo.Dir, "visible.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(path, syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
	snap := checkoutSnapshot(t, repo, defaultCapture)
	t.Chdir(repo.Dir)
	requireChanges(t, mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")}), "ADDED visible.txt")
}

// TestDiffGitWindowsJunctionInTheWorkingTree shows that a Git source never
// reports a junction: Git stages what it reads through a working-tree
// junction, never the junction itself, and never checks one out. The
// filesystem source records the junction.
func TestDiffGitWindowsJunctionInTheWorkingTree(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("target/inside.txt", "x")
	makeJunction(t, filepath.Join(repo.Dir, "j"), filepath.Join(repo.Dir, "target"))
	repo.CommitAll("junction")
	snap := checkoutSnapshot(t, repo, defaultCapture)
	t.Chdir(repo.Dir)
	diff := mustDiff(t, DiffRequest{A: snap, B: gitSpec("HEAD")})
	recorded := false
	for _, change := range diff.Changes {
		if change.After != nil && change.After.Kind == artifact.KindJunction {
			t.Fatalf("the Git source reports a junction: %+v", change)
		}
		if change.Path == "j" && change.Before != nil && change.Before.Kind == artifact.KindJunction {
			recorded = true
		}
	}
	if !recorded {
		t.Fatalf("changes %q, want the checkout's junction j to differ from the revision", changeLines(diff))
	}
}

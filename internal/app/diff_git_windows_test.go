//go:build windows

package app

import (
	"path/filepath"
	"syscall"
	"testing"

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

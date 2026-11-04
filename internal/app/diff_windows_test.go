//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiffWindowsJunctionRetarget mirrors the verify junction test: a
// junction is kind junction, and retargeting it is a CHANGED entry.
func TestDiffWindowsJunctionRetarget(t *testing.T) {
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
	makeJunction(t, junction, target)
	snap := mustDiffSnapshotFile(t, store, "ref.dlm.json", mustFilesystemSnapshot(t, root))
	request := DiffRequest{
		A: DiffSourceSpec{Kind: DiffSourceSnapshot, Path: snap},
		B: DiffSourceSpec{Kind: DiffSourceLive, Path: root},
	}
	diff := mustDiff(t, request)
	if DiffStatusOf(diff) != DiffNoDifferences {
		t.Fatalf("unchanged junction = %+v", diff.Changes)
	}

	// A write through the junction lands in its target, which is a change
	// of target/, never of the junction.
	if err := os.WriteFile(filepath.Join(junction, "through.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(describeChanges(mustDiff(t, request).Changes), "; "); got != "ADDED target/through.txt" {
		t.Fatalf("write through the junction = %s", got)
	}
	if err := os.Remove(filepath.Join(target, "through.txt")); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(root, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(junction); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, junction, other)
	// The retargeted junction is a CHANGED entry; the new "other" directory
	// is an ADDED entry.
	if got := strings.Join(describeChanges(mustDiff(t, request).Changes), "; "); got != "ADDED other; CHANGED junction (junction:target->junction:other)" {
		t.Fatalf("retargeted junction = %s", got)
	}

	if err := os.Remove(junction); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(describeChanges(mustDiff(t, request).Changes), "; "); got != "ADDED other; REMOVED junction" {
		t.Fatalf("junction removal = %s", got)
	}
}

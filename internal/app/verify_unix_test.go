//go:build unix

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func TestVerifyUnixSymlinks(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("target-a", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	snap := mustFilesystemSnapshot(t, root)
	assertSnapshotStatus(t, snap, root, VerifyMatch)

	if err := os.Remove(filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target-b", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	assertSnapshotStatus(t, snap, root, VerifyMismatch)
}

func TestVerifyUnixBrokenSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("missing-target", filepath.Join(root, "broken")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	snap := mustFilesystemSnapshot(t, root)
	assertSnapshotStatus(t, snap, root, VerifyMatch)
}

func TestVerifyUnixFileBecomesSymlink(t *testing.T) {
	root := writeTree(t, t.TempDir(), [][2]string{{"node", "content"}})
	snap := mustFilesystemSnapshot(t, root)
	if err := os.Remove(filepath.Join(root, "node")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(root, "node")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	assertSnapshotStatus(t, snap, root, VerifyMismatch)
}

func TestVerifyUnixBackslashFilename(t *testing.T) {
	root := t.TempDir()
	name := "weird\\name.txt"
	if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := mustFilesystemSnapshot(t, root)
	assertSnapshotStatus(t, snap, root, VerifyMatch)
	validated, err := snapshot.LoadBytes(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !artifactContains(validated.Artifact, artifact.Path(name)) {
		t.Fatalf("backslash filename was rewritten: %#v", validated.Artifact.Root.Children)
	}
}

func TestVerifyUnixCanonicalCollision(t *testing.T) {
	root := t.TempDir()
	nfc := "\u00e9.txt"
	nfd := "e\u0301.txt"
	if err := os.WriteFile(filepath.Join(root, nfc), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, nfd), []byte("b"), 0o644); err != nil {
		t.Skipf("filesystem rejected NFD name: %v", err)
	}
	left, err := os.Stat(filepath.Join(root, nfc))
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.Stat(filepath.Join(root, nfd))
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(left, right) {
		t.Skip("filesystem collapsed NFC and NFD names")
	}
	_, err = Snapshot(context.Background(), InspectRequest{Root: root, UseDefaultIgnores: false, UseGitIgnore: false})
	if err == nil {
		t.Fatal("expected a canonical collision")
	}
	if ClassifyVerifyFailure(err) != FailureObservation {
		t.Fatalf("collision = %v", err)
	}
	code, _ := VerifyDiagnostic(err)
	if code != artifact.CodeCollision {
		t.Fatalf("code = %s", code)
	}
}

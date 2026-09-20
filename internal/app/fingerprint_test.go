package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestFingerprintMemorySource(t *testing.T) {
	art := artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{{Path: "a.txt", Name: "a.txt", Kind: artifact.KindFile}},
	}}
	result, err := FingerprintFromSource(context.Background(), source.Memory{Artifact: art})
	if err != nil {
		t.Fatal(err)
	}
	if result.NodeCount != 2 || result.SourceKind != source.KindMemory {
		t.Fatalf("%#v", result)
	}
	if _, err := identity.Parse(result.Fingerprint.String()); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintObservesOnce(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	scans := 0
	src := source.Filesystem{
		RootAbs: root,
		Scan: func(ctx context.Context) (*tree.Node, error) {
			scans++
			return Inspect(ctx, InspectRequest{Root: root})
		},
	}
	if _, err := FingerprintFromSource(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if scans != 1 {
		t.Fatalf("scans = %d, want 1", scans)
	}
}

func TestFingerprintCrossRootAndContent(t *testing.T) {
	payload := [][2]string{{"src/main.go", "package main\n"}, {"README.md", "hi\n"}}
	a := writeTree(t, t.TempDir(), payload)
	b := writeTree(t, filepath.Join(t.TempDir(), "completely-different-name"), payload)
	left, err := Fingerprint(context.Background(), InspectRequest{Root: a, UseDefaultIgnores: true})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Fingerprint(context.Background(), InspectRequest{Root: b, UseDefaultIgnores: true})
	if err != nil {
		t.Fatal(err)
	}
	if left.Fingerprint != right.Fingerprint {
		t.Fatalf("cross-root %s vs %s", left.Fingerprint, right.Fingerprint)
	}
	if err := os.WriteFile(filepath.Join(a, "src", "main.go"), []byte("package main\n// changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changedContent, err := Fingerprint(context.Background(), InspectRequest{Root: a, UseDefaultIgnores: true})
	if err != nil {
		t.Fatal(err)
	}
	if changedContent.Fingerprint != left.Fingerprint {
		t.Fatal("content-only change altered fingerprint")
	}
	if err := os.Mkdir(filepath.Join(a, "src", "payments"), 0o755); err != nil {
		t.Fatal(err)
	}
	structural, err := Fingerprint(context.Background(), InspectRequest{Root: a, UseDefaultIgnores: true})
	if err != nil {
		t.Fatal(err)
	}
	if structural.Fingerprint == left.Fingerprint {
		t.Fatal("structural addition kept fingerprint")
	}
}

func TestFingerprintFilterChangesIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "generated", "out.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	all, err := Fingerprint(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := Fingerprint(context.Background(), InspectRequest{Root: root, IgnorePatterns: []string{"generated"}})
	if err != nil {
		t.Fatal(err)
	}
	if all.Fingerprint == filtered.Fingerprint {
		t.Fatal("ignore rule did not change fingerprint")
	}
}

func TestFingerprintMeaningfulChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := Fingerprint(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := Fingerprint(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if added.Fingerprint == before.Fingerprint {
		t.Fatal("add file kept fingerprint")
	}
	if err := os.Remove(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	removed, err := Fingerprint(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if removed.Fingerprint != before.Fingerprint {
		t.Fatal("remove file did not restore fingerprint")
	}
	if err := os.Rename(filepath.Join(root, "a.txt"), filepath.Join(root, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	renamed, err := Fingerprint(context.Background(), InspectRequest{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Fingerprint == before.Fingerprint {
		t.Fatal("rename kept fingerprint")
	}
}

func writeTree(t *testing.T, root string, files [][2]string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file[0]))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file[1]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

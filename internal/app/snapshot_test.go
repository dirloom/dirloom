package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestSnapshotObservesOnce(t *testing.T) {
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
	result, err := SnapshotFromSource(context.Background(), src, CaptureFromInspect(InspectRequest{Root: root}))
	if err != nil {
		t.Fatal(err)
	}
	if scans != 1 {
		t.Fatalf("scans=%d", scans)
	}
	loaded, err := snapshot.LoadBytes(result.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Fingerprint != result.Fingerprint {
		t.Fatal("fingerprint mismatch")
	}
}

func TestSnapshotCaptureExcludesOutputPath(t *testing.T) {
	req := InspectRequest{
		Root: ".", MaxDepth: intPtr(2), DirectoriesOnly: true, IncludeHidden: true,
		IgnorePatterns: []string{"a", "b"}, UseDefaultIgnores: false, UseGitIgnore: false,
		OutputPath: "architecture.dlm.json",
	}
	capture := CaptureFromInspect(req)
	if capture.Depth == nil || *capture.Depth != 2 || !capture.DirsOnly || !capture.Hidden {
		t.Fatalf("%#v", capture)
	}
	if capture.UseDefaultIgnores || capture.UseGitignore {
		t.Fatalf("%#v", capture)
	}
	if len(capture.Ignore) != 2 || capture.Ignore[0] != "a" {
		t.Fatalf("%#v", capture.Ignore)
	}
}

func TestSnapshotRelocationAndContentInvariance(t *testing.T) {
	payload := [][2]string{{"src/main.go", "package main\n"}, {"README.md", "hi\n"}}
	a := writeTree(t, t.TempDir(), payload)
	b := writeTree(t, filepath.Join(t.TempDir(), "other-root-name"), payload)
	left, err := Snapshot(context.Background(), InspectRequest{Root: a, UseDefaultIgnores: true, UseGitIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Snapshot(context.Background(), InspectRequest{Root: b, UseDefaultIgnores: true, UseGitIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left.Bytes, right.Bytes) {
		t.Fatal("relocation changed snapshot bytes")
	}
	if err := os.WriteFile(filepath.Join(a, "src", "main.go"), []byte("package main\n// edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	content, err := Snapshot(context.Background(), InspectRequest{Root: a, UseDefaultIgnores: true, UseGitIgnore: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content.Bytes, left.Bytes) {
		t.Fatal("content-only change altered snapshot")
	}
}

func TestSnapshotMemorySource(t *testing.T) {
	art := artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{{Path: "a.txt", Name: "a.txt", Kind: artifact.KindFile}},
	}}
	result, err := SnapshotFromSource(context.Background(), source.Memory{Artifact: art}, snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.NodeCount != 2 || result.SourceKind != source.KindMemory {
		t.Fatalf("%#v", result)
	}
}

func intPtr(v int) *int { return &v }

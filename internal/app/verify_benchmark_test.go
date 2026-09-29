package app

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/dirloom/dirloom/internal/source"
)

func BenchmarkVerifyMemoryCompare(b *testing.B) {
	for _, size := range []int{1000, 10000, 100000} {
		art := wideArtifact(size)
		raw := mustSnapshotBytes(b, art, snapshot.CaptureV1{Ignore: []string{}})
		validated, err := snapshot.LoadBytes(raw)
		if err != nil {
			b.Fatal(err)
		}
		src := source.Memory{Artifact: art}
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := VerifyValidatedFromSource(context.Background(), validated, src)
				if err != nil || result.Status != VerifyMatch {
					b.Fatalf("%s %v", result.Status, err)
				}
			}
		})
	}
}

func BenchmarkVerifyFilesystem(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		root := b.TempDir()
		writeSyntheticTree(b, root, size)
		raw := mustFilesystemSnapshotWith(b, root, InspectRequest{Root: root, UseDefaultIgnores: true})
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result, err := Verify(context.Background(), VerifyRequest{Snapshot: bytes.NewReader(raw), Root: root})
				if err != nil || result.Status != VerifyMatch {
					b.Fatalf("%s %v", result.Status, err)
				}
			}
		})
	}
}

func wideArtifact(n int) artifact.Artifact {
	children := make([]artifact.Node, n)
	for i := range children {
		name := "f" + itoa(i) + ".txt"
		children[i] = fileNode(name)
	}
	return directoryArtifact(children...)
}

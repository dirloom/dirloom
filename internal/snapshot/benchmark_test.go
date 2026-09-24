package snapshot_test

import (
	"fmt"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func BenchmarkSnapshotPipeline(b *testing.B) {
	for _, size := range []int{1000, 10000, 100000} {
		art := wideArtifact(size)
		capture := snapshot.CaptureV1{UseDefaultIgnores: true, UseGitignore: true, Ignore: []string{}}
		b.Run(fmt.Sprintf("project/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := snapshot.ProjectV1(art); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("build/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := snapshot.Build(art, capture); err != nil {
					b.Fatal(err)
				}
			}
		})
		doc, err := snapshot.Build(art, capture)
		if err != nil {
			b.Fatal(err)
		}
		encoded, err := snapshot.Marshal(doc)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("serialize/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(encoded)))
			for i := 0; i < b.N; i++ {
				if _, err := snapshot.Marshal(doc); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("decode/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(encoded)))
			for i := 0; i < b.N; i++ {
				if _, err := snapshot.DecodeBytes(encoded); err != nil {
					b.Fatal(err)
				}
			}
		})
		decoded, err := snapshot.DecodeBytes(encoded)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("validate/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := snapshot.Validate(decoded); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("selfverify/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				art2, err := snapshot.Reconstruct(decoded.Artifact)
				if err != nil {
					b.Fatal(err)
				}
				fp, err := identity.Compute(art2)
				if err != nil {
					b.Fatal(err)
				}
				if fp.String() != decoded.Fingerprint {
					b.Fatal("mismatch")
				}
			}
		})
		b.Run(fmt.Sprintf("full-build-serialize/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				doc, err := snapshot.Build(art, capture)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := snapshot.Marshal(doc); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("full-load-validate/%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(encoded)))
			for i := 0; i < b.N; i++ {
				if _, err := snapshot.LoadBytes(encoded); err != nil {
					b.Fatal(err)
				}
			}
		})
		if size == 100000 {
			b.Logf("100k serialized size = %d bytes", len(encoded))
		}
	}
	deep := deepArtifact(200)
	capture := snapshot.CaptureV1{Ignore: []string{}}
	b.Run("build/deep200", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := snapshot.Build(deep, capture); err != nil {
				b.Fatal(err)
			}
		}
	})
	wide := wideArtifact(5000)
	b.Run("build/wide5k", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := snapshot.Build(wide, capture); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestLargeSnapshot100k(t *testing.T) {
	art := wideArtifact(100000)
	doc, err := snapshot.Build(art, snapshot.CaptureV1{Ignore: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := snapshot.LoadBytes(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Artifact.CountNodes() != 100000 {
		t.Fatalf("nodes=%d", loaded.Artifact.CountNodes())
	}
	t.Logf("100k snapshot bytes=%d fingerprint=%s", len(encoded), loaded.Fingerprint)
}

func wideArtifact(n int) artifact.Artifact {
	children := make([]artifact.Node, 0, n-1)
	for i := 0; i < n-1; i++ {
		name := fmt.Sprintf("f%06d.txt", i)
		children = append(children, artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindFile})
	}
	return artifact.Artifact{Root: artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory, Children: children}}
}

func deepArtifact(depth int) artifact.Artifact {
	node := artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}
	current := &node
	path := "."
	for i := 0; i < depth; i++ {
		name := fmt.Sprintf("d%d", i)
		if path == "." {
			path = name
		} else {
			path = path + "/" + name
		}
		child := artifact.Node{Path: artifact.Path(path), Name: name, Kind: artifact.KindDirectory}
		current.Children = []artifact.Node{child}
		current = &current.Children[0]
	}
	return artifact.Artifact{Root: node}
}

package comparison

import (
	"context"
	"fmt"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/source"
)

// BenchmarkCompare measures Identity Projection v1 plus the two-way merge and
// model validation on in-memory artifacts. No filesystem scan is involved.
func BenchmarkCompare(b *testing.B) {
	for _, size := range []int{1000, 10000, 100000} {
		for _, changedPercent := range []int{1, 50, 100} {
			base := benchmarkArtifact(size, 0)
			mutated := benchmarkArtifact(size, changedPercent)
			sourceA := source.Memory{Artifact: base}
			sourceB := source.Memory{Artifact: mutated}
			wantChanges := size * changedPercent / 100
			b.Run(fmt.Sprintf("%d/%d%%", size, changedPercent), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					diff, err := Compare(context.Background(), sourceA, sourceB, noMoves)
					if err != nil {
						b.Fatal(err)
					}
					if len(diff.Changes) != wantChanges {
						b.Fatalf("changes = %d, want %d", len(diff.Changes), wantChanges)
					}
				}
			})
		}
	}
}

// BenchmarkCompareMoves measures Compare without move detection and with
// structural-v1 on in-memory artifacts of about size records: no change, a
// share of modules relocated to a new parent, and generated folders that are
// all ambiguous. Projection, merge, detection and validation are included;
// no filesystem scan is involved.
func BenchmarkCompareMoves(b *testing.B) {
	scenarios := []struct {
		name  string
		build func(size int) (artifact.Artifact, artifact.Artifact, int)
	}{
		{"unchanged", func(size int) (artifact.Artifact, artifact.Artifact, int) {
			return moduleArtifact(size, 0), moduleArtifact(size, 0), 0
		}},
		{"relocated-1%", func(size int) (artifact.Artifact, artifact.Artifact, int) {
			return moduleArtifact(size, 0), moduleArtifact(size, 1), size / 10 / 100
		}},
		{"relocated-50%", func(size int) (artifact.Artifact, artifact.Artifact, int) {
			return moduleArtifact(size, 0), moduleArtifact(size, 50), size / 10 * 50 / 100
		}},
		{"ambiguous", func(size int) (artifact.Artifact, artifact.Artifact, int) {
			return generatedArtifact(size, false), generatedArtifact(size, true), 0
		}},
	}
	for _, size := range []int{1000, 10000, 100000} {
		for _, scenario := range scenarios {
			base, mutated, wantMoved := scenario.build(size)
			sourceA := source.Memory{Artifact: base}
			sourceB := source.Memory{Artifact: mutated}
			for _, opts := range []Options{noMoves, structuralV1} {
				want := 0
				if opts.MoveDetection == MoveDetectionStructuralV1 {
					want = wantMoved
				}
				b.Run(fmt.Sprintf("%d/%s/%s", size, scenario.name, opts.MoveDetection), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						diff, err := Compare(context.Background(), sourceA, sourceB, opts)
						if err != nil {
							b.Fatal(err)
						}
						if diff.Summary.Moved != want {
							b.Fatalf("moved = %d, want %d", diff.Summary.Moved, want)
						}
					}
				})
			}
		}
	}
}

// moduleArtifact builds src/m<i>/f<j>.go: size/10 modules of nine files.
// The first relocatedPercent percent of modules sit under moved/ instead of
// src/, so each relocated module is one uniquely named subtree.
func moduleArtifact(size, relocatedPercent int) artifact.Artifact {
	modules := size / 10
	relocated := modules * relocatedPercent / 100
	var src, moved []artifact.Node
	for i := 0; i < modules; i++ {
		parent := "src"
		if i < relocated {
			parent = "moved"
		}
		name := "m" + itoa(i)
		path := parent + "/" + name
		files := make([]artifact.Node, 9)
		for j := range files {
			file := "f" + itoa(j) + ".go"
			files[j] = fileNodeAt(path+"/"+file, file)
		}
		module := directoryNodeAt(path, name, files...)
		if i < relocated {
			moved = append(moved, module)
		} else {
			src = append(src, module)
		}
	}
	children := []artifact.Node{directoryNode("src", src...)}
	if len(moved) > 0 {
		children = append(children, directoryNode("moved", moved...))
	}
	return directoryArtifact(children...)
}

// generatedArtifact builds pkg<i>/gen/ for size/2 packages, or
// pkg<i>/build/gen/ when rebuilt: every gen/ is removed and re-added under a
// key repeated in every package, so no candidate is unique.
func generatedArtifact(size int, rebuilt bool) artifact.Artifact {
	packages := make([]artifact.Node, size/2)
	for i := range packages {
		name := "pkg" + itoa(i)
		gen := directoryNodeAt(name+"/gen", "gen")
		if rebuilt {
			gen = directoryNodeAt(name+"/build", "build", directoryNodeAt(name+"/build/gen", "gen"))
		}
		packages[i] = directoryNodeAt(name, name, gen)
	}
	return directoryArtifact(packages...)
}

// benchmarkArtifact builds a flat artifact of size files; the first
// changedPercent percent flip from file to symlink, producing one CHANGED
// entry each.
func benchmarkArtifact(size, changedPercent int) artifact.Artifact {
	changed := size * changedPercent / 100
	children := make([]artifact.Node, size)
	for i := range children {
		name := "f" + itoa(i) + ".txt"
		if i < changed {
			children[i] = symlinkNode(name, "target")
		} else {
			children[i] = fileNode(name)
		}
	}
	return directoryArtifact(children...)
}

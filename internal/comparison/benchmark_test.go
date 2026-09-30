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
					diff, err := Compare(context.Background(), sourceA, sourceB)
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

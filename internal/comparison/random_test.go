package comparison

import (
	"math/rand/v2"

	"github.com/dirloom/dirloom/internal/artifact"
)

// newSeededRand returns a deterministic generator for property tests.
func newSeededRand() *rand.Rand {
	return rand.New(rand.NewPCG(11, 23))
}

// randomArtifact builds a pseudo-random valid artifact: unique sibling names,
// NFC-safe ASCII segments, bounded depth.
func randomArtifact(rng *rand.Rand) artifact.Artifact {
	return directoryArtifact(randomChildren(rng, artifact.RootPath, 0)...)
}

func randomChildren(rng *rand.Rand, parent artifact.Path, depth int) []artifact.Node {
	count := rng.IntN(5)
	out := make([]artifact.Node, 0, count)
	for i := 0; i < count; i++ {
		name := "n" + itoa(i)
		path := artifact.Path(name)
		if parent != artifact.RootPath {
			path = artifact.Path(string(parent) + "/" + name)
		}
		switch rng.IntN(3) {
		case 0:
			out = append(out, artifact.Node{Path: path, Name: name, Kind: artifact.KindFile})
		case 1:
			out = append(out, artifact.Node{Path: path, Name: name, Kind: artifact.KindSymlink, Target: "target-" + itoa(rng.IntN(4))})
		default:
			var children []artifact.Node
			if depth < 3 {
				children = randomChildren(rng, path, depth+1)
			}
			out = append(out, artifact.Node{Path: path, Name: name, Kind: artifact.KindDirectory, Children: children})
		}
	}
	return out
}

// artifactFromBytes builds a deterministic valid artifact from a byte stream
// for fuzzing. Names stay unique by construction; byte value 3 skips a slot so
// the two halves of one input produce asymmetric trees.
func artifactFromBytes(data []byte) artifact.Artifact {
	if len(data) > 512 {
		data = data[:512]
	}
	var children []artifact.Node
	for i, b := range data {
		name := "n" + itoa(i)
		switch b % 4 {
		case 0:
			children = append(children, artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindFile})
		case 1:
			children = append(children, artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindSymlink, Target: "t" + itoa(int(b))})
		case 2:
			children = append(children, artifact.Node{
				Path: artifact.Path(name), Name: name, Kind: artifact.KindDirectory,
				Children: []artifact.Node{{
					Path: artifact.Path(name + "/leaf.txt"), Name: "leaf.txt", Kind: artifact.KindFile,
				}},
			})
		}
	}
	return directoryArtifact(children...)
}

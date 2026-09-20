package identity

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
)

func TestPropertyChildPermutation(t *testing.T) {
	base := sampleTree()
	want, err := Compute(base)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 32; i++ {
		clone := base.Clone()
		shuffleNode(&clone.Root, rng)
		got, err := Compute(clone)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("permutation %d changed fingerprint", i)
		}
	}
}

func TestPropertyFingerprintRoundTrip(t *testing.T) {
	arts := []artifact.Artifact{emptyArtifact(), fileArtifact("x"), sampleTree()}
	for _, art := range arts {
		fp, err := Compute(art)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := Parse(fp.String())
		if err != nil || parsed != fp {
			t.Fatalf("round trip failed: %v %#v", err, parsed)
		}
	}
}

func TestPropertyCanonicalIdempotence(t *testing.T) {
	paths := []string{".", "src/main.go", "café.txt", "a/b/c", ".env"}
	for _, path := range paths {
		first, err := artifact.Canonicalize(path, artifact.POSIXSeparator)
		if err != nil {
			t.Fatal(err)
		}
		second, err := artifact.Canonicalize(string(first), artifact.POSIXSeparator)
		if err != nil || first != second {
			t.Fatalf("%q: %q vs %q (%v)", path, first, second, err)
		}
	}
}

func TestNoPresentationImports(t *testing.T) {
	forbidden := []string{
		"github.com/dirloom/dirloom/internal/presentation",
		"github.com/dirloom/dirloom/internal/render",
		"github.com/dirloom/dirloom/internal/cli",
		"github.com/spf13/cobra",
	}
	err := filepath.Walk(".", func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(data)
		for _, pkg := range forbidden {
			if strings.Contains(text, `"`+pkg+`"`) {
				t.Errorf("%s imports %s", path, pkg)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func sampleTree() artifact.Artifact {
	return artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory,
		Children: []artifact.Node{
			{Path: "z.txt", Name: "z.txt", Kind: artifact.KindFile},
			{Path: "src", Name: "src", Kind: artifact.KindDirectory, Children: []artifact.Node{
				{Path: "src/b.go", Name: "b.go", Kind: artifact.KindFile},
				{Path: "src/a.go", Name: "a.go", Kind: artifact.KindFile},
			}},
			{Path: "link", Name: "link", Kind: artifact.KindSymlink, Target: "src/a.go"},
		},
	}}
}

func shuffleNode(node *artifact.Node, rng *rand.Rand) {
	rng.Shuffle(len(node.Children), func(i, j int) {
		node.Children[i], node.Children[j] = node.Children[j], node.Children[i]
	})
	for i := range node.Children {
		shuffleNode(&node.Children[i], rng)
	}
}

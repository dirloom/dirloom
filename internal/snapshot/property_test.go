package snapshot_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/snapshot"
)

func TestPropertyChildPermutation(t *testing.T) {
	art := sampleArtifact()
	capture := snapshot.CaptureV1{UseDefaultIgnores: true, UseGitignore: true, Ignore: []string{"x"}}
	baseDoc, err := snapshot.Build(art, capture)
	if err != nil {
		t.Fatal(err)
	}
	baseBytes, err := snapshot.Marshal(baseDoc)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 32; i++ {
		clone := art.Clone()
		permuteChildren(rng, &clone.Root)
		doc, err := snapshot.Build(clone, capture)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := snapshot.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(baseBytes, encoded) {
			t.Fatalf("iteration %d changed bytes", i)
		}
		if doc.Fingerprint != baseDoc.Fingerprint {
			t.Fatalf("iteration %d changed fingerprint", i)
		}
	}
}

func TestPropertyRoundTripCanonical(t *testing.T) {
	doc, err := snapshot.Build(sampleArtifact(), snapshot.CaptureV1{Ignore: []string{"a", "b"}, UseDefaultIgnores: true, UseGitignore: false})
	if err != nil {
		t.Fatal(err)
	}
	first, err := snapshot.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := snapshot.LoadBytes(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := snapshot.Marshal(loaded.Document)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonical re-encode changed bytes")
	}
}

func permuteChildren(rng *rand.Rand, node *artifact.Node) {
	rng.Shuffle(len(node.Children), func(i, j int) {
		node.Children[i], node.Children[j] = node.Children[j], node.Children[i]
	})
	for i := range node.Children {
		permuteChildren(rng, &node.Children[i])
	}
}

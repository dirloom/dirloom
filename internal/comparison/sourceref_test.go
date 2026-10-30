package comparison

import (
	"context"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/source"
)

func TestSourceRefGitIDs(t *testing.T) {
	sha1 := strings.Repeat("a", 40)
	sha256 := strings.Repeat("b", 64)
	valid := []SourceRef{
		{Kind: source.KindGit, NodeCount: 1, Tree: sha1},
		{Kind: source.KindGit, NodeCount: 1, Tree: sha1, Commit: strings.Repeat("c", 40)},
		{Kind: source.KindGit, NodeCount: 1, Tree: sha256, Commit: sha256},
		{Kind: source.KindSnapshot, NodeCount: 1},
	}
	for _, ref := range valid {
		if err := ref.validate("a"); err != nil {
			t.Fatalf("%+v: %v", ref, err)
		}
	}
	invalid := []SourceRef{
		{Kind: source.KindGit, NodeCount: 1},
		{Kind: source.KindGit, NodeCount: 1, Commit: sha1},
		{Kind: source.KindGit, NodeCount: 1, Tree: strings.ToUpper(sha1)},
		{Kind: source.KindGit, NodeCount: 1, Tree: sha1[:39]},
		{Kind: source.KindGit, NodeCount: 1, Tree: sha1, Commit: sha256},
		{Kind: source.KindGit, NodeCount: 1, Tree: sha1, Commit: "HEAD"},
		{Kind: source.KindFilesystem, NodeCount: 1, Tree: sha1},
		{Kind: source.KindSnapshot, NodeCount: 1, Commit: sha1},
	}
	for _, ref := range invalid {
		if err := ref.validate("a"); !artifact.IsInternal(err) {
			t.Fatalf("%+v accepted: %v", ref, err)
		}
	}
}

// gitMemory is a memory source that reports Git ids, as source.Git does.
type gitMemory struct {
	source.Memory
	tree, commit string
}

func (g gitMemory) Kind() source.Kind { return source.KindGit }
func (g gitMemory) Tree() string      { return g.tree }
func (g gitMemory) Commit() string    { return g.commit }

func TestCompareRecordsGitIDs(t *testing.T) {
	root := artifact.Artifact{Root: artifact.Node{Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory}}
	tree := strings.Repeat("1", 40)
	commit := strings.Repeat("2", 40)
	a := gitMemory{Memory: source.Memory{Artifact: root}, tree: tree, commit: commit}
	b := gitMemory{Memory: source.Memory{Artifact: root}, tree: tree}
	diff, err := Compare(context.Background(), a, b, Options{MoveDetection: MoveDetectionStructuralV1})
	if err != nil {
		t.Fatal(err)
	}
	if diff.SourceA != (SourceRef{Kind: source.KindGit, NodeCount: 1, Tree: tree, Commit: commit}) || diff.SourceB != (SourceRef{Kind: source.KindGit, NodeCount: 1, Tree: tree}) {
		t.Fatalf("%+v %+v", diff.SourceA, diff.SourceB)
	}
	// A Git source without ids breaks an invariant.
	if _, err := Compare(context.Background(), gitMemory{Memory: source.Memory{Artifact: root}}, b, Options{MoveDetection: MoveDetectionNone}); !artifact.IsInternal(err) {
		t.Fatalf("missing ids: %v", err)
	}
}

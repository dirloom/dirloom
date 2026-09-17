package artifact

import (
	"errors"
	"testing"
)

func TestEmptyRoot(t *testing.T) {
	art := Artifact{Root: Node{Path: RootPath, Name: ".", Kind: KindDirectory}}
	if err := art.Validate(); err != nil {
		t.Fatal(err)
	}
	if art.CountNodes() != 1 {
		t.Fatalf("count = %d", art.CountNodes())
	}
}

func TestSingleFileAndHierarchy(t *testing.T) {
	art := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{
			{Path: "README.md", Name: "README.md", Kind: KindFile},
			{Path: "src", Name: "src", Kind: KindDirectory, Children: []Node{
				{Path: "src/main.go", Name: "main.go", Kind: KindFile},
			}},
		},
	}}
	if err := art.Validate(); err != nil {
		t.Fatal(err)
	}
	if art.CountNodes() != 4 {
		t.Fatalf("count = %d", art.CountNodes())
	}
}

func TestInvalidRoot(t *testing.T) {
	art := Artifact{Root: Node{Path: "src", Name: "src", Kind: KindDirectory}}
	if err := art.Validate(); err == nil {
		t.Fatal("invalid root accepted")
	}
}

func TestAbsolutePathRejected(t *testing.T) {
	art := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{{Path: "/etc/passwd", Name: "passwd", Kind: KindFile}},
	}}
	if err := art.Validate(); err == nil {
		t.Fatal("absolute path accepted")
	}
}

func TestParentTraversalAndMismatch(t *testing.T) {
	art := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{{Path: "src/main.go", Name: "main.go", Kind: KindFile}},
	}}
	if err := art.Validate(); err == nil {
		t.Fatal("missing parent accepted")
	}
	mismatch := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{{Path: "src", Name: "other", Kind: KindDirectory}},
	}}
	if err := mismatch.Validate(); err == nil {
		t.Fatal("name/path mismatch accepted")
	}
}

func TestDuplicateNode(t *testing.T) {
	art := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{
			{Path: "a.txt", Name: "a.txt", Kind: KindFile},
			{Path: "a.txt", Name: "a.txt", Kind: KindFile},
		},
	}}
	if err := art.Validate(); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestInvalidKind(t *testing.T) {
	art := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{{Path: "pipe", Name: "pipe", Kind: Kind("socket")}},
	}}
	err := art.Validate()
	if err == nil {
		t.Fatal("invalid kind accepted")
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeUnsupportedKind {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestCanonicalCollisionViaCollisionSet(t *testing.T) {
	set := NewCollisionSet()
	path, err := Canonicalize("café.txt", POSIXSeparator)
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Add("café.txt", path); err != nil {
		t.Fatal(err)
	}
	if err := set.Add("cafe\u0301.txt", path); err == nil {
		t.Fatal("collision not detected")
	}
}

func TestCloneDoesNotShareChildren(t *testing.T) {
	art := Artifact{DisplayRootName: "proj", Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{{Path: "a.txt", Name: "a.txt", Kind: KindFile}},
	}}
	clone := art.Clone()
	clone.Root.Children[0].Name = "mutated"
	clone.DisplayRootName = "other"
	if art.Root.Children[0].Name != "a.txt" || art.DisplayRootName != "proj" {
		t.Fatal("clone shared memory")
	}
}

func TestSymlinkMayHaveTargetFileMustNot(t *testing.T) {
	ok := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{{Path: "link", Name: "link", Kind: KindSymlink, Target: "a.txt"}},
	}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := Artifact{Root: Node{
		Path: RootPath, Name: ".", Kind: KindDirectory,
		Children: []Node{{Path: "a.txt", Name: "a.txt", Kind: KindFile, Target: "nope"}},
	}}
	if err := bad.Validate(); err == nil {
		t.Fatal("file target accepted")
	}
}

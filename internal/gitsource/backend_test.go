package gitsource

import (
	"context"
	"io/fs"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
)

func TestBackendLocations(t *testing.T) {
	backend := NewBackend(&Batch{ctx: context.Background(), format: SHA1})

	entries, err := backend.ReadDir(submoduleLocation)
	if err != nil || len(entries) != 0 {
		t.Fatalf("submodule: %v %v", entries, err)
	}
	if _, ok, err := backend.ReadGitIgnore(submoduleLocation, "module"); ok || err != nil {
		t.Fatalf("submodule .gitignore: %v %v", ok, err)
	}

	foreign := fs.FileInfoToDirEntry(fileInfo{entry: TreeEntry{Mode: ModeTree, Name: "x", ID: "t"}})
	location := backend.Child("tree", foreign)
	if location != invalidLocation {
		t.Fatalf("foreign entry location %q", location)
	}
	if _, err := backend.ReadDir(location); !artifact.IsInternal(err) {
		t.Fatalf("invalid location: %v", err)
	}

	for _, tc := range []struct {
		entry    TreeEntry
		location string
		mode     fs.FileMode
	}{
		{TreeEntry{Mode: ModeTree, Name: "d", ID: "t"}, "t", fs.ModeDir},
		{TreeEntry{Mode: ModeGitlink, Name: "m", ID: "c"}, submoduleLocation, fs.ModeDir},
		{TreeEntry{Mode: ModeLink, Name: "l", ID: "b"}, "b", fs.ModeSymlink},
		{TreeEntry{Mode: ModeFile, Name: "f", ID: "f"}, "f", 0},
	} {
		entry := dirEntry{entry: tc.entry}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if got := backend.Child("tree", entry); got != tc.location {
			t.Fatalf("%s location %q", tc.entry.Name, got)
		}
		if entry.Type() != tc.mode || info.Mode().Type() != tc.mode || entry.IsDir() != (tc.mode == fs.ModeDir) || info.IsDir() != entry.IsDir() {
			t.Fatalf("%s mode %v %v", tc.entry.Name, entry.Type(), info.Mode())
		}
		if info.Sys() != nil || info.Name() != tc.entry.Name || entry.Name() != tc.entry.Name || info.Size() != 0 || !info.ModTime().IsZero() {
			t.Fatalf("%s info %+v", tc.entry.Name, info)
		}
	}
}

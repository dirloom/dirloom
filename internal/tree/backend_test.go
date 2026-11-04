package tree

import (
	"context"
	"io/fs"
	"testing"
	"time"
)

// memoryBackend serves one directory from memory. Its locations are not
// filesystem paths: the scanner learns about a junction from Junction alone.
type memoryBackend struct {
	infos     []fs.FileInfo
	links     map[string]string
	junctions map[string]bool
}

func (b memoryBackend) ReadDir(location string) ([]fs.DirEntry, error) {
	if location != "root" {
		return nil, nil
	}
	entries := make([]fs.DirEntry, 0, len(b.infos))
	for _, info := range b.infos {
		entries = append(entries, fs.FileInfoToDirEntry(info))
	}
	return entries, nil
}

func (memoryBackend) Child(_ string, entry fs.DirEntry) string { return "id:" + entry.Name() }

func (memoryBackend) ReadGitIgnore(string, string) ([]byte, bool, error) { return nil, false, nil }

func (b memoryBackend) ReadLink(location, _ string) (string, error) { return b.links[location], nil }

func (b memoryBackend) Junction(location string, _ fs.FileInfo) (bool, error) {
	return b.junctions[location], nil
}

type memoryInfo struct {
	name string
	mode fs.FileMode
}

func (i memoryInfo) Name() string       { return i.name }
func (i memoryInfo) Size() int64        { return 0 }
func (i memoryInfo) Mode() fs.FileMode  { return i.mode }
func (i memoryInfo) ModTime() time.Time { return time.Time{} }
func (i memoryInfo) IsDir() bool        { return i.mode.IsDir() }
func (i memoryInfo) Sys() any           { return nil }

func childByName(node *Node, name string) *Node {
	for _, child := range node.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

func TestScannerTakesJunctionsFromTheBackend(t *testing.T) {
	backend := memoryBackend{
		infos: []fs.FileInfo{
			memoryInfo{name: "irregular-junction", mode: fs.ModeIrregular},
			memoryInfo{name: "symlink-junction", mode: fs.ModeSymlink},
			memoryInfo{name: "link", mode: fs.ModeSymlink},
			memoryInfo{name: "irregular", mode: fs.ModeIrregular},
		},
		links: map[string]string{
			"id:irregular-junction": "C:/a",
			"id:symlink-junction":   "C:/b",
			"id:link":               "c",
		},
		junctions: map[string]bool{"id:irregular-junction": true, "id:symlink-junction": true},
	}
	scan := func(directories bool) *Node {
		t.Helper()
		model, err := NewScanner(ScanOptions{RootName: "root", Backend: backend, RootLocation: "root", Directories: directories}).Scan(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return model
	}
	model := scan(false)
	for _, want := range []Node{
		{Name: "irregular-junction", Type: NodeSymlink, Target: "C:/a", Junction: true},
		{Name: "symlink-junction", Type: NodeSymlink, Target: "C:/b", Junction: true},
		{Name: "link", Type: NodeSymlink, Target: "c"},
		{Name: "irregular", Type: NodeFile},
	} {
		got := childByName(model, want.Name)
		if got == nil || got.Type != want.Type || got.Target != want.Target || got.Junction != want.Junction || len(got.Children) != 0 {
			t.Fatalf("%s = %#v, want %#v", want.Name, got, want)
		}
	}
	if directories := scan(true); len(directories.Children) != 0 {
		t.Fatalf("directories only kept %v", childNames(directories))
	}
}

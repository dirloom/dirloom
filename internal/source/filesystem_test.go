package source

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestFromTreeEmptyAndNested(t *testing.T) {
	empty, err := FromTree(&tree.Node{Name: "proj", Type: tree.NodeDirectory}, FromTreeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if empty.DisplayRootName != "proj" || empty.Root.Name != "." || empty.CountNodes() != 1 {
		t.Fatalf("%#v", empty)
	}
	model := &tree.Node{Name: "app", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "src", Path: "src", Type: tree.NodeDirectory, Children: []*tree.Node{
			{Name: "main.go", Path: "src/main.go", Type: tree.NodeFile},
		}},
		{Name: "README.md", Path: "README.md", Type: tree.NodeFile},
	}}
	art, err := FromTree(model, FromTreeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if art.CountNodes() != 4 || art.Root.Path != "." {
		t.Fatalf("nodes=%d root=%q", art.CountNodes(), art.Root.Path)
	}
}

func TestFromTreeUnicodeCollision(t *testing.T) {
	model := &tree.Node{Name: "root", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "café.txt", Path: "café.txt", Type: tree.NodeFile},
		{Name: "cafe\u0301.txt", Path: "cafe\u0301.txt", Type: tree.NodeFile},
	}}
	if _, err := FromTree(model, FromTreeOptions{}); err == nil {
		t.Fatal("expected collision")
	}
}

func TestFilesystemEmptyRoot(t *testing.T) {
	root := t.TempDir()
	art := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "tmp"})
	if art.Root.Path != "." || art.CountNodes() != 1 || art.DisplayRootName == "" {
		t.Fatalf("%#v", art)
	}
}

func TestFilesystemNestedHiddenGitignoreDepth(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "src"))
	mustWrite(t, filepath.Join(root, "src", "main.go"), "package main")
	mustWrite(t, filepath.Join(root, ".hidden"), "x")
	mustWrite(t, filepath.Join(root, "skip.log"), "x")
	mustWrite(t, filepath.Join(root, ".gitignore"), "*.log\n")
	mustMkdir(t, filepath.Join(root, "node_modules"))
	mustWrite(t, filepath.Join(root, "node_modules", "x.js"), "x")

	git := filter.NewGitIgnore()
	policy := filter.NewPolicy("", true, nil, git, false)
	art := observeFS(t, root, tree.ScanOptions{
		RootAbs: root, RootName: "proj", UseGitIgnore: true, FilterPolicy: policy, GitIgnore: git,
	})
	names := childNames(art.Root)
	if !contains(names, "src") || contains(names, "skip.log") || contains(names, ".hidden") || contains(names, "node_modules") {
		t.Fatalf("children = %#v", names)
	}

	depth := 0
	shallow := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj", MaxDepth: &depth})
	if shallow.CountNodes() != 1 {
		t.Fatalf("depth 0 count = %d", shallow.CountNodes())
	}
}

func TestFilesystemCustomIgnoreAndDirsOnly(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "keep"))
	mustMkdir(t, filepath.Join(root, "drop"))
	mustWrite(t, filepath.Join(root, "keep", "a.go"), "x")
	mustWrite(t, filepath.Join(root, "drop", "a.go"), "x")
	mustWrite(t, filepath.Join(root, "file.txt"), "x")
	matcher, err := filter.NewIgnoreMatcher([]string{"drop"})
	if err != nil {
		t.Fatal(err)
	}
	policy := filter.NewPolicy("", false, matcher, nil, true)
	dirs := observeFS(t, root, tree.ScanOptions{
		RootAbs: root, RootName: "proj", Directories: true, FilterPolicy: policy,
	})
	if contains(childNames(dirs.Root), "file.txt") || contains(childNames(dirs.Root), "drop") {
		t.Fatalf("%#v", childNames(dirs.Root))
	}
}

func TestFilesystemSymlinkAndBroken(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "target.txt"), "x")
	if err := os.Symlink("target.txt", filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink("missing", filepath.Join(root, "broken")); err != nil {
		t.Fatal(err)
	}
	art := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
	var link, broken *artifact.Node
	for i := range art.Root.Children {
		child := &art.Root.Children[i]
		switch child.Name {
		case "link":
			link = child
		case "broken":
			broken = child
		}
	}
	if link == nil || link.Kind != artifact.KindSymlink || link.Target != "target.txt" {
		t.Fatalf("link = %#v", link)
	}
	if broken == nil || broken.Kind != artifact.KindSymlink || broken.Target != "missing" {
		t.Fatalf("broken = %#v", broken)
	}
}

func TestFilesystemAbsoluteRootDoesNotLeak(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "x")
	art := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: filepath.Base(root)})
	if art.Root.Path != "." || art.Root.Name != "." {
		t.Fatalf("root identity leaked: %#v", art.Root)
	}
}

func TestFilesystemNFDNormalizesWhenOSAllows(t *testing.T) {
	root := t.TempDir()
	nfd := "cafe\u0301.txt"
	mustWrite(t, filepath.Join(root, nfd), "x")
	art := observeFS(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
	if len(art.Root.Children) != 1 {
		t.Fatalf("children = %#v", art.Root.Children)
	}
	if art.Root.Children[0].Path != artifact.Path(norm.NFC.String(nfd)) {
		t.Fatalf("path = %q", art.Root.Children[0].Path)
	}
}

func TestLinuxUnicodeCollisionOnFilesystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("dual NFC/NFD directory entries are a Linux case")
	}
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "café.txt"), "a")
	if err := os.WriteFile(filepath.Join(root, "cafe\u0301.txt"), []byte("b"), 0o644); err != nil {
		t.Skip(err)
	}
	_, err := observeFSErr(t, root, tree.ScanOptions{RootAbs: root, RootName: "proj"})
	if err == nil {
		t.Fatal("expected collision")
	}
}

func observeFS(t *testing.T, root string, options tree.ScanOptions) *artifact.Artifact {
	t.Helper()
	art, err := observeFSErr(t, root, options)
	if err != nil {
		t.Fatal(err)
	}
	return art
}

func observeFSErr(t *testing.T, root string, options tree.ScanOptions) (*artifact.Artifact, error) {
	t.Helper()
	src := Filesystem{
		RootAbs: root,
		Scan: func(ctx context.Context) (*tree.Node, error) {
			return tree.NewScanner(options).Scan(ctx)
		},
	}
	return src.Observe(context.Background())
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func childNames(node artifact.Node) []string {
	names := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		names = append(names, child.Name)
	}
	return names
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

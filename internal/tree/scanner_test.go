package tree

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dirloom/dirloom/internal/filter"
)

func TestScannerDepthAndDirectoriesOnly(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "a", "b"))
	mustWriteFile(t, filepath.Join(root, "a", "inside.txt"), "inside")
	mustWriteFile(t, filepath.Join(root, "a", "b", "deep.txt"), "deep")
	mustWriteFile(t, filepath.Join(root, "root.txt"), "root")

	depth := 1
	model := scanForTest(t, root, &depth, false, false, nil)
	if len(model.Children) != 2 {
		t.Fatalf("depth 1 children = %d, want 2", len(model.Children))
	}
	if got := model.Children[0]; got.Name != "a" || len(got.Children) != 0 {
		t.Fatalf("directory at depth limit = %#v", got)
	}

	directories := scanForTest(t, root, nil, true, false, nil)
	if len(directories.Children) != 1 || directories.Children[0].Name != "a" {
		t.Fatalf("directories-only root = %#v", directories.Children)
	}
	if len(directories.Children[0].Children) != 1 || directories.Children[0].Children[0].Name != "b" {
		t.Fatalf("directories-only subtree = %#v", directories.Children[0].Children)
	}
}

func TestScannerDepthZero(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "file.txt"), "x")
	depth := 0
	model := scanForTest(t, root, &depth, false, false, nil)
	if len(model.Children) != 0 {
		t.Fatalf("depth zero returned children: %#v", model.Children)
	}
}

func TestScannerGitIgnoreNestedAndHiddenControlFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "nested"))
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "*.log\n")
	mustWriteFile(t, filepath.Join(root, "root.log"), "ignored")
	mustWriteFile(t, filepath.Join(root, "visible.txt"), "visible")
	mustWriteFile(t, filepath.Join(root, "nested", ".gitignore"), "*.tmp\n!keep.tmp\n")
	mustWriteFile(t, filepath.Join(root, "nested", "drop.tmp"), "ignored")
	mustWriteFile(t, filepath.Join(root, "nested", "keep.tmp"), "kept")

	git := filter.NewGitIgnore()
	policy := filter.NewPolicy("", true, nil, git, false)
	model := scanForTest(t, root, nil, false, true, policy)
	if names := childNames(model); !equalStrings(names, []string{"nested", "visible.txt"}) {
		t.Fatalf("root children = %#v", names)
	}
	if names := childNames(model.Children[0]); !equalStrings(names, []string{"keep.tmp"}) {
		t.Fatalf("nested children = %#v", names)
	}
}

func TestScannerDoesNotFollowGitIgnoreSymlink(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "external-ignore")
	mustWriteFile(t, external, "secret.txt\n")
	if err := os.Symlink(external, filepath.Join(root, ".gitignore")); err != nil {
		t.Skipf("symlinks unavailable on %s: %v", runtime.GOOS, err)
	}
	mustWriteFile(t, filepath.Join(root, "secret.txt"), "visible")

	git := filter.NewGitIgnore()
	policy := filter.NewPolicy("", false, nil, git, true)
	model := scanForTest(t, root, nil, false, true, policy)
	if names := childNames(model); !equalStrings(names, []string{".gitignore", "secret.txt"}) {
		t.Fatalf("symlinked .gitignore was followed: %#v", names)
	}
	if model.Children[0].Type != NodeSymlink {
		t.Fatalf(".gitignore node type = %q", model.Children[0].Type)
	}
}

func TestScannerPrunesExcludedDirectoryBeforeNestedGitIgnore(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "ignored", ".gitignore"))
	mustWriteFile(t, filepath.Join(root, ".gitignore"), "ignored/\n")
	mustWriteFile(t, filepath.Join(root, "ignored", "file.txt"), "x")

	git := filter.NewGitIgnore()
	policy := filter.NewPolicy("", false, nil, git, true)
	model := scanForTest(t, root, nil, false, true, policy)
	if len(model.Children) != 1 || model.Children[0].Name != ".gitignore" {
		t.Fatalf("excluded directory was not pruned: %#v", childNames(model))
	}
}

func TestScannerSymlinkIsTerminal(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	mustMkdirAll(t, target)
	mustWriteFile(t, filepath.Join(target, "inside.txt"), "x")
	link := filepath.Join(root, "link")
	if err := os.Symlink("target", link); err != nil {
		t.Skipf("symlinks unavailable on %s: %v", runtime.GOOS, err)
	}

	model := scanForTest(t, root, nil, false, false, nil)
	var found *Node
	for _, child := range model.Children {
		if child.Name == "link" {
			found = child
		}
	}
	if found == nil || found.Type != NodeSymlink || found.Target != "target" || len(found.Children) != 0 {
		t.Fatalf("symlink node = %#v", found)
	}
}

func TestScannerBrokenSymlinkIsTerminal(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "broken")
	if err := os.Symlink("missing-target", link); err != nil {
		t.Skipf("symlinks unavailable on %s: %v", runtime.GOOS, err)
	}
	model := scanForTest(t, root, nil, false, false, nil)
	if len(model.Children) != 1 || model.Children[0].Type != NodeSymlink || model.Children[0].Target != "missing-target" {
		t.Fatalf("broken symlink = %#v", model.Children)
	}
}

func TestScannerCancellationReturnsNoTree(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	scanner := NewScanner(ScanOptions{RootAbs: root, RootName: "root"})
	model, err := scanner.Scan(ctx)
	if err == nil || model != nil {
		t.Fatalf("Scan() = (%#v, %v), want nil tree and cancellation error", model, err)
	}
}

func TestRootLabel(t *testing.T) {
	root := t.TempDir()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := RootLabel(abs), filepath.Base(abs); got != want {
		t.Fatalf("RootLabel(%q) = %q, want %q", abs, got, want)
	}
	volumeRoot := string(filepath.Separator)
	if volume := filepath.VolumeName(abs); volume != "" {
		volumeRoot = volume + string(filepath.Separator)
	}
	if got := RootLabel(volumeRoot); got != filepath.Clean(volumeRoot) {
		t.Fatalf("RootLabel(volume root) = %q, want %q", got, filepath.Clean(volumeRoot))
	}
}

func scanForTest(t *testing.T, root string, depth *int, dirsOnly, useGit bool, policy *filter.Policy) *Node {
	t.Helper()
	var git *filter.GitIgnore
	if useGit {
		git = policy.Git
	}
	if policy == nil {
		policy = filter.NewPolicy("", false, nil, git, true)
	}
	scanner := NewScanner(ScanOptions{
		RootAbs: root, RootName: filepath.Base(root), MaxDepth: depth,
		Directories: dirsOnly, UseGitIgnore: useGit, FilterPolicy: policy, GitIgnore: git,
	})
	model, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func childNames(node *Node) []string {
	names := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		names = append(names, child.Name)
	}
	return names
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

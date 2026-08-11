package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

func TestInspectEndToEndFixture(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "basic-project")
	model, err := Inspect(context.Background(), InspectRequest{
		Root: root, UseDefaultIgnores: true, UseGitIgnore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.Name != "basic-project" {
		t.Fatalf("root name = %q", model.Name)
	}
	want := []string{"src", "README.md"}
	if got := names(model); !sameNames(got, want) {
		t.Fatalf("children = %#v, want %#v", got, want)
	}
}

func TestInspectExplicitRootSurvivesFilters(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "node_modules")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.js"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	model, err := Inspect(context.Background(), InspectRequest{
		Root: root, IgnorePatterns: []string{"node_modules"}, UseDefaultIgnores: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.Name != "node_modules" || len(model.Children) != 1 {
		t.Fatalf("explicit root was filtered: %#v", model)
	}
}

func TestInspectOutputDestinationIsExcluded(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "structure.txt")
	if err := os.WriteFile(output, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	model, err := Inspect(context.Background(), InspectRequest{Root: root, OutputPath: output})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(model); !sameNames(got, []string{"visible.txt"}) {
		t.Fatalf("children = %#v", got)
	}
}

func TestInspectFilterLayersRemainIndependent(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{".git", "node_modules"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{".hidden", "ignored.log", "visible.txt"} {
		if err := os.WriteFile(filepath.Join(root, file), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	defaults, err := Inspect(context.Background(), InspectRequest{
		Root: root, UseDefaultIgnores: true, UseGitIgnore: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(defaults); !sameNames(got, []string{"visible.txt"}) {
		t.Fatalf("default filters = %#v", got)
	}

	hidden, err := Inspect(context.Background(), InspectRequest{
		Root: root, UseDefaultIgnores: true, UseGitIgnore: true, IncludeHidden: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(hidden); !sameNames(got, []string{".gitignore", ".hidden", "visible.txt"}) {
		t.Fatalf("--hidden filters = %#v", got)
	}

	noDefaults, err := Inspect(context.Background(), InspectRequest{
		Root: root, UseDefaultIgnores: false, UseGitIgnore: true, IncludeHidden: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(noDefaults); !sameNames(got, []string{".git", "node_modules", ".gitignore", ".hidden", "visible.txt"}) {
		t.Fatalf("--no-default-ignore filters = %#v", got)
	}

	noGitIgnore, err := Inspect(context.Background(), InspectRequest{
		Root: root, UseDefaultIgnores: true, UseGitIgnore: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(noGitIgnore); !sameNames(got, []string{"ignored.log", "visible.txt"}) {
		t.Fatalf("--no-gitignore filters = %#v", got)
	}
}

func TestInspectErrorsAreActionable(t *testing.T) {
	if _, err := Inspect(context.Background(), InspectRequest{Root: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("missing directory should fail")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(context.Background(), InspectRequest{Root: file}); err == nil {
		t.Fatal("file root should fail")
	}
}

func TestInspectRejectsNegativeDepth(t *testing.T) {
	depth := -1
	if _, err := Inspect(context.Background(), InspectRequest{Root: t.TempDir(), MaxDepth: &depth}); err == nil {
		t.Fatal("negative application depth should fail")
	}
}

func names(node *tree.Node) []string {
	result := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		result = append(result, child.Name)
	}
	return result
}

func sameNames(left, right []string) bool {
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

package tree

import (
	"reflect"
	"testing"
)

func TestSortDirectoriesFirstAndDeterministic(t *testing.T) {
	root := &Node{Type: NodeDirectory, Children: []*Node{
		{Name: "z.txt", Path: "z.txt", Type: NodeFile},
		{Name: "users", Path: "users", Type: NodeDirectory},
		{Name: "Alpha", Path: "Alpha", Type: NodeDirectory},
		{Name: "alpha", Path: "alpha", Type: NodeDirectory},
		{Name: "Ä.txt", Path: "Ä.txt", Type: NodeFile},
		{Name: "link", Path: "link", Type: NodeSymlink},
	}}

	Sort(root)
	got := make([]string, 0, len(root.Children))
	for _, child := range root.Children {
		got = append(got, child.Name)
	}
	want := []string{"Alpha", "alpha", "users", "link", "z.txt", "Ä.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected order\n got: %#v\nwant: %#v", got, want)
	}
}

func TestSortRecursively(t *testing.T) {
	child := &Node{Type: NodeDirectory, Children: []*Node{
		{Name: "b", Type: NodeFile},
		{Name: "a", Type: NodeFile},
	}}
	root := &Node{Type: NodeDirectory, Children: []*Node{child}}
	Sort(root)
	if child.Children[0].Name != "a" {
		t.Fatalf("recursive sort did not run: %#v", child.Children)
	}
}

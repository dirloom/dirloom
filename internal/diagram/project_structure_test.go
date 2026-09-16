package diagram

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/tree"
)

func TestProjectStructureContract(t *testing.T) {
	root := sampleStructureTree()
	document, err := ProjectStructure(root, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if document.ContractVersion != 1 || document.View != ViewStructure || document.Direction != DirectionTopDown {
		t.Fatalf("document metadata = %#v", document)
	}
	labels := make([]string, 0, len(document.Nodes))
	kinds := make([]NodeKind, 0, len(document.Nodes))
	for _, node := range document.Nodes {
		labels = append(labels, node.Label)
		kinds = append(kinds, node.Kind)
	}
	if want := []string{"project/", "empty/", "src/", "link -> ../shared", "index.ts", "README.md"}; !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %#v, want %#v", labels, want)
	}
	if want := []NodeKind{NodeDirectory, NodeDirectory, NodeDirectory, NodeSymlink, NodeFile, NodeFile}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("kinds = %#v, want %#v", kinds, want)
	}
	if len(document.Edges) != len(document.Nodes)-1 {
		t.Fatalf("edges = %d, nodes = %d", len(document.Edges), len(document.Nodes))
	}
}

func TestStableIdentifiersSurviveSiblingInsertion(t *testing.T) {
	before, err := ProjectStructure(sampleStructureTree(), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	afterTree := sampleStructureTree()
	afterTree.Children = append([]*tree.Node{{Name: "aaa", Path: "aaa", Type: tree.NodeDirectory, Children: []*tree.Node{}}}, afterTree.Children...)
	after, err := ProjectStructure(afterTree, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	beforeIDs := idsByLabel(before)
	afterIDs := idsByLabel(after)
	for label, id := range beforeIDs {
		if afterIDs[label] != id {
			t.Errorf("identifier for %q changed from %q to %q", label, id, afterIDs[label])
		}
	}
}

func TestProjectStructurePreservesDistinctBranchesAndRootOnly(t *testing.T) {
	rootOnly, err := ProjectStructure(&tree.Node{Name: "solo", Type: tree.NodeDirectory, Children: []*tree.Node{}}, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(rootOnly.Nodes) != 1 || rootOnly.Nodes[0].ID != "n_root" || len(rootOnly.Edges) != 0 {
		t.Fatalf("root-only document = %#v", rootOnly)
	}

	branched := &tree.Node{Name: "root", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "app", Path: "app", Type: tree.NodeDirectory, Children: []*tree.Node{
			{Name: "main.go", Path: "app/main.go", Type: tree.NodeFile},
		}},
		{Name: "pkg", Path: "pkg", Type: tree.NodeDirectory, Children: []*tree.Node{
			{Name: "main.go", Path: "pkg/main.go", Type: tree.NodeFile},
		}},
	}}
	document, err := ProjectStructure(branched, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if document.Nodes[2].Label != "main.go" || document.Nodes[4].Label != "main.go" || document.Nodes[2].ID == document.Nodes[4].ID {
		t.Fatalf("same-name files must keep distinct identifiers: %#v", document.Nodes)
	}
	if CountNodes(branched) != len(document.Nodes) {
		t.Fatalf("count = %d, nodes = %d", CountNodes(branched), len(document.Nodes))
	}
}

func TestProjectionLimitsAndValidation(t *testing.T) {
	limit := 2
	if _, err := ProjectStructure(sampleStructureTree(), Options{View: ViewStructure, Direction: DirectionLeftRight, MaxNodes: &limit}); err == nil ||
		!strings.Contains(err.Error(), "exceeding the explicit limit") {
		t.Fatalf("limit error = %v", err)
	}
	zero := 0
	if _, err := ProjectStructure(sampleStructureTree(), Options{View: ViewStructure, Direction: DirectionTopDown, MaxNodes: &zero}); err == nil {
		t.Fatal("zero maxNodes must fail")
	}
	if _, err := ProjectStructure(nil, DefaultOptions()); err == nil {
		t.Fatal("nil root must fail")
	}
}

func TestProjectionDetectsIdentifierCollision(t *testing.T) {
	hasher := func(string) string { return "n_collision" }
	if _, err := projectStructure(sampleStructureTree(), DefaultOptions(), hasher); err == nil ||
		!strings.Contains(err.Error(), "identifier collision") {
		t.Fatalf("collision error = %v", err)
	}
}

func TestSymlinkWithoutTargetAndUnicodeRemainLiteral(t *testing.T) {
	root := &tree.Node{Name: "projet-é", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "lien", Path: "lien", Type: tree.NodeSymlink},
		{Name: "e\u0301.txt", Path: "e\u0301.txt", Type: tree.NodeFile},
	}}
	document, err := ProjectStructure(root, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if document.Nodes[1].Label != "lien [symlink]" || document.Nodes[2].Label != "e\u0301.txt" {
		t.Fatalf("labels = %#v", document.Nodes)
	}
}

func idsByLabel(document Document) map[string]string {
	result := make(map[string]string, len(document.Nodes))
	for _, node := range document.Nodes {
		result[node.Label] = node.ID
	}
	return result
}

func sampleStructureTree() *tree.Node {
	return &tree.Node{Name: "project", Type: tree.NodeDirectory, Children: []*tree.Node{
		{Name: "empty", Path: "empty", Type: tree.NodeDirectory, Children: []*tree.Node{}},
		{Name: "src", Path: "src", Type: tree.NodeDirectory, Children: []*tree.Node{
			{Name: "link", Path: "src/link", Type: tree.NodeSymlink, Target: "../shared"},
			{Name: "index.ts", Path: "src/index.ts", Type: tree.NodeFile},
		}},
		{Name: "README.md", Path: "README.md", Type: tree.NodeFile},
	}}
}

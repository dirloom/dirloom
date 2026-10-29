package comparison

import (
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
)

func mustIndex(t *testing.T, art artifact.Artifact, interner *shapeInterner) *treeIndex {
	t.Helper()
	records, err := identity.ProjectV1(art)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := newTreeIndex(records, interner)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func shapeAt(t *testing.T, idx *treeIndex, path string) shapeID {
	t.Helper()
	i, ok := idx.byPath[artifact.Path(path)]
	if !ok {
		t.Fatalf("no record %q", path)
	}
	return idx.shape[i]
}

func TestShapeIsRenameInvariantAndSharedAcrossSides(t *testing.T) {
	interner := newShapeInterner()
	a := mustIndex(t, tree("src/payment/api.go", "src/payment/model/"), interner)
	b := mustIndex(t, tree("lib/billing/api.go", "lib/billing/model/"), interner)
	if shapeAt(t, a, "src/payment") != shapeAt(t, b, "lib/billing") {
		t.Fatal("renamed identical subtrees have different shapes")
	}
	if shapeAt(t, a, "src/payment/api.go") != shapeAt(t, b, "lib/billing/api.go") {
		t.Fatal("identical files have different shapes across sides")
	}
	// The parent's shape includes its child's name.
	if shapeAt(t, a, "src") == shapeAt(t, b, "lib") {
		t.Fatal("parents with differently named children share a shape")
	}
}

func TestShapeIsSensitiveToKindTargetAndChildren(t *testing.T) {
	interner := newShapeInterner()
	idx := mustIndex(t, tree(
		"file",
		"empty/",
		"link1 -> target",
		"link2 -> target",
		"link3 -> other",
		"one/x",
		"two/y",
		"three/x/",
	), interner)
	distinct := []string{"file", "empty", "link1", "link3", "one", "two", "three"}
	seen := map[shapeID]string{}
	for _, path := range distinct {
		shape := shapeAt(t, idx, path)
		if other, ok := seen[shape]; ok {
			t.Fatalf("%q and %q share shape %d", path, other, shape)
		}
		seen[shape] = path
	}
	if shapeAt(t, idx, "link1") != shapeAt(t, idx, "link2") {
		t.Fatal("symlinks with the same target differ")
	}
}

func TestShapeIgnoresArtifactChildOrder(t *testing.T) {
	forward := directoryArtifact(directoryNode("d", fileNodeAt("d/a", "a"), fileNodeAt("d/b", "b")))
	backward := directoryArtifact(directoryNode("d", fileNodeAt("d/b", "b"), fileNodeAt("d/a", "a")))
	interner := newShapeInterner()
	if shapeAt(t, mustIndex(t, forward, interner), "d") != shapeAt(t, mustIndex(t, backward, interner), "d") {
		t.Fatal("child order changed the shape")
	}
}

func TestTreeIndexSizesAndRootOrdering(t *testing.T) {
	// "-a" sorts before the root "." (0x2D < 0x2E): the index must not assume
	// the root comes first.
	idx := mustIndex(t, tree("-a/x", "-a/y/z", "b"), newShapeInterner())
	if idx.records[0].Path != "-a" || idx.root == 0 {
		t.Fatalf("records = %+v, root = %d", idx.records, idx.root)
	}
	for path, want := range map[string]int{"-a": 4, "-a/x": 1, "-a/y": 2, "b": 1} {
		if got := idx.size[idx.byPath[artifact.Path(path)]]; got != want {
			t.Fatalf("size(%q) = %d, want %d", path, got, want)
		}
	}
	if got := len(idx.childrenOf(idx.root)); got != 2 {
		t.Fatalf("root children = %d", got)
	}
}

func TestTreeIndexDeepChainKeepsKeysSmall(t *testing.T) {
	const depth = 3000
	records := []identity.Record{{Path: artifact.RootPath, Kind: artifact.KindDirectory}}
	path := ""
	for i := 0; i < depth; i++ {
		if path == "" {
			path = "d"
		} else {
			path += "/d"
		}
		records = append(records, identity.Record{Path: artifact.Path(path), Kind: artifact.KindDirectory})
	}
	interner := newShapeInterner()
	idx, err := newTreeIndex(records, interner)
	if err != nil {
		t.Fatal(err)
	}
	if idx.size[1] != depth {
		t.Fatalf("size = %d", idx.size[1])
	}
	if len(interner.ids) != depth {
		t.Fatalf("interned shapes = %d, want %d", len(interner.ids), depth)
	}
	for key := range interner.ids {
		if len(key) > 32 {
			t.Fatalf("interning key of %d bytes grows with depth", len(key))
		}
	}
}

func TestTreeIndexRejectsBrokenRecords(t *testing.T) {
	dir := func(path string) identity.Record {
		return identity.Record{Path: artifact.Path(path), Kind: artifact.KindDirectory}
	}
	cases := map[string][]identity.Record{
		"unsorted":       {dir("."), dir("b"), dir("a")},
		"duplicate":      {dir("."), dir("a"), dir("a")},
		"no root":        {dir("a")},
		"missing parent": {dir("."), dir("a/b")},
		"unknown kind":   {dir("."), {Path: "a", Kind: artifact.Kind("fifo")}},
	}
	for name, records := range cases {
		if _, err := newTreeIndex(records, newShapeInterner()); !artifact.IsInternal(err) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

// tree builds a valid artifact from path specs: "a/b/" is a directory,
// "a/link -> target" a symlink, anything else a file. Missing parent
// directories are created.
func tree(specs ...string) artifact.Artifact {
	type entry struct {
		kind   artifact.Kind
		target string
	}
	entries := map[string]entry{}
	ensureDirs := func(path string) {
		for {
			cut := strings.LastIndex(path, "/")
			if cut < 0 {
				return
			}
			path = path[:cut]
			if _, ok := entries[path]; !ok {
				entries[path] = entry{kind: artifact.KindDirectory}
			}
		}
	}
	for _, spec := range specs {
		path, current := spec, entry{kind: artifact.KindFile}
		if link, target, ok := strings.Cut(spec, " -> "); ok {
			path, current = link, entry{kind: artifact.KindSymlink, target: target}
		} else if strings.HasSuffix(spec, "/") {
			path, current = strings.TrimSuffix(spec, "/"), entry{kind: artifact.KindDirectory}
		}
		ensureDirs(path)
		entries[path] = current
	}
	children := map[string][]string{}
	for path := range entries {
		parent := ""
		if cut := strings.LastIndex(path, "/"); cut >= 0 {
			parent = path[:cut]
		}
		children[parent] = append(children[parent], path)
	}
	var build func(parent string) []artifact.Node
	build = func(parent string) []artifact.Node {
		var nodes []artifact.Node
		for _, path := range children[parent] {
			current := entries[path]
			node := artifact.Node{
				Path:   artifact.Path(path),
				Name:   path[strings.LastIndex(path, "/")+1:],
				Kind:   current.kind,
				Target: current.target,
			}
			if current.kind == artifact.KindDirectory {
				node.Children = build(path)
			}
			nodes = append(nodes, node)
		}
		return nodes
	}
	return directoryArtifact(build("")...)
}

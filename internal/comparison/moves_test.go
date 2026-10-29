package comparison

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
)

// TestMoveDetectionReferenceOutcomes covers the reference outcomes of the
// Move Detection structural-v1 contract. Every case also checks the
// lossless, symmetry and determinism properties.
func TestMoveDetectionReferenceOutcomes(t *testing.T) {
	cases := []struct {
		name string
		a, b artifact.Artifact
		want []string
	}{
		{
			name: "large subtree moved intact",
			a:    tree("src/old/payment/api/handler.go", "src/old/payment/service.go"),
			b:    tree("src/old/", "src/features/payment/api/handler.go", "src/features/payment/service.go"),
			want: []string{
				"ADDED src/features",
				"MOVED src/old/payment -> src/features/payment subtree 4",
			},
		},
		{
			name: "lone file with a unique name relocated",
			a:    tree("docs/old/guide.md"),
			b:    tree("docs/old/", "docs/guide.md"),
			want: []string{"MOVED docs/old/guide.md -> docs/guide.md name 1"},
		},
		{
			name: "same basename in several places, one moved",
			a:    tree("a/config.yaml", "b/config.yaml", "c/"),
			b:    tree("a/", "b/config.yaml", "c/config.yaml"),
			want: []string{"REMOVED a/config.yaml", "ADDED c/config.yaml"},
		},
		{
			name: "same-named empty folder repeated",
			a:    tree("a/build/", "b/build/", "c/"),
			b:    tree("a/", "b/build/", "c/build/"),
			want: []string{"REMOVED a/build", "ADDED c/build"},
		},
		{
			name: "empty folders with distinct names relocated",
			a:    tree("old/cache/", "old/tmp/"),
			b:    tree("old/", "new/cache/", "new/tmp/"),
			want: []string{
				"ADDED new",
				"MOVED old/cache -> new/cache name 1",
				"MOVED old/tmp -> new/tmp name 1",
			},
		},
		{
			name: "directory renamed in place",
			a:    tree("src/payment/api.go", "src/payment/model.go"),
			b:    tree("src/billing/api.go", "src/billing/model.go"),
			want: []string{"MOVED src/payment -> src/billing subtree 3"},
		},
		{
			name: "repeated module shapes, one renamed in place",
			a:    tree("components/Button/index.ts", "components/Card/index.ts"),
			b:    tree("components/Card/index.ts", "components/Link/index.ts"),
			want: []string{
				"REMOVED components/Button",
				"REMOVED components/Button/index.ts",
				"ADDED components/Link",
				"ADDED components/Link/index.ts",
			},
		},
		{
			name: "renamed sibling directories with distinct shapes",
			a:    tree("a/x.go", "b/y.go"),
			b:    tree("c/x.go", "d/y.go"),
			want: []string{"MOVED a -> c subtree 2", "MOVED b -> d subtree 2"},
		},
		{
			name: "renamed sibling directories with identical shapes",
			a:    tree("a/index.ts", "b/index.ts"),
			b:    tree("c/index.ts", "d/index.ts"),
			want: []string{
				"REMOVED a", "REMOVED a/index.ts", "REMOVED b", "REMOVED b/index.ts",
				"ADDED c", "ADDED c/index.ts", "ADDED d", "ADDED d/index.ts",
			},
		},
		{
			name: "partial subtree rewrite",
			a:    tree("src/old/payment/api/handler.go", "src/old/payment/service.go"),
			b:    tree("src/old/", "src/features/payment/api/handler.go", "src/features/payment/service.go", "src/features/payment/new.go"),
			want: []string{
				"ADDED src/features",
				"ADDED src/features/payment",
				"MOVED src/old/payment/api -> src/features/payment/api subtree 2",
				"ADDED src/features/payment/new.go",
				"MOVED src/old/payment/service.go -> src/features/payment/service.go name 1",
				"REMOVED src/old/payment",
			},
		},
		{
			name: "copy is ambiguous",
			a:    tree("a/unique.txt", "b/", "c/"),
			b:    tree("a/", "b/unique.txt", "c/unique.txt"),
			want: []string{"REMOVED a/unique.txt", "ADDED b/unique.txt", "ADDED c/unique.txt"},
		},
		{
			name: "file renamed in place",
			a:    tree("a/foo.go"),
			b:    tree("a/bar.go"),
			want: []string{"ADDED a/bar.go", "REMOVED a/foo.go"},
		},
		{
			name: "directory moved and renamed",
			a:    tree("old/payment/api.go", "old/payment/model.go", "new/"),
			b:    tree("old/", "new/billing/api.go", "new/billing/model.go"),
			want: []string{
				"ADDED new/billing",
				"MOVED old/payment/api.go -> new/billing/api.go name 1",
				"MOVED old/payment/model.go -> new/billing/model.go name 1",
				"REMOVED old/payment",
			},
		},
		{
			name: "swap of two uniquely named files",
			a:    tree("x/one.txt", "y/two.txt"),
			b:    tree("x/two.txt", "y/one.txt"),
			want: []string{
				"MOVED y/two.txt -> x/two.txt name 1",
				"MOVED x/one.txt -> y/one.txt name 1",
			},
		},
		{
			name: "symlink relocated with the same target",
			a:    tree("a/link -> ../shared", "b/"),
			b:    tree("a/", "b/link -> ../shared"),
			want: []string{"MOVED a/link -> b/link name 1"},
		},
		{
			name: "symlink relocated with a different target",
			a:    tree("a/link -> ../shared", "b/"),
			b:    tree("a/", "b/link -> ../other"),
			want: []string{"REMOVED a/link", "ADDED b/link"},
		},
		{
			name: "kind change at the same path",
			a:    tree("thing"),
			b:    tree("thing/"),
			want: []string{"CHANGED thing"},
		},
		{
			name: "file moved out of a directory replaced by a file",
			a:    tree("data/unique.csv"),
			b:    tree("data", "archive/unique.csv"),
			want: []string{
				"ADDED archive",
				"MOVED data/unique.csv -> archive/unique.csv name 1",
				"CHANGED data",
			},
		},
		{
			name: "case-only directory rename",
			a:    tree("Payment/api.go", "Payment/model.go"),
			b:    tree("payment/api.go", "payment/model.go"),
			want: []string{"MOVED Payment -> payment subtree 3"},
		},
		{
			name: "case-only file rename",
			a:    tree("README.md"),
			b:    tree("readme.md"),
			want: []string{"REMOVED README.md", "ADDED readme.md"},
		},
		{
			name: "rename of a directory sorting before the root",
			a:    tree("-old/data.bin", "-old/meta.json"),
			b:    tree("-new/data.bin", "-new/meta.json"),
			want: []string{"MOVED -old -> -new subtree 3"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diff := mustCompareWith(t, tc.a, tc.b, structuralV1)
			if got := describe(diff.Changes); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("changes\n got  %q\n want %q", got, tc.want)
			}
			checkMoveProperties(t, tc.a, tc.b, diff)
		})
	}
}

func TestMoveDetectionManyGeneratedFolders(t *testing.T) {
	var specsA, specsB []string
	for i := 0; i < 1000; i++ {
		pkg := "pkg" + itoa(i)
		specsA = append(specsA, pkg+"/gen/")
		specsB = append(specsB, pkg+"/build/gen/")
	}
	a, b := tree(specsA...), tree(specsB...)
	diff := mustCompareWith(t, a, b, structuralV1)
	if diff.Summary != (Summary{Added: 2000, Removed: 1000, Total: 3000}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	checkMoveProperties(t, a, b, diff)
}

func TestMoveDetectionModeAndMetadata(t *testing.T) {
	a := tree("src/old/payment/api/handler.go", "src/old/payment/service.go")
	b := tree("src/old/", "src/features/payment/api/handler.go", "src/features/payment/service.go")

	moved := mustCompareWith(t, a, b, structuralV1)
	if moved.Metadata.MoveDetection != MoveDetectionStructuralV1 || moved.Summary != (Summary{Added: 1, Moved: 1, Total: 2}) {
		t.Fatalf("structural-v1 = %+v %+v", moved.Metadata, moved.Summary)
	}
	change := moved.Changes[1]
	if change.Before == nil || change.After == nil || change.Before.Kind != artifact.KindDirectory || !change.Before.Equal(*change.After) {
		t.Fatalf("moved states = %+v", change)
	}

	none := mustCompareWith(t, a, b, noMoves)
	if none.Metadata.MoveDetection != MoveDetectionNone || none.Summary != (Summary{Added: 5, Removed: 4, Total: 9}) {
		t.Fatalf("none = %+v %+v", none.Metadata, none.Summary)
	}

	if _, err := Compare(t.Context(), source.Memory{Artifact: a}, source.Memory{Artifact: b}, Options{}); !artifact.IsInternal(err) {
		t.Fatalf("zero options: err = %v", err)
	}
}

func TestPropertyUniqueRelocationAndRenameAreDetected(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 8))
	for i := 0; i < 200; i++ {
		names := &nameSource{rng: rng}
		base := randomMutTree(rng, names, 0)
		mutated := base.clone()
		var from, to string
		var nodes int
		var subtree bool
		if i%2 == 0 {
			from, to, nodes, subtree = relocateRandom(rng, mutated)
		} else {
			from, to, nodes = renameRandomDirectory(rng, mutated, names)
			subtree = true
		}
		if from == "" {
			continue
		}
		a, b := base.artifact(), mutated.artifact()
		diff := mustCompareWith(t, a, b, structuralV1)
		evidence := EvidenceName
		if subtree {
			evidence = EvidenceSubtree
		}
		want := []string{fmt.Sprintf("MOVED %s -> %s %s %d", from, to, evidence, nodes)}
		if got := describe(diff.Changes); !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d\n got  %q\n want %q", i, got, want)
		}
		checkMoveProperties(t, a, b, diff)
	}
}

func TestPropertyMoveDetectionOnRandomMutations(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 21))
	moves := 0
	for i := 0; i < 300; i++ {
		names := &nameSource{rng: rng, generic: true}
		base := randomMutTree(rng, names, 0)
		mutated := base.clone()
		for m := 0; m < 1+rng.IntN(4); m++ {
			mutateRandom(rng, mutated, names)
		}
		a, b := base.artifact(), mutated.artifact()
		diff := mustCompareWith(t, a, b, structuralV1)
		moves += diff.Summary.Moved
		checkMoveProperties(t, a, b, diff)
	}
	if moves == 0 {
		t.Fatal("random mutations never produced a move")
	}
}

func TestValidateRejectsBrokenMoves(t *testing.T) {
	file := NodeState{Kind: artifact.KindFile}
	dir := NodeState{Kind: artifact.KindDirectory}
	move := Change{Path: "new/a.txt", Op: OpMoved, From: "old/a.txt", Before: &file, After: &file, Evidence: EvidenceName, Nodes: 1}
	valid := StructuralDiff{
		Metadata: Metadata{
			ComparisonVersion:         Version,
			IdentityProjectionVersion: IdentityProjectionVersion,
			MoveDetection:             MoveDetectionStructuralV1,
		},
		SourceA: SourceRef{Kind: source.KindMemory, NodeCount: 3},
		SourceB: SourceRef{Kind: source.KindMemory, NodeCount: 3},
		Summary: Summary{Moved: 1, Total: 1},
		Changes: []Change{move},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid model: %v", err)
	}
	withMove := func(edit func(*Change)) func() StructuralDiff {
		return func() StructuralDiff {
			d := valid
			changed := move
			edit(&changed)
			d.Changes = []Change{changed}
			return d
		}
	}
	cases := map[string]func() StructuralDiff{
		"moved without detection": func() StructuralDiff {
			d := valid
			d.Metadata.MoveDetection = MoveDetectionNone
			return d
		},
		"no origin":            withMove(func(c *Change) { c.From = "" }),
		"origin equals path":   withMove(func(c *Change) { c.From = c.Path }),
		"origin is ancestor":   withMove(func(c *Change) { c.From = "new" }),
		"path is ancestor":     withMove(func(c *Change) { c.Path, c.From = "a", "a/b" }),
		"root origin":          withMove(func(c *Change) { c.From = artifact.RootPath }),
		"missing before":       withMove(func(c *Change) { c.Before = nil }),
		"state changes":        withMove(func(c *Change) { c.After = &dir }),
		"unknown evidence":     withMove(func(c *Change) { c.Evidence = "content" }),
		"zero nodes":           withMove(func(c *Change) { c.Nodes = 0 }),
		"name with many nodes": withMove(func(c *Change) { c.Nodes = 2 }),
		"subtree with one node": withMove(func(c *Change) {
			c.Before, c.After, c.Evidence = &dir, &dir, EvidenceSubtree
		}),
		"subtree on a file": withMove(func(c *Change) { c.Evidence, c.Nodes = EvidenceSubtree, 2 }),
		"move fields on added": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: "a.txt", Op: OpAdded, After: &file, From: "b.txt"}}
			d.Summary = Summary{Added: 1, Total: 1}
			return d
		},
		"wrong moved count": func() StructuralDiff {
			d := valid
			d.Summary = Summary{Removed: 1, Total: 1}
			return d
		},
		"origin twice": func() StructuralDiff {
			d := valid
			second := move
			second.Path = "other/a.txt"
			d.Changes = []Change{move, second}
			d.Summary = Summary{Moved: 2, Total: 2}
			return d
		},
		"origin is also removed": func() StructuralDiff {
			d := valid
			d.Changes = []Change{move, {Path: "old/a.txt", Op: OpRemoved, Before: &file}}
			d.Summary = Summary{Removed: 1, Moved: 1, Total: 2}
			return d
		},
		"removed inside a moved origin": func() StructuralDiff {
			d := valid
			subtree := Change{Path: "new/d", Op: OpMoved, From: "old/d", Before: &dir, After: &dir, Evidence: EvidenceSubtree, Nodes: 2}
			d.Changes = []Change{subtree, {Path: "old/d/x", Op: OpRemoved, Before: &file}}
			d.Summary = Summary{Removed: 1, Moved: 1, Total: 2}
			return d
		},
		"added inside a moved destination": func() StructuralDiff {
			d := valid
			subtree := Change{Path: "new/d", Op: OpMoved, From: "old/d", Before: &dir, After: &dir, Evidence: EvidenceSubtree, Nodes: 2}
			d.Changes = []Change{subtree, {Path: "new/d/x", Op: OpAdded, After: &file}}
			d.Summary = Summary{Added: 1, Moved: 1, Total: 2}
			return d
		},
		"move nested in a moved origin": func() StructuralDiff {
			d := valid
			subtree := Change{Path: "new/d", Op: OpMoved, From: "old/d", Before: &dir, After: &dir, Evidence: EvidenceSubtree, Nodes: 2}
			nested := Change{Path: "zzz/x", Op: OpMoved, From: "old/d/x", Before: &file, After: &file, Evidence: EvidenceName, Nodes: 1}
			d.Changes = []Change{subtree, nested}
			d.Summary = Summary{Moved: 2, Total: 2}
			return d
		},
	}
	for name, build := range cases {
		if err := build().Validate(); !artifact.IsInternal(err) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}

	subtree := valid
	subtree.Changes = []Change{{Path: "new/d", Op: OpMoved, From: "old/d", Before: &dir, After: &dir, Evidence: EvidenceSubtree, Nodes: 3}}
	if err := subtree.Validate(); err != nil {
		t.Fatalf("subtree move: %v", err)
	}
}

// describe renders changes as compact lines for table assertions.
func describe(changes []Change) []string {
	out := []string{}
	for _, change := range changes {
		if change.Op == OpMoved {
			out = append(out, fmt.Sprintf("MOVED %s -> %s %s %d", change.From, change.Path, change.Evidence, change.Nodes))
			continue
		}
		out = append(out, string(change.Op)+" "+string(change.Path))
	}
	return out
}

// checkMoveProperties asserts the contract invariants that need both
// sources: lossless expansion back to the path-anchored merge (I1),
// symmetry (I4) and determinism (I5).
func checkMoveProperties(t *testing.T, a, b artifact.Artifact, diff StructuralDiff) {
	t.Helper()
	none := mustCompareWith(t, a, b, noMoves)
	expanded := expandMoves(t, a, b, diff.Changes)
	if !reflect.DeepEqual(describe(expanded), describe(none.Changes)) || !reflect.DeepEqual(nonNil(expanded), nonNil(none.Changes)) {
		t.Fatalf("lossless expansion broken\n expanded %q\n none     %q", describe(expanded), describe(none.Changes))
	}
	backward := mustCompareWith(t, b, a, structuralV1)
	if inverted := invertDiff(diff); !reflect.DeepEqual(inverted, backward) {
		t.Fatalf("symmetry broken\n inverted %q\n backward %q", describe(inverted.Changes), describe(backward.Changes))
	}
	if again := mustCompareWith(t, a, b, structuralV1); !reflect.DeepEqual(again, diff) {
		t.Fatal("move detection is not deterministic")
	}
}

// expandMoves replaces each MOVED by REMOVED for every record of the A
// subtree of From and ADDED for every record of the B subtree of Path.
func expandMoves(t *testing.T, a, b artifact.Artifact, changes []Change) []Change {
	t.Helper()
	recordsA, err := identity.ProjectV1(a)
	if err != nil {
		t.Fatal(err)
	}
	recordsB, err := identity.ProjectV1(b)
	if err != nil {
		t.Fatal(err)
	}
	var out []Change
	for _, change := range changes {
		if change.Op != OpMoved {
			out = append(out, change)
			continue
		}
		for _, record := range recordsA {
			if record.Path == change.From || isAncestor(change.From, record.Path) {
				before := stateOf(record)
				out = append(out, Change{Path: record.Path, Op: OpRemoved, Before: &before})
			}
		}
		for _, record := range recordsB {
			if record.Path == change.Path || isAncestor(change.Path, record.Path) {
				after := stateOf(record)
				out = append(out, Change{Path: record.Path, Op: OpAdded, After: &after})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func nonNil(changes []Change) []Change {
	if changes == nil {
		return []Change{}
	}
	return changes
}

// mutNode is a mutable tree used to derive source B from source A by
// relocations, renames, additions, removals and copies.
type mutNode struct {
	name     string
	kind     artifact.Kind
	target   string
	children []*mutNode
}

func (n *mutNode) clone() *mutNode {
	out := &mutNode{name: n.name, kind: n.kind, target: n.target}
	for _, child := range n.children {
		out.children = append(out.children, child.clone())
	}
	return out
}

func (n *mutNode) artifact() artifact.Artifact {
	return directoryArtifact(n.nodes("")...)
}

func (n *mutNode) nodes(parent string) []artifact.Node {
	var out []artifact.Node
	for _, child := range n.children {
		path := child.name
		if parent != "" {
			path = parent + "/" + child.name
		}
		node := artifact.Node{Path: artifact.Path(path), Name: child.name, Kind: child.kind, Target: child.target}
		if child.kind == artifact.KindDirectory {
			node.Children = child.nodes(path)
		}
		out = append(out, node)
	}
	return out
}

func (n *mutNode) hasChild(name string) bool {
	for _, child := range n.children {
		if child.name == name {
			return true
		}
	}
	return false
}

func (n *mutNode) size() int {
	total := 1
	for _, child := range n.children {
		total += child.size()
	}
	return total
}

// placed is a non-root node with its parent and canonical path.
type placed struct {
	node, parent *mutNode
	path         string
}

func (n *mutNode) walk(path string, parent *mutNode, out *[]placed) {
	if parent != nil {
		*out = append(*out, placed{node: n, parent: parent, path: path})
	}
	for _, child := range n.children {
		childPath := child.name
		if path != "" {
			childPath = path + "/" + child.name
		}
		child.walk(childPath, n, out)
	}
}

func (n *mutNode) placedNodes() []placed {
	var out []placed
	n.walk("", nil, &out)
	return out
}

// nameSource yields globally unique names, plus repeated generic names when
// generic is set, so ambiguity is exercised too.
type nameSource struct {
	rng     *rand.Rand
	next    int
	generic bool
}

func (s *nameSource) name(siblings *mutNode) string {
	if s.generic && s.rng.IntN(3) == 0 {
		candidate := []string{"index.ts", "README.md", "gen", "util"}[s.rng.IntN(4)]
		if !siblings.hasChild(candidate) {
			return candidate
		}
	}
	s.next++
	return "u" + itoa(s.next)
}

func randomMutTree(rng *rand.Rand, names *nameSource, depth int) *mutNode {
	root := &mutNode{kind: artifact.KindDirectory}
	fillRandom(rng, names, root, depth)
	return root
}

func fillRandom(rng *rand.Rand, names *nameSource, dir *mutNode, depth int) {
	for i := rng.IntN(5); i > 0; i-- {
		child := &mutNode{name: names.name(dir)}
		switch rng.IntN(4) {
		case 0:
			child.kind = artifact.KindSymlink
			child.target = "target-" + itoa(rng.IntN(3))
		case 1:
			child.kind = artifact.KindFile
		default:
			child.kind = artifact.KindDirectory
			if depth < 3 {
				fillRandom(rng, names, child, depth+1)
			}
		}
		dir.children = append(dir.children, child)
	}
}

func detach(parent, child *mutNode) {
	for i, candidate := range parent.children {
		if candidate == child {
			parent.children = append(parent.children[:i], parent.children[i+1:]...)
			return
		}
	}
}

func inSubtree(root, target *mutNode) bool {
	if root == target {
		return true
	}
	for _, child := range root.children {
		if inSubtree(child, target) {
			return true
		}
	}
	return false
}

// relocateRandom moves one random node under another directory that does
// not already hold its name. It returns the old and new paths, the moved
// size, and whether the node has descendants.
func relocateRandom(rng *rand.Rand, root *mutNode) (string, string, int, bool) {
	nodes := root.placedNodes()
	if len(nodes) == 0 {
		return "", "", 0, false
	}
	moving := nodes[rng.IntN(len(nodes))]
	type destination struct {
		dir  *mutNode
		path string
	}
	dirs := []destination{{dir: root}}
	for _, candidate := range nodes {
		if candidate.node.kind == artifact.KindDirectory {
			dirs = append(dirs, destination{dir: candidate.node, path: candidate.path})
		}
	}
	var options []destination
	for _, d := range dirs {
		if d.dir != moving.parent && !inSubtree(moving.node, d.dir) && !d.dir.hasChild(moving.node.name) {
			options = append(options, d)
		}
	}
	if len(options) == 0 {
		return "", "", 0, false
	}
	target := options[rng.IntN(len(options))]
	detach(moving.parent, moving.node)
	target.dir.children = append(target.dir.children, moving.node)
	to := moving.node.name
	if target.path != "" {
		to = target.path + "/" + moving.node.name
	}
	return moving.path, to, moving.node.size(), len(moving.node.children) > 0
}

// renameRandomDirectory renames one random non-empty directory in place.
func renameRandomDirectory(rng *rand.Rand, root *mutNode, names *nameSource) (string, string, int) {
	var dirs []placed
	for _, candidate := range root.placedNodes() {
		if candidate.node.kind == artifact.KindDirectory && len(candidate.node.children) > 0 {
			dirs = append(dirs, candidate)
		}
	}
	if len(dirs) == 0 {
		return "", "", 0
	}
	renaming := dirs[rng.IntN(len(dirs))]
	renaming.node.name = names.name(renaming.parent)
	to := renaming.node.name
	if cut := strings.LastIndex(renaming.path, "/"); cut >= 0 {
		to = renaming.path[:cut] + "/" + renaming.node.name
	}
	return renaming.path, to, renaming.node.size()
}

func mutateRandom(rng *rand.Rand, root *mutNode, names *nameSource) {
	nodes := root.placedNodes()
	switch rng.IntN(5) {
	case 0:
		relocateRandom(rng, root)
	case 1:
		renameRandomDirectory(rng, root, names)
	case 2:
		dir := root
		for _, candidate := range nodes {
			if candidate.node.kind == artifact.KindDirectory && rng.IntN(2) == 0 {
				dir = candidate.node
			}
		}
		dir.children = append(dir.children, &mutNode{name: names.name(dir), kind: artifact.KindFile})
	case 3:
		if len(nodes) > 0 {
			removing := nodes[rng.IntN(len(nodes))]
			detach(removing.parent, removing.node)
		}
	default:
		if len(nodes) == 0 {
			return
		}
		copying := nodes[rng.IntN(len(nodes))]
		for _, candidate := range nodes {
			dir := candidate.node
			if dir.kind == artifact.KindDirectory && !inSubtree(copying.node, dir) && !dir.hasChild(copying.node.name) {
				dir.children = append(dir.children, copying.node.clone())
				return
			}
		}
	}
}

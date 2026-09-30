package comparison

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/source"
)

func TestCompareEmptyAndIdentical(t *testing.T) {
	empty := directoryArtifact()
	diff := mustCompare(t, empty, empty)
	if diff.Summary != (Summary{}) || len(diff.Changes) != 0 {
		t.Fatalf("empty = %+v", diff)
	}
	if diff.Metadata.ComparisonVersion != Version || diff.Metadata.IdentityProjectionVersion != 1 {
		t.Fatalf("metadata = %+v", diff.Metadata)
	}
	if diff.SourceA.Kind != source.KindMemory || diff.SourceB.Kind != source.KindMemory {
		t.Fatalf("kinds = %+v %+v", diff.SourceA, diff.SourceB)
	}
	if diff.SourceA.NodeCount != 1 || diff.SourceB.NodeCount != 1 {
		t.Fatalf("node counts = %+v %+v", diff.SourceA, diff.SourceB)
	}

	same := nestedArtifact()
	again := mustCompare(t, same, same.Clone())
	if len(again.Changes) != 0 || again.Summary.Total != 0 {
		t.Fatalf("identical = %+v", again)
	}
	if again.SourceA.NodeCount != 4 || again.SourceB.NodeCount != 4 {
		t.Fatalf("node counts = %+v %+v", again.SourceA, again.SourceB)
	}
}

func TestCompareAdded(t *testing.T) {
	diff := mustCompare(t, directoryArtifact(), directoryArtifact(fileNode("new.txt")))
	if diff.Summary != (Summary{Added: 1, Total: 1}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	change := diff.Changes[0]
	if change.Op != OpAdded || change.Path != artifact.Path("new.txt") {
		t.Fatalf("change = %+v", change)
	}
	if change.Before != nil || change.After == nil || change.After.Kind != artifact.KindFile || change.After.Target != nil {
		t.Fatalf("states = %+v", change)
	}
}

func TestCompareRemoved(t *testing.T) {
	diff := mustCompare(t, directoryArtifact(fileNode("old.txt")), directoryArtifact())
	if diff.Summary != (Summary{Removed: 1, Total: 1}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	change := diff.Changes[0]
	if change.Op != OpRemoved || change.Path != artifact.Path("old.txt") {
		t.Fatalf("change = %+v", change)
	}
	if change.After != nil || change.Before == nil || change.Before.Kind != artifact.KindFile {
		t.Fatalf("states = %+v", change)
	}
}

func TestCompareChangedKind(t *testing.T) {
	before := directoryArtifact(fileNode("node"))
	after := directoryArtifact(directoryNode("node", fileNodeAt("node/child.txt", "child.txt")))
	diff := mustCompare(t, before, after)
	if diff.Summary != (Summary{Added: 1, Changed: 1, Total: 2}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	// "node" sorts before "node/child.txt": the kind change comes first.
	change := diff.Changes[0]
	if change.Op != OpChanged || change.Path != artifact.Path("node") {
		t.Fatalf("change = %+v", change)
	}
	if change.Before.Kind != artifact.KindFile || change.After.Kind != artifact.KindDirectory {
		t.Fatalf("kinds = %s -> %s", change.Before.Kind, change.After.Kind)
	}
	if change.Before.Target != nil || change.After.Target != nil {
		t.Fatal("non-target kinds must not carry a target")
	}
}

func TestCompareChangedTarget(t *testing.T) {
	before := directoryArtifact(symlinkNode("link", "target-a"))
	after := directoryArtifact(symlinkNode("link", "target-b"))
	diff := mustCompare(t, before, after)
	if diff.Summary != (Summary{Changed: 1, Total: 1}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	change := diff.Changes[0]
	if change.Op != OpChanged || change.Before.Target == nil || *change.Before.Target != "target-a" {
		t.Fatalf("before = %+v", change.Before)
	}
	if change.After.Target == nil || *change.After.Target != "target-b" {
		t.Fatalf("after = %+v", change.After)
	}
}

func TestCompareKindFlipKeepsTargetPresence(t *testing.T) {
	before := directoryArtifact(symlinkNode("node", "elsewhere"))
	after := directoryArtifact(fileNode("node"))
	diff := mustCompare(t, before, after)
	if diff.Summary != (Summary{Changed: 1, Total: 1}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	change := diff.Changes[0]
	if change.Before.Target == nil || *change.Before.Target != "elsewhere" {
		t.Fatalf("before = %+v", change.Before)
	}
	if change.After.Target != nil {
		t.Fatalf("file must not carry a target: %+v", change.After)
	}
}

func TestCompareMixedOrdering(t *testing.T) {
	before := directoryArtifact(
		fileNode("b-removed.txt"),
		fileNode("c-changed.txt"),
		fileNode("d-same.txt"),
	)
	after := directoryArtifact(
		fileNode("a-added.txt"),
		directoryNode("c-changed.txt"),
		fileNode("d-same.txt"),
	)
	diff := mustCompare(t, before, after)
	if diff.Summary != (Summary{Added: 1, Removed: 1, Changed: 1, Total: 3}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	want := []struct {
		path string
		op   Operation
	}{
		{"a-added.txt", OpAdded},
		{"b-removed.txt", OpRemoved},
		{"c-changed.txt", OpChanged},
	}
	if len(diff.Changes) != len(want) {
		t.Fatalf("changes = %+v", diff.Changes)
	}
	for i, expected := range want {
		if diff.Changes[i].Path != artifact.Path(expected.path) || diff.Changes[i].Op != expected.op {
			t.Fatalf("change %d = %+v", i, diff.Changes[i])
		}
	}
}

func TestCompareSubtreeAddAndRemove(t *testing.T) {
	subtree := directoryNode("pkg",
		fileNodeAt("pkg/a.txt", "a.txt"),
		directoryNodeAt("pkg/sub", "sub", fileNodeAt("pkg/sub/b.txt", "b.txt")),
	)
	added := mustCompare(t, directoryArtifact(), directoryArtifact(subtree))
	if added.Summary != (Summary{Added: 4, Total: 4}) {
		t.Fatalf("added summary = %+v", added.Summary)
	}
	for _, change := range added.Changes {
		if change.Op != OpAdded {
			t.Fatalf("op = %s", change.Op)
		}
	}
	wantPaths := []string{"pkg", "pkg/a.txt", "pkg/sub", "pkg/sub/b.txt"}
	for i, path := range wantPaths {
		if added.Changes[i].Path != artifact.Path(path) {
			t.Fatalf("path %d = %q", i, added.Changes[i].Path)
		}
	}

	removed := mustCompare(t, directoryArtifact(subtree), directoryArtifact())
	if removed.Summary != (Summary{Removed: 4, Total: 4}) {
		t.Fatalf("removed summary = %+v", removed.Summary)
	}
	for i, path := range wantPaths {
		if removed.Changes[i].Path != artifact.Path(path) || removed.Changes[i].Op != OpRemoved {
			t.Fatalf("removed %d = %+v", i, removed.Changes[i])
		}
	}
}

func TestCompareRootNeverEmitted(t *testing.T) {
	// Even when every child differs, the synthetic root is never a change.
	before := directoryArtifact(fileNode("a.txt"))
	after := directoryArtifact(fileNode("b.txt"))
	diff := mustCompare(t, before, after)
	for _, change := range diff.Changes {
		if change.Path == artifact.RootPath {
			t.Fatal("root emitted as a change")
		}
	}
}

func TestCompareObservesEachSourceExactlyOnce(t *testing.T) {
	counterA := &countingSource{inner: source.Memory{Artifact: nestedArtifact()}}
	counterB := &countingSource{inner: source.Memory{Artifact: nestedArtifact()}}
	if _, err := Compare(context.Background(), counterA, counterB); err != nil {
		t.Fatal(err)
	}
	if counterA.calls != 1 || counterB.calls != 1 {
		t.Fatalf("observations = %d/%d", counterA.calls, counterB.calls)
	}
}

func TestCompareNilAndBrokenSources(t *testing.T) {
	if _, err := Compare(context.Background(), nil, source.Memory{Artifact: directoryArtifact()}); !artifact.IsInternal(err) {
		t.Fatalf("nil a = %v", err)
	}
	if _, err := Compare(context.Background(), source.Memory{Artifact: directoryArtifact()}, nil); !artifact.IsInternal(err) {
		t.Fatalf("nil b = %v", err)
	}
	if _, err := Compare(context.Background(), source.Memory{Artifact: directoryArtifact()}, stubSource{}); !artifact.IsInternal(err) {
		t.Fatalf("nil artifact = %v", err)
	}

	broken := errors.New("unreadable")
	_, err := Compare(context.Background(), stubSource{err: artifact.SourceReadFailure(broken)}, source.Memory{Artifact: directoryArtifact()})
	side, ok := SideOf(err)
	if !ok || side != SideA {
		t.Fatalf("side = %q %v", side, ok)
	}
	_, err = Compare(context.Background(), source.Memory{Artifact: directoryArtifact()}, stubSource{err: artifact.SourceReadFailure(broken)})
	side, ok = SideOf(err)
	if !ok || side != SideB {
		t.Fatalf("side = %q %v", side, ok)
	}
	// A failing side A must not trigger an observation of side B.
	counter := &countingSource{inner: source.Memory{Artifact: directoryArtifact()}}
	_, err = Compare(context.Background(), stubSource{err: broken}, counter)
	if err == nil || counter.calls != 0 {
		t.Fatalf("b observed %d times after a failed", counter.calls)
	}
}

func TestCompareContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Compare(ctx, source.Memory{Artifact: directoryArtifact()}, source.Memory{Artifact: directoryArtifact()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled = %v", err)
	}
}

func TestCompareCaseSensitivity(t *testing.T) {
	before := directoryArtifact(fileNode("Auth.txt"))
	after := directoryArtifact(fileNode("auth.txt"))
	diff := mustCompare(t, before, after)
	if diff.Summary != (Summary{Added: 1, Removed: 1, Total: 2}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	if diff.Changes[0].Path != "Auth.txt" || diff.Changes[0].Op != OpRemoved {
		t.Fatalf("first = %+v", diff.Changes[0])
	}
	if diff.Changes[1].Path != "auth.txt" || diff.Changes[1].Op != OpAdded {
		t.Fatalf("second = %+v", diff.Changes[1])
	}
}

func TestCompareDeepPathsAndFanOut(t *testing.T) {
	deep := artifact.Node{Path: "d0", Name: "d0", Kind: artifact.KindDirectory}
	cursor := &deep
	const depth = 40
	for i := 1; i <= depth; i++ {
		name := "d" + itoa(i)
		child := artifact.Node{Path: artifact.Path(string(cursor.Path) + "/" + name), Name: name, Kind: artifact.KindDirectory}
		cursor.Children = []artifact.Node{child}
		cursor = &cursor.Children[0]
	}
	cursor.Children = []artifact.Node{{Path: artifact.Path(string(cursor.Path) + "/leaf.txt"), Name: "leaf.txt", Kind: artifact.KindFile}}

	wide := make([]artifact.Node, 500)
	for i := range wide {
		name := "f" + itoa(i) + ".txt"
		wide[i] = fileNode(name)
	}
	before := directoryArtifact(append(wide, deep)...)
	after := before.Clone()
	after.Root.Children[0] = fileNode("f0-changed.txt")

	diff := mustCompare(t, before, after)
	if diff.Summary != (Summary{Added: 1, Removed: 1, Total: 2}) {
		t.Fatalf("summary = %+v", diff.Summary)
	}
	// root + wide files + (d0..d40) directories + leaf.txt
	if diff.SourceA.NodeCount != 1+len(wide)+(depth+1)+1 {
		t.Fatalf("node count = %d", diff.SourceA.NodeCount)
	}
}

func TestValidateRejectsBrokenModels(t *testing.T) {
	file := NodeState{Kind: artifact.KindFile}
	dir := NodeState{Kind: artifact.KindDirectory}
	target := "elsewhere"
	link := NodeState{Kind: artifact.KindSymlink, Target: &target}
	valid := StructuralDiff{
		Metadata: Metadata{
			ComparisonVersion:         Version,
			IdentityProjectionVersion: IdentityProjectionVersion,
		},
		SourceA: SourceRef{Kind: source.KindMemory, NodeCount: 2},
		SourceB: SourceRef{Kind: source.KindMemory, NodeCount: 1},
		Summary: Summary{Removed: 1, Total: 1},
		Changes: []Change{{Path: "a.txt", Op: OpRemoved, Before: &file}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid model: %v", err)
	}

	cases := map[string]func() StructuralDiff{
		"comparison version": func() StructuralDiff {
			d := valid
			d.Metadata.ComparisonVersion = 2
			return d
		},
		"projection version": func() StructuralDiff {
			d := valid
			d.Metadata.IdentityProjectionVersion = 7
			return d
		},
		"empty source kind": func() StructuralDiff {
			d := valid
			d.SourceA.Kind = ""
			return d
		},
		"zero node count": func() StructuralDiff {
			d := valid
			d.SourceB.NodeCount = 0
			return d
		},
		"root change": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: artifact.RootPath, Op: OpRemoved, Before: &dir}}
			return d
		},
		"unsorted changes": func() StructuralDiff {
			d := valid
			d.Changes = []Change{
				{Path: "b.txt", Op: OpRemoved, Before: &file},
				{Path: "a.txt", Op: OpRemoved, Before: &file},
			}
			d.Summary = Summary{Removed: 2, Total: 2}
			return d
		},
		"duplicate paths": func() StructuralDiff {
			d := valid
			d.Changes = []Change{
				{Path: "a.txt", Op: OpRemoved, Before: &file},
				{Path: "a.txt", Op: OpAdded, After: &file},
			}
			d.Summary = Summary{Added: 1, Removed: 1, Total: 2}
			return d
		},
		"added with before": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: "a.txt", Op: OpAdded, Before: &file, After: &file}}
			d.Summary = Summary{Added: 1, Total: 1}
			return d
		},
		"removed with after": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: "a.txt", Op: OpRemoved, After: &file}}
			return d
		},
		"changed missing side": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: "a.txt", Op: OpChanged, Before: &file}}
			d.Summary = Summary{Changed: 1, Total: 1}
			return d
		},
		"changed identical states": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: "a.txt", Op: OpChanged, Before: &file, After: &file}}
			d.Summary = Summary{Changed: 1, Total: 1}
			return d
		},
		"target on file": func() StructuralDiff {
			d := valid
			bad := NodeState{Kind: artifact.KindFile, Target: &target}
			d.Changes = []Change{{Path: "a.txt", Op: OpAdded, After: &bad}}
			d.Summary = Summary{Added: 1, Total: 1}
			return d
		},
		"symlink without target": func() StructuralDiff {
			d := valid
			bare := NodeState{Kind: artifact.KindSymlink}
			d.Changes = []Change{{Path: "a.txt", Op: OpAdded, After: &bare}}
			d.Summary = Summary{Added: 1, Total: 1}
			return d
		},
		"unknown operation": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: "a.txt", Op: Operation("MOVED"), Before: &file, After: &file}}
			d.Summary = Summary{Total: 1}
			return d
		},
		"wrong summary": func() StructuralDiff {
			d := valid
			d.Summary = Summary{Removed: 2, Total: 2}
			return d
		},
		"wrong total": func() StructuralDiff {
			d := valid
			d.Summary = Summary{Removed: 1, Total: 5}
			return d
		},
		"empty path": func() StructuralDiff {
			d := valid
			d.Changes = []Change{{Path: "", Op: OpRemoved, Before: &file}}
			return d
		},
	}
	for name, build := range cases {
		if err := build().Validate(); !artifact.IsInternal(err) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}

	// Valid changed models pass: kind-only flip and symlink-to-file.
	d := valid
	d.Changes = []Change{{Path: "a.txt", Op: OpChanged, Before: &file, After: &dir}}
	d.Summary = Summary{Changed: 1, Total: 1}
	if err := d.Validate(); err != nil {
		t.Fatalf("kind flip: %v", err)
	}
	d.Changes = []Change{{Path: "link", Op: OpChanged, Before: &link, After: &file}}
	if err := d.Validate(); err != nil {
		t.Fatalf("symlink to file: %v", err)
	}
}

func TestPropertyCompareSymmetry(t *testing.T) {
	rng := newSeededRand()
	for i := 0; i < 32; i++ {
		a := randomArtifact(rng)
		b := randomArtifact(rng)
		forward := mustCompare(t, a, b)
		backward := mustCompare(t, b, a)
		inverted := invertDiff(forward)
		if !reflect.DeepEqual(inverted, backward) {
			t.Fatalf("iteration %d\nforward  = %+v\nbackward = %+v", i, forward, backward)
		}
	}
}

func TestPropertyCompareDeterminism(t *testing.T) {
	rng := newSeededRand()
	a := randomArtifact(rng)
	b := randomArtifact(rng)
	first := mustCompare(t, a, b)
	for i := 0; i < 8; i++ {
		again := mustCompare(t, a, b)
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("run %d differs", i)
		}
	}
}

// invertDiff swaps the sides of a diff: ADDED becomes REMOVED, REMOVED
// becomes ADDED, and CHANGED swaps before/after.
func invertDiff(d StructuralDiff) StructuralDiff {
	inverted := StructuralDiff{
		Metadata: Metadata{
			ComparisonVersion:         d.Metadata.ComparisonVersion,
			IdentityProjectionVersion: d.Metadata.IdentityProjectionVersion,
		},
		SourceA: d.SourceB,
		SourceB: d.SourceA,
		Summary: Summary{
			Added:   d.Summary.Removed,
			Removed: d.Summary.Added,
			Changed: d.Summary.Changed,
			Total:   d.Summary.Total,
		},
	}
	// Keep Changes nil when empty so DeepEqual matches the engine output.
	for _, change := range d.Changes {
		switch change.Op {
		case OpAdded:
			inverted.Changes = append(inverted.Changes, Change{Path: change.Path, Op: OpRemoved, Before: change.After})
		case OpRemoved:
			inverted.Changes = append(inverted.Changes, Change{Path: change.Path, Op: OpAdded, After: change.Before})
		case OpChanged:
			inverted.Changes = append(inverted.Changes, Change{Path: change.Path, Op: OpChanged, Before: change.After, After: change.Before})
		}
	}
	return inverted
}

func mustCompare(t testing.TB, a, b artifact.Artifact) StructuralDiff {
	t.Helper()
	diff, err := Compare(context.Background(), source.Memory{Artifact: a}, source.Memory{Artifact: b})
	if err != nil {
		t.Fatal(err)
	}
	return diff
}

type countingSource struct {
	inner source.Source
	calls int
}

func (c *countingSource) Kind() source.Kind {
	if c.inner == nil {
		return source.KindMemory
	}
	return c.inner.Kind()
}

func (c *countingSource) Observe(ctx context.Context) (*artifact.Artifact, error) {
	c.calls++
	return c.inner.Observe(ctx)
}

type stubSource struct {
	err error
}

func (stubSource) Kind() source.Kind { return source.KindMemory }

func (s stubSource) Observe(context.Context) (*artifact.Artifact, error) {
	return nil, s.err
}

func directoryArtifact(children ...artifact.Node) artifact.Artifact {
	return artifact.Artifact{Root: artifact.Node{
		Path: artifact.RootPath, Name: ".", Kind: artifact.KindDirectory, Children: children,
	}}
}

func fileNode(name string) artifact.Node {
	return artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindFile}
}

func fileNodeAt(path, name string) artifact.Node {
	return artifact.Node{Path: artifact.Path(path), Name: name, Kind: artifact.KindFile}
}

func directoryNode(name string, children ...artifact.Node) artifact.Node {
	return artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindDirectory, Children: children}
}

func directoryNodeAt(path, name string, children ...artifact.Node) artifact.Node {
	return artifact.Node{Path: artifact.Path(path), Name: name, Kind: artifact.KindDirectory, Children: children}
}

func symlinkNode(name, target string) artifact.Node {
	return artifact.Node{Path: artifact.Path(name), Name: name, Kind: artifact.KindSymlink, Target: target}
}

func nestedArtifact() artifact.Artifact {
	return directoryArtifact(
		fileNode("README.md"),
		directoryNode("src", fileNodeAt("src/main.go", "main.go")),
	)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [16]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

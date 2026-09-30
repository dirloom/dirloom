package comparison

import (
	"context"
	"reflect"
	"testing"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/source"
)

// FuzzCompareGenerated compares two valid generated artifacts and checks the
// engine invariants: no panic, a validated model, sorted unique changes, an
// exact summary, and byte-stable determinism.
func FuzzCompareGenerated(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{})
	f.Add([]byte{255, 255, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		half := len(data) / 2
		a := artifactFromBytes(data[:half])
		b := artifactFromBytes(data[half:])
		diff, err := Compare(context.Background(), source.Memory{Artifact: a}, source.Memory{Artifact: b})
		if err != nil {
			t.Fatalf("valid generated artifacts: %v", err)
		}
		if err := diff.Validate(); err != nil {
			t.Fatalf("engine produced an invalid model: %v", err)
		}
		for i, change := range diff.Changes {
			if change.Path == artifact.RootPath {
				t.Fatal("root emitted as a change")
			}
			if i > 0 && diff.Changes[i-1].Path >= change.Path {
				t.Fatalf("changes not strictly sorted at %q", change.Path)
			}
		}
		if diff.Summary.Total != len(diff.Changes) {
			t.Fatalf("summary = %+v for %d changes", diff.Summary, len(diff.Changes))
		}
		again, err := Compare(context.Background(), source.Memory{Artifact: a}, source.Memory{Artifact: b})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(diff, again) {
			t.Fatal("comparison is not deterministic")
		}
		inverted := invertDiff(diff)
		backward, err := Compare(context.Background(), source.Memory{Artifact: b}, source.Memory{Artifact: a})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(inverted, backward) {
			t.Fatal("symmetry broken: invert(diff(A,B)) != diff(B,A)")
		}
	})
}

// FuzzValidateModel feeds semi-structured models into the invariant validator:
// it must never panic and must answer deterministically.
func FuzzValidateModel(f *testing.F) {
	f.Add([]byte{1, 2, 3})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		model := modelFromBytes(data)
		first := model.Validate()
		second := model.Validate()
		if (first == nil) != (second == nil) {
			t.Fatalf("validator is not deterministic: %v vs %v", first, second)
		}
	})
}

// modelFromBytes builds a deliberately unconstrained model so the validator
// sees unsorted, duplicated, and malformed changes.
func modelFromBytes(data []byte) StructuralDiff {
	if len(data) > 64 {
		data = data[:64]
	}
	file := NodeState{Kind: artifact.KindFile}
	dir := NodeState{Kind: artifact.KindDirectory}
	target := "t"
	link := NodeState{Kind: artifact.KindSymlink, Target: &target}
	states := []NodeState{file, dir, link}
	ops := []Operation{OpAdded, OpRemoved, OpChanged, Operation("MOVED"), Operation("")}
	var changes []Change
	for i, b := range data {
		path := artifact.Path("p" + itoa(int(b)%4))
		if b%7 == 0 {
			path = artifact.RootPath
		}
		change := Change{Path: path, Op: ops[int(b)%len(ops)]}
		if b%2 == 0 {
			before := states[int(b)%len(states)]
			change.Before = &before
		}
		if b%3 == 0 {
			after := states[int(b)%len(states)]
			change.After = &after
		}
		_ = i
		changes = append(changes, change)
	}
	return StructuralDiff{
		Metadata: Metadata{
			ComparisonVersion:         Version,
			IdentityProjectionVersion: IdentityProjectionVersion,
		},
		SourceA: SourceRef{Kind: source.KindMemory, NodeCount: 1},
		SourceB: SourceRef{Kind: source.KindMemory, NodeCount: 1},
		Summary: Summary{Added: 1, Removed: 1, Changed: 1, Total: len(changes)},
		Changes: changes,
	}
}

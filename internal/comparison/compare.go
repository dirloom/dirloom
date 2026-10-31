package comparison

import (
	"context"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
)

// Compare observes each source exactly once, projects both artifacts through
// Identity Projection v1, merges the sorted record lists in one pass, then
// applies the move detection strategy named by opts. The result is validated
// before it is returned.
func Compare(ctx context.Context, a, b source.Source, opts Options) (StructuralDiff, error) {
	if err := ctx.Err(); err != nil {
		return StructuralDiff{}, err
	}
	if !opts.MoveDetection.Valid() {
		return StructuralDiff{}, artifact.InternalError("unsupported move detection %q", opts.MoveDetection)
	}
	if a == nil || b == nil {
		return StructuralDiff{}, artifact.InternalError("comparison source is nil")
	}
	artA, err := observe(ctx, a)
	if err != nil {
		return StructuralDiff{}, &SideError{Side: SideA, Err: err}
	}
	artB, err := observe(ctx, b)
	if err != nil {
		return StructuralDiff{}, &SideError{Side: SideB, Err: err}
	}
	recordsA, err := identity.ProjectV1(*artA)
	if err != nil {
		return StructuralDiff{}, &SideError{Side: SideA, Err: err}
	}
	recordsB, err := identity.ProjectV1(*artB)
	if err != nil {
		return StructuralDiff{}, &SideError{Side: SideB, Err: err}
	}
	return CompareRecords(recordsA, recordsB, sourceRef(a, len(recordsA)), sourceRef(b, len(recordsB)), opts)
}

// CompareRecords compares the Identity Projection v1 records of two
// already-observed sources: the two-way merge, then the move detection
// strategy named by opts. refA and refB describe the sources; their node
// counts are taken from the records. The result is validated before it is
// returned.
func CompareRecords(recordsA, recordsB []identity.Record, refA, refB SourceRef, opts Options) (StructuralDiff, error) {
	if !opts.MoveDetection.Valid() {
		return StructuralDiff{}, artifact.InternalError("unsupported move detection %q", opts.MoveDetection)
	}
	changes := mergeRecords(recordsA, recordsB)
	if opts.MoveDetection == MoveDetectionStructuralV1 {
		var err error
		changes, err = detectMovesStructuralV1(recordsA, recordsB, changes)
		if err != nil {
			return StructuralDiff{}, err
		}
	}
	refA.NodeCount, refB.NodeCount = len(recordsA), len(recordsB)
	diff := StructuralDiff{
		Metadata: Metadata{
			ComparisonVersion:         Version,
			IdentityProjectionVersion: identity.ProjectionVersion,
			MoveDetection:             opts.MoveDetection,
		},
		SourceA: refA,
		SourceB: refB,
		Summary: summarize(changes),
		Changes: changes,
	}
	if err := diff.Validate(); err != nil {
		return StructuralDiff{}, err
	}
	return diff, nil
}

// sourceRef describes an observed source; a Git source adds the ids of the
// tree it observed.
func sourceRef(src source.Source, nodeCount int) SourceRef {
	ref := SourceRef{Kind: src.Kind(), NodeCount: nodeCount}
	if git, ok := src.(source.GitTree); ok {
		ref.Tree, ref.Commit = git.Tree(), git.Commit()
	}
	return ref
}

func observe(ctx context.Context, src source.Source) (*artifact.Artifact, error) {
	art, err := src.Observe(ctx)
	if err != nil {
		return nil, err
	}
	if art == nil {
		return nil, artifact.InternalError("comparison source returned a nil artifact")
	}
	return art, nil
}

// mergeRecords is the single O(N+M) two-way merge over path-sorted identity
// records. The synthetic root is never emitted as a change.
func mergeRecords(a, b []identity.Record) []Change {
	var changes []Change
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i].Path < b[j].Path:
			changes = appendRemoved(changes, a[i])
			i++
		case a[i].Path > b[j].Path:
			changes = appendAdded(changes, b[j])
			j++
		default:
			if !recordEqual(a[i], b[j]) {
				changes = appendChanged(changes, a[i], b[j])
			}
			i++
			j++
		}
	}
	for ; i < len(a); i++ {
		changes = appendRemoved(changes, a[i])
	}
	for ; j < len(b); j++ {
		changes = appendAdded(changes, b[j])
	}
	return changes
}

func recordEqual(a, b identity.Record) bool {
	return a.Kind == b.Kind && a.Target == b.Target
}

func appendAdded(changes []Change, record identity.Record) []Change {
	if record.Path == artifact.RootPath {
		return changes
	}
	after := stateOf(record)
	return append(changes, Change{Path: record.Path, Op: OpAdded, After: &after})
}

func appendRemoved(changes []Change, record identity.Record) []Change {
	if record.Path == artifact.RootPath {
		return changes
	}
	before := stateOf(record)
	return append(changes, Change{Path: record.Path, Op: OpRemoved, Before: &before})
}

func appendChanged(changes []Change, a, b identity.Record) []Change {
	if a.Path == artifact.RootPath {
		return changes
	}
	before := stateOf(a)
	after := stateOf(b)
	return append(changes, Change{Path: a.Path, Op: OpChanged, Before: &before, After: &after})
}

// stateOf projects one identity record into a node state with explicit target
// presence for target-carrying kinds.
func stateOf(record identity.Record) NodeState {
	state := NodeState{Kind: record.Kind}
	if record.Kind.HasTarget() {
		target := record.Target
		state.Target = &target
	}
	return state
}

func summarize(changes []Change) Summary {
	var summary Summary
	for _, change := range changes {
		switch change.Op {
		case OpAdded:
			summary.Added++
		case OpRemoved:
			summary.Removed++
		case OpChanged:
			summary.Changed++
		case OpMoved:
			summary.Moved++
		}
	}
	summary.Total = summary.Added + summary.Removed + summary.Changed + summary.Moved
	return summary
}

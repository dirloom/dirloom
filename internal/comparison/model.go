// Package comparison implements the Structural Diff engine: it compares two
// structural sources through Identity Projection v1 records and produces a
// validated StructuralDiff model. It contains no presentation, no CLI, and no
// filesystem traversal of its own.
package comparison

import (
	"sort"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
)

const (
	// Version is the path-anchored comparison version frozen by Diff Result
	// Schema v1. Move detection is versioned separately by MoveDetection and
	// does not change it.
	Version = 1
	// IdentityProjectionVersion is the Identity Projection contract the
	// comparison is anchored to. It must equal identity.ProjectionVersion.
	IdentityProjectionVersion = identity.ProjectionVersion
)

// Operation is the closed Structural Diff vocabulary. There is no RENAMED
// operation: a rename in place is a MOVED whose parent is unchanged.
type Operation string

const (
	// OpAdded marks a path present in B and absent from A.
	OpAdded Operation = "ADDED"
	// OpRemoved marks a path present in A and absent from B.
	OpRemoved Operation = "REMOVED"
	// OpChanged marks a path present in both whose Identity Projection v1
	// record differs (kind, or target of a target-carrying kind).
	OpChanged Operation = "CHANGED"
	// OpMoved marks a removed node of A and an added node of B that
	// correspond under the move detection strategy. Path is the destination
	// in B, From the origin in A.
	OpMoved Operation = "MOVED"
)

// Valid reports whether op is one of the operations.
func (op Operation) Valid() bool {
	switch op {
	case OpAdded, OpRemoved, OpChanged, OpMoved:
		return true
	default:
		return false
	}
}

// MoveDetection names the strategy that relabels REMOVED and ADDED records
// as MOVED after the path-anchored merge.
type MoveDetection string

const (
	// MoveDetectionNone keeps the path-anchored merge result unchanged.
	MoveDetectionNone MoveDetection = "none"
	// MoveDetectionStructuralV1 is Move Detection structural-v1.
	MoveDetectionStructuralV1 MoveDetection = "structural-v1"
)

// Valid reports whether m is a known strategy.
func (m MoveDetection) Valid() bool {
	return m == MoveDetectionNone || m == MoveDetectionStructuralV1
}

// Evidence is the closed vocabulary justifying a MOVED change. It is not a
// confidence score.
type Evidence string

const (
	// EvidenceSubtree means the moved node has descendants whose names, kinds
	// and targets are identical on both sides.
	EvidenceSubtree Evidence = "subtree"
	// EvidenceName means the moved node has no descendant: only its name and
	// state identify it.
	EvidenceName Evidence = "name"
)

// Valid reports whether e is a known evidence class.
func (e Evidence) Valid() bool {
	return e == EvidenceSubtree || e == EvidenceName
}

// Options selects comparison behavior beyond the path-anchored merge.
type Options struct {
	MoveDetection MoveDetection
}

// NodeState is one side of a change. Target is present exactly when Kind
// carries one; presence is explicit so an empty target stays distinguishable
// from an absent one.
type NodeState struct {
	Kind   artifact.Kind
	Target *string
}

// Equal reports whether two states are identical, including target presence.
func (s NodeState) Equal(other NodeState) bool {
	if s.Kind != other.Kind {
		return false
	}
	if (s.Target == nil) != (other.Target == nil) {
		return false
	}
	if s.Target == nil {
		return true
	}
	return *s.Target == *other.Target
}

// SourceRef describes one compared source without environment data: no dates,
// hosts, users, absolute paths, or source expressions.
type SourceRef struct {
	Kind      source.Kind
	NodeCount int
}

// Metadata describes the comparison contract versions and the move detection
// strategy. It never names the compared sources: those are sibling fields on
// StructuralDiff.
type Metadata struct {
	ComparisonVersion         int
	IdentityProjectionVersion int
	MoveDetection             MoveDetection
}

// Change is one canonical path that differs between A (before) and B (after).
// ADDED carries only After, REMOVED only Before, CHANGED and MOVED carry
// both. From, Evidence and Nodes are set on MOVED only: Nodes counts the
// records the move covers, root included.
type Change struct {
	Path     artifact.Path
	Op       Operation
	From     artifact.Path
	Before   *NodeState
	After    *NodeState
	Evidence Evidence
	Nodes    int
}

// Summary counts changes by operation. Total is always
// Added+Removed+Changed+Moved.
type Summary struct {
	Added   int
	Removed int
	Changed int
	Moved   int
	Total   int
}

// StructuralDiff is the validated comparison outcome.
type StructuralDiff struct {
	Metadata Metadata
	SourceA  SourceRef
	SourceB  SourceRef
	Summary  Summary
	Changes  []Change
}

// Validate checks the Structural Diff model invariants. Every violation is a
// broken Dirloom invariant and therefore an internal error.
func (d StructuralDiff) Validate() error {
	if d.Metadata.ComparisonVersion != Version {
		return artifact.InternalError("comparison version %d is not %d", d.Metadata.ComparisonVersion, Version)
	}
	if d.Metadata.IdentityProjectionVersion != identity.ProjectionVersion {
		return artifact.InternalError("identity projection version %d is not %d", d.Metadata.IdentityProjectionVersion, identity.ProjectionVersion)
	}
	if !d.Metadata.MoveDetection.Valid() {
		return artifact.InternalError("unsupported move detection %q", d.Metadata.MoveDetection)
	}
	if err := d.SourceA.validate("a"); err != nil {
		return err
	}
	if err := d.SourceB.validate("b"); err != nil {
		return err
	}
	added, removed, changed, moved := 0, 0, 0, 0
	for i, change := range d.Changes {
		if err := change.validate(); err != nil {
			return err
		}
		if change.Path == artifact.RootPath || change.From == artifact.RootPath {
			return artifact.InternalError("change %d targets the root path %q", i, artifact.RootPath)
		}
		if i > 0 && d.Changes[i-1].Path >= change.Path {
			return artifact.InternalError("changes are not strictly sorted at %q", change.Path)
		}
		switch change.Op {
		case OpAdded:
			added++
		case OpRemoved:
			removed++
		case OpChanged:
			changed++
		case OpMoved:
			moved++
		}
	}
	if moved > 0 && d.Metadata.MoveDetection == MoveDetectionNone {
		return artifact.InternalError("%d moved changes without move detection", moved)
	}
	if d.Summary.Added != added || d.Summary.Removed != removed || d.Summary.Changed != changed || d.Summary.Moved != moved {
		return artifact.InternalError("summary %d/%d/%d/%d does not match changes %d/%d/%d/%d",
			d.Summary.Added, d.Summary.Removed, d.Summary.Changed, d.Summary.Moved, added, removed, changed, moved)
	}
	if d.Summary.Total != added+removed+changed+moved || d.Summary.Total != len(d.Changes) {
		return artifact.InternalError("summary total %d does not match %d changes", d.Summary.Total, len(d.Changes))
	}
	if moved > 0 {
		return d.validateMoves()
	}
	return nil
}

// validateMoves checks the invariants that span several changes: every path
// and origin appears once, and no change path or origin lies strictly below
// a move's origin or destination, since a move absorbs both subtrees.
//
// Paths below p are exactly those with prefix p+"/", and they form one
// contiguous block in byte order, so a binary search over the sorted changes
// and the sorted origins finds them. The cost is linear in total path bytes
// plus O(K log C) path comparisons for K moves among C changes, whatever the
// depth of the paths.
func (d StructuralDiff) validateMoves() error {
	paths := make(map[artifact.Path]struct{}, len(d.Changes))
	for _, change := range d.Changes {
		paths[change.Path] = struct{}{}
	}
	var origins []artifact.Path
	for _, change := range d.Changes {
		if change.Op != OpMoved {
			continue
		}
		if _, ok := paths[change.From]; ok {
			return artifact.InternalError("move origin %q is also a change path", change.From)
		}
		origins = append(origins, change.From)
	}
	sort.Slice(origins, func(i, j int) bool { return origins[i] < origins[j] })
	for i := 1; i < len(origins); i++ {
		if origins[i-1] == origins[i] {
			return artifact.InternalError("move origin %q appears twice", origins[i])
		}
	}
	for _, change := range d.Changes {
		if change.Op != OpMoved {
			continue
		}
		for _, root := range [2]artifact.Path{change.From, change.Path} {
			prefix := string(root) + "/"
			i := sort.Search(len(d.Changes), func(i int) bool { return string(d.Changes[i].Path) >= prefix })
			j := sort.Search(len(origins), func(j int) bool { return string(origins[j]) >= prefix })
			if (i < len(d.Changes) && strings.HasPrefix(string(d.Changes[i].Path), prefix)) ||
				(j < len(origins) && strings.HasPrefix(string(origins[j]), prefix)) {
				return artifact.InternalError("a change lies inside the moved subtree %q", root)
			}
		}
	}
	return nil
}

// isAncestor reports whether ancestor is a proper ancestor of path.
func isAncestor(ancestor, path artifact.Path) bool {
	return strings.HasPrefix(string(path), string(ancestor)+"/")
}

func (ref SourceRef) validate(side string) error {
	if ref.Kind == "" {
		return artifact.InternalError("source %s has an empty kind", side)
	}
	if ref.NodeCount < 1 {
		return artifact.InternalError("source %s has node count %d without a root", side, ref.NodeCount)
	}
	return nil
}

func (c Change) validate() error {
	if c.Path == "" {
		return artifact.InternalError("change has an empty path")
	}
	if !c.Op.Valid() {
		return artifact.InternalError("change %q has unsupported operation %q", c.Path, c.Op)
	}
	switch c.Op {
	case OpAdded:
		if c.Before != nil || c.After == nil {
			return artifact.InternalError("added change %q must carry only after", c.Path)
		}
	case OpRemoved:
		if c.Before == nil || c.After != nil {
			return artifact.InternalError("removed change %q must carry only before", c.Path)
		}
	case OpChanged:
		if c.Before == nil || c.After == nil {
			return artifact.InternalError("changed change %q must carry before and after", c.Path)
		}
		if c.Before.Equal(*c.After) {
			return artifact.InternalError("changed change %q has identical before and after", c.Path)
		}
	case OpMoved:
		if err := c.validateMove(); err != nil {
			return err
		}
	}
	if c.Op != OpMoved && (c.From != "" || c.Evidence != "" || c.Nodes != 0) {
		return artifact.InternalError("%s change %q carries move fields", c.Op, c.Path)
	}
	if c.Before != nil {
		if err := c.Before.validate(c.Path); err != nil {
			return err
		}
	}
	if c.After != nil {
		if err := c.After.validate(c.Path); err != nil {
			return err
		}
	}
	return nil
}

// validateMove checks the shape of one MOVED change. Under structural-v1 a
// move never changes the node state, and evidence is "name" exactly when the
// move covers a single record.
func (c Change) validateMove() error {
	if c.Before == nil || c.After == nil {
		return artifact.InternalError("moved change %q must carry before and after", c.Path)
	}
	if !c.Before.Equal(*c.After) {
		return artifact.InternalError("moved change %q changes the node state", c.Path)
	}
	if c.From == "" {
		return artifact.InternalError("moved change %q has no origin", c.Path)
	}
	if c.From == c.Path || isAncestor(c.From, c.Path) || isAncestor(c.Path, c.From) {
		return artifact.InternalError("moved change %q has an overlapping origin %q", c.Path, c.From)
	}
	if !c.Evidence.Valid() {
		return artifact.InternalError("moved change %q has unsupported evidence %q", c.Path, c.Evidence)
	}
	if c.Nodes < 1 {
		return artifact.InternalError("moved change %q covers %d nodes", c.Path, c.Nodes)
	}
	if (c.Evidence == EvidenceName) != (c.Nodes == 1) {
		return artifact.InternalError("moved change %q has evidence %q for %d nodes", c.Path, c.Evidence, c.Nodes)
	}
	if c.Evidence == EvidenceSubtree && c.Before.Kind != artifact.KindDirectory {
		return artifact.InternalError("moved change %q has subtree evidence on kind %q", c.Path, c.Before.Kind)
	}
	return nil
}

func (s NodeState) validate(path artifact.Path) error {
	if !s.Kind.Valid() {
		return artifact.InternalError("change %q has unsupported kind %q", path, s.Kind)
	}
	if s.Kind.HasTarget() != (s.Target != nil) {
		return artifact.InternalError("change %q kind %q has inconsistent target presence", path, s.Kind)
	}
	return nil
}

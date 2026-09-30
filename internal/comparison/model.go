// Package comparison implements the Structural Diff engine: it compares two
// structural sources through Identity Projection v1 records and produces a
// validated StructuralDiff model. It contains no presentation, no CLI, and no
// filesystem traversal of its own.
package comparison

import (
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
)

const (
	// Version is the comparison model version frozen by Diff Result Schema v1.
	Version = 1
	// IdentityProjectionVersion is the Identity Projection contract the
	// comparison is anchored to. It must equal identity.ProjectionVersion.
	IdentityProjectionVersion = identity.ProjectionVersion
)

// Operation is the closed Structural Diff vocabulary. There is no MOVED or
// RENAMED operation: a rename is one REMOVED plus one ADDED.
type Operation string

const (
	// OpAdded marks a path present in B and absent from A.
	OpAdded Operation = "ADDED"
	// OpRemoved marks a path present in A and absent from B.
	OpRemoved Operation = "REMOVED"
	// OpChanged marks a path present in both whose Identity Projection v1
	// record differs (kind, or target of a target-carrying kind).
	OpChanged Operation = "CHANGED"
)

// Valid reports whether op is one of the v1 operations.
func (op Operation) Valid() bool {
	switch op {
	case OpAdded, OpRemoved, OpChanged:
		return true
	default:
		return false
	}
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

// Metadata describes the comparison itself.
type Metadata struct {
	ComparisonVersion         int
	IdentityProjectionVersion int
	A                         SourceRef
	B                         SourceRef
}

// Change is one canonical path that differs between A (before) and B (after).
// ADDED carries only After, REMOVED only Before, CHANGED carries both.
type Change struct {
	Path   artifact.Path
	Op     Operation
	Before *NodeState
	After  *NodeState
}

// Summary counts changes by operation. Total is always Added+Removed+Changed.
type Summary struct {
	Added   int
	Removed int
	Changed int
	Total   int
}

// StructuralDiff is the validated comparison outcome.
type StructuralDiff struct {
	Metadata Metadata
	Changes  []Change
	Summary  Summary
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
	if err := d.Metadata.A.validate("a"); err != nil {
		return err
	}
	if err := d.Metadata.B.validate("b"); err != nil {
		return err
	}
	added, removed, changed := 0, 0, 0
	for i, change := range d.Changes {
		if err := change.validate(); err != nil {
			return err
		}
		if change.Path == artifact.RootPath {
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
		}
	}
	if d.Summary.Added != added || d.Summary.Removed != removed || d.Summary.Changed != changed {
		return artifact.InternalError("summary %d/%d/%d does not match changes %d/%d/%d",
			d.Summary.Added, d.Summary.Removed, d.Summary.Changed, added, removed, changed)
	}
	if d.Summary.Total != added+removed+changed || d.Summary.Total != len(d.Changes) {
		return artifact.InternalError("summary total %d does not match %d changes", d.Summary.Total, len(d.Changes))
	}
	return nil
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

func (s NodeState) validate(path artifact.Path) error {
	if !s.Kind.Valid() {
		return artifact.InternalError("change %q has unsupported kind %q", path, s.Kind)
	}
	if s.Kind.HasTarget() != (s.Target != nil) {
		return artifact.InternalError("change %q kind %q has inconsistent target presence", path, s.Kind)
	}
	return nil
}

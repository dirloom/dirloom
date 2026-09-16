// Package diagram defines renderer-independent graph projections.
package diagram

const (
	// ContractVersion is the current shape and invariant version of Document.
	ContractVersion = 1

	// LargeGraphWarningThreshold is the advisory threshold used by the CLI.
	LargeGraphWarningThreshold = 500
)

// View identifies the semantic projection represented by a document.
type View string

const (
	ViewStructure View = "structure"
)

// Direction identifies the preferred graph flow.
type Direction string

const (
	DirectionTopDown   Direction = "top-down"
	DirectionLeftRight Direction = "left-right"
)

// NodeKind identifies the structural kind of a diagram node.
type NodeKind string

const (
	NodeDirectory NodeKind = "directory"
	NodeFile      NodeKind = "file"
	NodeSymlink   NodeKind = "symlink"
)

// Relation identifies the semantic meaning of an edge.
type Relation string

const (
	RelationContains Relation = "contains"
)

// Node is a stable diagram node independent from any output dialect.
type Node struct {
	ID    string
	Label string
	Kind  NodeKind
}

// Edge is a directed semantic relation between two nodes.
type Edge struct {
	From     string
	To       string
	Relation Relation
}

// Document is the canonical graph projection consumed by all diagram encoders.
type Document struct {
	ContractVersion int
	View            View
	Direction       Direction
	Nodes           []Node
	Edges           []Edge
}

// Options controls projection behavior.
type Options struct {
	View      View
	Direction Direction
	MaxNodes  *int
}

// DefaultOptions returns the stable built-in diagram defaults.
func DefaultOptions() Options {
	return Options{View: ViewStructure, Direction: DirectionTopDown}
}

package artifact

import "golang.org/x/text/unicode/norm"

// Artifact is a Canonical Structural Artifact v1. After Build/Validate it is
// logically immutable: identity code must copy, not mutate, this value.
type Artifact struct {
	// DisplayRootName is the physical folder label from observation. It is
	// outside Identity Projection v1.
	DisplayRootName string
	Root            Node
}

// Node is one structural entry.
type Node struct {
	Path     Path
	Name     string
	Kind     Kind
	Target   string
	Children []Node
}

// Clone returns a deep copy so callers can permute children without sharing.
func (a Artifact) Clone() Artifact {
	return Artifact{DisplayRootName: a.DisplayRootName, Root: cloneNode(a.Root)}
}

func cloneNode(node Node) Node {
	out := node
	if len(node.Children) == 0 {
		out.Children = nil
		if node.Children != nil {
			out.Children = []Node{}
		}
		return out
	}
	out.Children = make([]Node, len(node.Children))
	for i, child := range node.Children {
		out.Children[i] = cloneNode(child)
	}
	return out
}

// CountNodes returns the number of nodes including the root.
func (a Artifact) CountNodes() int {
	return countNodes(a.Root)
}

func countNodes(node Node) int {
	total := 1
	for _, child := range node.Children {
		total += countNodes(child)
	}
	return total
}

// CollisionSet detects two raw paths that NFC to the same canonical path.
type CollisionSet struct {
	first map[Path]string
}

// NewCollisionSet prepares an empty detector.
func NewCollisionSet() *CollisionSet {
	return &CollisionSet{first: make(map[Path]string)}
}

// Add records raw -> canonical. Distinct raw strings that share a canonical
// path are a collision. Repeating the same raw string is a duplicate node.
func (set *CollisionSet) Add(raw string, canonical Path) error {
	if set.first == nil {
		set.first = make(map[Path]string)
	}
	previous, exists := set.first[canonical]
	if !exists {
		set.first[canonical] = raw
		return nil
	}
	if previous == raw {
		return invariant("duplicate node %q", raw)
	}
	return collision(previous, raw, canonical.String())
}

// NormalizeRaw canonicalizes raw and records it in the collision set.
func (set *CollisionSet) NormalizeRaw(raw string, nativeSeparator byte) (Path, error) {
	canonical, err := Canonicalize(raw, nativeSeparator)
	if err != nil {
		return "", err
	}
	nfcRaw := norm.NFC.String(raw)
	if err := set.Add(nfcRaw, canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

// Package tree defines Dirloom's renderer-independent filesystem model.
package tree

import (
	"sort"
	"strings"
)

// NodeType identifies the kind of filesystem entry represented by a Node.
type NodeType string

const (
	NodeDirectory NodeType = "directory"
	NodeFile      NodeType = "file"
	NodeSymlink   NodeType = "symlink"
)

// Node is a deterministic, renderer-independent representation of an entry.
// Path is always relative to the inspected root and uses forward slashes. It
// is internal metadata and is intentionally absent from public renderings.
type Node struct {
	Name     string
	Path     string
	Type     NodeType
	Target   string
	Children []*Node
}

// Sort recursively orders directories first, then all terminal entries. Names
// are compared using deterministic Unicode case folding, with bytewise names
// and normalized relative paths as tie-breakers.
func Sort(root *Node) {
	if root == nil {
		return
	}

	sort.Slice(root.Children, func(i, j int) bool {
		left, right := root.Children[i], root.Children[j]
		leftRank, rightRank := typeRank(left.Type), typeRank(right.Type)
		if leftRank != rightRank {
			return leftRank < rightRank
		}

		leftFolded, rightFolded := strings.ToLower(left.Name), strings.ToLower(right.Name)
		if leftFolded != rightFolded {
			return leftFolded < rightFolded
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.Path < right.Path
	})

	for _, child := range root.Children {
		Sort(child)
	}
}

func typeRank(kind NodeType) int {
	if kind == NodeDirectory {
		return 0
	}
	return 1
}

package diagram

import (
	"fmt"
	"strings"

	"github.com/dirloom/dirloom/internal/tree"
)

type projectionItem struct {
	node        *tree.Node
	parentID    string
	logicalPath string
}

// ProjectStructure creates the canonical structure view from a sorted tree.
func ProjectStructure(root *tree.Node, options Options) (Document, error) {
	return projectStructure(root, options, stableNodeID)
}

// CountNodes returns the number of nodes without recursion.
func CountNodes(root *tree.Node) int {
	if root == nil {
		return 0
	}
	count := 0
	stack := []*tree.Node{root}
	for len(stack) > 0 {
		index := len(stack) - 1
		node := stack[index]
		stack = stack[:index]
		count++
		stack = append(stack, node.Children...)
	}
	return count
}

func projectStructure(root *tree.Node, options Options, hasher idHasher) (Document, error) {
	if root == nil {
		return Document{}, fmt.Errorf("project diagram: root must not be nil")
	}
	if options.View == "" {
		options.View = ViewStructure
	}
	if options.Direction == "" {
		options.Direction = DirectionTopDown
	}
	if options.View != ViewStructure {
		return Document{}, fmt.Errorf("project diagram: unsupported view %q", options.View)
	}
	if options.Direction != DirectionTopDown && options.Direction != DirectionLeftRight {
		return Document{}, fmt.Errorf("project diagram: unsupported direction %q", options.Direction)
	}
	if options.MaxNodes != nil && *options.MaxNodes <= 0 {
		return Document{}, fmt.Errorf("project diagram: max nodes must be positive or unlimited")
	}

	document := Document{
		ContractVersion: ContractVersion,
		View:            options.View,
		Direction:       options.Direction,
		Nodes:           make([]Node, 0),
		Edges:           make([]Edge, 0),
	}
	seenIDs := make(map[string]string)
	stack := []projectionItem{{node: root, logicalPath: "."}}
	for len(stack) > 0 {
		index := len(stack) - 1
		item := stack[index]
		stack = stack[:index]

		id := "n_root"
		if item.parentID != "" {
			identity := string(item.node.Type) + "\x00" + item.logicalPath
			id = hasher(identity)
			if previous, exists := seenIDs[id]; exists && previous != identity {
				return Document{}, fmt.Errorf("project diagram: node identifier collision for %q and %q", previous, identity)
			}
			seenIDs[id] = identity
		}

		kind, err := diagramNodeKind(item.node.Type)
		if err != nil {
			return Document{}, err
		}
		document.Nodes = append(document.Nodes, Node{ID: id, Label: structureLabel(item.node), Kind: kind})
		if item.parentID != "" {
			document.Edges = append(document.Edges, Edge{From: item.parentID, To: id, Relation: RelationContains})
		}
		if options.MaxNodes != nil && len(document.Nodes) > *options.MaxNodes {
			return Document{}, fmt.Errorf("diagram contains %d nodes, exceeding the explicit limit of %d; refine the view with --depth, --dirs-only, or --ignore", len(document.Nodes), *options.MaxNodes)
		}

		for childIndex := len(item.node.Children) - 1; childIndex >= 0; childIndex-- {
			child := item.node.Children[childIndex]
			logicalPath := child.Path
			if logicalPath == "" {
				logicalPath = child.Name
				if item.logicalPath != "." && item.logicalPath != "" {
					logicalPath = strings.TrimSuffix(item.logicalPath, "/") + "/" + child.Name
				}
			}
			stack = append(stack, projectionItem{node: child, parentID: id, logicalPath: logicalPath})
		}
	}
	if err := Validate(document); err != nil {
		return Document{}, fmt.Errorf("project diagram: %w", err)
	}
	return document, nil
}

func diagramNodeKind(kind tree.NodeType) (NodeKind, error) {
	switch kind {
	case tree.NodeDirectory:
		return NodeDirectory, nil
	case tree.NodeFile:
		return NodeFile, nil
	case tree.NodeSymlink:
		return NodeSymlink, nil
	default:
		return "", fmt.Errorf("project diagram: unsupported node type %q", kind)
	}
}

func structureLabel(node *tree.Node) string {
	switch node.Type {
	case tree.NodeDirectory:
		return node.Name + "/"
	case tree.NodeSymlink:
		if node.Target != "" {
			return node.Name + " -> " + node.Target
		}
		return node.Name + " [symlink]"
	default:
		return node.Name
	}
}

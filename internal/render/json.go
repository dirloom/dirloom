package render

import (
	"encoding/json"
	"io"

	"github.com/dirloom/dirloom/internal/tree"
)

type jsonDocument struct {
	SchemaVersion int      `json:"schemaVersion"`
	Root          jsonNode `json:"root"`
}

type jsonNode struct {
	Name     string        `json:"name"`
	Type     tree.NodeType `json:"type"`
	Target   string        `json:"target,omitempty"`
	Children *[]jsonNode   `json:"children,omitempty"`
}

type jsonRenderer struct{}

func (jsonRenderer) Render(w io.Writer, root *tree.Node) error {
	document := jsonDocument{SchemaVersion: 1, Root: toJSONNode(root)}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func toJSONNode(node *tree.Node) jsonNode {
	result := jsonNode{Name: node.Name, Type: node.Type, Target: node.Target}
	if node.Type == tree.NodeDirectory {
		children := make([]jsonNode, 0, len(node.Children))
		for _, child := range node.Children {
			children = append(children, toJSONNode(child))
		}
		result.Children = &children
	}
	return result
}

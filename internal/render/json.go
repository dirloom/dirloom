package render

import (
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

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

// InvalidUTF8Error reports a tree string that is not valid UTF-8. Tree JSON
// refuses it: encoding/json would replace each invalid byte with U+FFFD and
// publish a name or a target that does not exist on disk.
type InvalidUTF8Error struct {
	// Path is the slash-separated path of the entry from the root, without
	// the root's own name; it is empty for the root.
	Path string
	// Field is the serialized field: "name" or "target".
	Field string
	// Value is the invalid string.
	Value string
}

func (e *InvalidUTF8Error) Error() string {
	entry := "the root"
	if e.Path != "" {
		entry = fmt.Sprintf("entry %q", e.Path)
	}
	// %q escapes the invalid bytes, so the diagnostic itself stays UTF-8.
	if e.Field == "name" && e.Path != "" {
		return fmt.Sprintf("%s: name is not valid UTF-8", entry)
	}
	return fmt.Sprintf("%s: %s %q is not valid UTF-8", entry, e.Field, e.Value)
}

func (jsonRenderer) Render(w io.Writer, root *tree.Node) error {
	// Every string is checked before any byte of the document is built or
	// written, so a refusal leaves no partial output.
	if err := validateJSONStrings(root, ""); err != nil {
		return err
	}
	document := jsonDocument{SchemaVersion: 1, Root: toJSONNode(root)}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

// validateJSONStrings checks, in document order, every string tree JSON
// serializes: each name and each link target. The type is one of three
// ASCII constants.
func validateJSONStrings(node *tree.Node, path string) error {
	if !utf8.ValidString(node.Name) {
		return &InvalidUTF8Error{Path: path, Field: "name", Value: node.Name}
	}
	if !utf8.ValidString(node.Target) {
		return &InvalidUTF8Error{Path: path, Field: "target", Value: node.Target}
	}
	for _, child := range node.Children {
		childPath := child.Name
		if path != "" {
			childPath = path + "/" + child.Name
		}
		if err := validateJSONStrings(child, childPath); err != nil {
			return err
		}
	}
	return nil
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

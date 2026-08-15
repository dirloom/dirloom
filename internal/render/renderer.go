// Package render turns a tree model into deterministic public formats.
package render

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/tree"
)

const (
	FormatText         = "text"
	FormatMarkdown     = "markdown"
	FormatMarkdownTree = "markdown-tree"
	FormatJSON         = "json"
	StyleUnicode       = "unicode"
	StyleASCII         = "ascii"
)

// Renderer writes exactly one complete representation of a tree.
type Renderer interface {
	Render(io.Writer, *tree.Node) error
}

// NodeContext supplies presentation-only metadata without changing the tree.
type NodeContext struct {
	Path    string
	Name    string
	Display string
	Type    tree.NodeType
}

// Decorator projects terminal presentation onto canonical text segments.
// Implementations must not add, remove, rename, or reorder nodes.
type Decorator interface {
	Edge(string) string
	Node(NodeContext) string
}

// New selects a renderer for validated format and style values.
func New(format, style string, decorators ...Decorator) (Renderer, error) {
	var decorator Decorator
	if len(decorators) > 0 {
		decorator = decorators[0]
	}
	switch format {
	case FormatText:
		return newTextRenderer(style, decorator)
	case FormatMarkdown:
		// Markdown is always canonical and deliberately ignores presentation.
		text, err := newTextRenderer(style, nil)
		if err != nil {
			return nil, err
		}
		return markdownRenderer{text: text}, nil
	case FormatMarkdownTree:
		return markdownTreeRenderer{}, nil
	case FormatJSON:
		return jsonRenderer{}, nil
	default:
		return nil, fmt.Errorf("unsupported format %q (expected text, markdown, markdown-tree, or json)", format)
	}
}

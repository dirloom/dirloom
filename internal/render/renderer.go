// Package render turns a tree model into deterministic public formats.
package render

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/diagram"
	"github.com/dirloom/dirloom/internal/outputformat"
	"github.com/dirloom/dirloom/internal/tree"
)

const (
	FormatText         = outputformat.Text
	FormatMarkdown     = outputformat.Markdown
	FormatMarkdownTree = outputformat.MarkdownTree
	FormatJSON         = outputformat.JSON
	FormatMermaid      = outputformat.Mermaid
	FormatGraphviz     = outputformat.Graphviz
	FormatD2           = outputformat.D2
	StyleUnicode       = "unicode"
	StyleASCII         = "ascii"
)

// Renderer writes exactly one complete representation of a tree.
type Renderer interface {
	Render(io.Writer, *tree.Node) error
}

// Options selects one renderer and its projection settings.
type Options struct {
	Format    string
	Style     string
	Diagram   diagram.Options
	Decorator Decorator
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
	options := Options{Format: format, Style: style, Diagram: diagram.DefaultOptions()}
	if len(decorators) > 0 {
		options.Decorator = decorators[0]
	}
	return NewConfigured(options)
}

// NewConfigured selects a renderer from explicit structured options.
func NewConfigured(options Options) (Renderer, error) {
	format := options.Format
	canonicalFormat, ok := outputformat.Canonical(format)
	if !ok {
		return nil, outputformat.Validate(format)
	}
	format = canonicalFormat
	defaultDiagram := diagram.DefaultOptions()
	if options.Diagram.View == "" {
		options.Diagram.View = defaultDiagram.View
	}
	if options.Diagram.Direction == "" {
		options.Diagram.Direction = defaultDiagram.Direction
	}
	switch format {
	case FormatText:
		return newTextRenderer(options.Style, options.Decorator)
	case FormatMarkdown:
		// Markdown is always canonical and deliberately ignores presentation.
		text, err := newTextRenderer(options.Style, nil)
		if err != nil {
			return nil, err
		}
		return markdownRenderer{text: text}, nil
	case FormatMarkdownTree:
		return markdownTreeRenderer{}, nil
	case FormatJSON:
		return jsonRenderer{}, nil
	case FormatMermaid, FormatGraphviz, FormatD2:
		return diagramRenderer{format: format, options: options.Diagram}, nil
	default:
		return nil, fmt.Errorf("format %q is recognized but has no renderer", format)
	}
}

type diagramRenderer struct {
	format  string
	options diagram.Options
}

func (renderer diagramRenderer) Render(writer io.Writer, root *tree.Node) error {
	document, err := diagram.ProjectStructure(root, renderer.options)
	if err != nil {
		return err
	}
	switch renderer.format {
	case FormatMermaid:
		return RenderMermaid(document, writer)
	case FormatGraphviz:
		return RenderGraphviz(document, writer)
	case FormatD2:
		return RenderD2(document, writer)
	default:
		return fmt.Errorf("unsupported diagram format %q", renderer.format)
	}
}

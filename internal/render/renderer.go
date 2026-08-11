// Package render turns a tree model into deterministic public formats.
package render

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/tree"
)

const (
	FormatText     = "text"
	FormatMarkdown = "markdown"
	FormatJSON     = "json"
	StyleUnicode   = "unicode"
	StyleASCII     = "ascii"
)

// Renderer writes exactly one complete representation of a tree.
type Renderer interface {
	Render(io.Writer, *tree.Node) error
}

// New selects a renderer for validated format and style values.
func New(format, style string) (Renderer, error) {
	switch format {
	case FormatText:
		return newTextRenderer(style)
	case FormatMarkdown:
		text, err := newTextRenderer(style)
		if err != nil {
			return nil, err
		}
		return markdownRenderer{text: text}, nil
	case FormatJSON:
		return jsonRenderer{}, nil
	default:
		return nil, fmt.Errorf("unsupported format %q (expected text, markdown, or json)", format)
	}
}

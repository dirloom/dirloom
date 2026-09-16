package render

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/dirloom/dirloom/internal/tree"
)

// markdownTreeRenderer writes a semantic Markdown list. It deliberately does
// not reuse the text drawing or presentation layer: this is a deterministic
// document artifact, not a terminal projection.
type markdownTreeRenderer struct{}

func (markdownTreeRenderer) Render(w io.Writer, root *tree.Node) error {
	buffer := bufio.NewWriter(w)
	if err := writeMarkdownTreeNode(buffer, root, 0); err != nil {
		return err
	}
	return buffer.Flush()
}

func writeMarkdownTreeNode(w io.Writer, node *tree.Node, depth int) error {
	if _, err := fmt.Fprintf(w, "%s- %s\n", strings.Repeat("  ", depth), markdownTreeLabel(node)); err != nil {
		return err
	}
	if node.Type != tree.NodeDirectory {
		return nil
	}
	for _, child := range node.Children {
		if err := writeMarkdownTreeNode(w, child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func markdownTreeLabel(node *tree.Node) string {
	switch node.Type {
	case tree.NodeDirectory:
		return markdownCodeSpan(node.Name + "/")
	case tree.NodeSymlink:
		name := markdownCodeSpan(node.Name)
		if node.Target != "" {
			return name + " -> " + markdownCodeSpan(node.Target)
		}
		return name + " [symlink]"
	default:
		return markdownCodeSpan(node.Name)
	}
}

func markdownCodeSpan(value string) string {
	value = escapeMarkdownTreeValue(value)
	longest := 0
	current := 0
	for _, r := range value {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	delimiter := strings.Repeat("`", longest+1)
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return delimiter + " " + value + " " + delimiter
	}
	return delimiter + value + delimiter
}

func escapeMarkdownTreeValue(value string) string {
	if value != "" && strings.Trim(value, " ") == "" {
		return strings.Repeat(`\x20`, len(value))
	}
	var escaped strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			escaped.WriteString(`\\`)
		case '\t':
			escaped.WriteString(`\t`)
		case '\n':
			escaped.WriteString(`\n`)
		case '\r':
			escaped.WriteString(`\r`)
		default:
			if unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp) || isBidirectionalControl(r) {
				_, _ = fmt.Fprintf(&escaped, `\u{%04X}`, r)
			} else {
				escaped.WriteRune(r)
			}
		}
	}
	return escaped.String()
}

func isBidirectionalControl(r rune) bool {
	switch r {
	case '\u061c', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}

package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/tree"
)

type drawing struct {
	branch   string
	last     string
	vertical string
	space    string
}

type textRenderer struct {
	drawing   drawing
	decorator Decorator
}

func newTextRenderer(style string, decorator Decorator) (textRenderer, error) {
	switch style {
	case StyleUnicode:
		return textRenderer{drawing: drawing{
			branch: "├── ", last: "└── ", vertical: "│   ", space: "    ",
		}, decorator: decorator}, nil
	case StyleASCII:
		return textRenderer{drawing: drawing{
			branch: "|-- ", last: "`-- ", vertical: "|   ", space: "    ",
		}, decorator: decorator}, nil
	default:
		return textRenderer{}, fmt.Errorf("unsupported style %q (expected unicode or ascii)", style)
	}
}

func (r textRenderer) Render(w io.Writer, root *tree.Node) error {
	buffer := bufio.NewWriter(w)
	if _, err := fmt.Fprintln(buffer, r.decorateNode(root)); err != nil {
		return err
	}
	if err := r.renderChildren(buffer, root.Children, ""); err != nil {
		return err
	}
	return buffer.Flush()
}

func (r textRenderer) renderChildren(w io.Writer, children []*tree.Node, prefix string) error {
	for index, child := range children {
		last := index == len(children)-1
		connector := r.drawing.branch
		nextPrefix := prefix + r.drawing.vertical
		if last {
			connector = r.drawing.last
			nextPrefix = prefix + r.drawing.space
		}
		if _, err := fmt.Fprintf(w, "%s%s%s\n", r.decorateEdge(prefix), r.decorateEdge(connector), r.decorateNode(child)); err != nil {
			return err
		}
		if child.Type == tree.NodeDirectory {
			if err := r.renderChildren(w, child.Children, nextPrefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r textRenderer) decorateEdge(value string) string {
	if r.decorator == nil || value == "" {
		return value
	}
	return r.decorator.Edge(value)
}

func (r textRenderer) decorateNode(node *tree.Node) string {
	display := displayName(node)
	if r.decorator == nil {
		return display
	}
	return r.decorator.Node(NodeContext{Path: node.Path, Name: node.Name, Display: display, Type: node.Type})
}

func displayName(node *tree.Node) string {
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

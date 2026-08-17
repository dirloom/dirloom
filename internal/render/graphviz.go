package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/diagram"
)

// RenderGraphviz writes Graphviz DOT from a validated diagram document.
func RenderGraphviz(document diagram.Document, writer io.Writer) error {
	if err := diagram.Validate(document); err != nil {
		return fmt.Errorf("render Graphviz: %w", err)
	}
	direction := "TB"
	if document.Direction == diagram.DirectionLeftRight {
		direction = "LR"
	}
	buffer := bufio.NewWriter(writer)
	if _, err := fmt.Fprintf(buffer, "// dirloom-diagram-contract: %d; view: %s; direction: %s\nstrict digraph dirloom {\n  rankdir=%s;\n",
		document.ContractVersion, document.View, document.Direction, direction); err != nil {
		return err
	}
	for _, node := range document.Nodes {
		if _, err := fmt.Fprintf(buffer, "  %s [label=\"%s\"];\n", node.ID, escapeDOTLabel(node.Label)); err != nil {
			return err
		}
	}
	for _, edge := range document.Edges {
		if _, err := fmt.Fprintf(buffer, "  %s -> %s;\n", edge.From, edge.To); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(buffer, "}"); err != nil {
		return err
	}
	return buffer.Flush()
}

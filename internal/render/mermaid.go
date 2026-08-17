package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/diagram"
)

// RenderMermaid writes a Mermaid flowchart from a validated diagram document.
func RenderMermaid(document diagram.Document, writer io.Writer) error {
	if err := diagram.Validate(document); err != nil {
		return fmt.Errorf("render Mermaid: %w", err)
	}
	direction := "TB"
	if document.Direction == diagram.DirectionLeftRight {
		direction = "LR"
	}
	buffer := bufio.NewWriter(writer)
	if _, err := fmt.Fprintf(buffer, "%%%% dirloom-diagram-contract: %d; view: %s; direction: %s\nflowchart %s\n",
		document.ContractVersion, document.View, document.Direction, direction); err != nil {
		return err
	}
	for _, node := range document.Nodes {
		if _, err := fmt.Fprintf(buffer, "  %s[\"%s\"]\n", node.ID, escapeMermaidLabel(node.Label)); err != nil {
			return err
		}
	}
	for _, edge := range document.Edges {
		if _, err := fmt.Fprintf(buffer, "  %s --> %s\n", edge.From, edge.To); err != nil {
			return err
		}
	}
	return buffer.Flush()
}

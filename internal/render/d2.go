package render

import (
	"bufio"
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/diagram"
)

// RenderD2 writes D2 source from a validated diagram document.
func RenderD2(document diagram.Document, writer io.Writer) error {
	if err := diagram.Validate(document); err != nil {
		return fmt.Errorf("render D2: %w", err)
	}
	direction := "down"
	if document.Direction == diagram.DirectionLeftRight {
		direction = "right"
	}
	buffer := bufio.NewWriter(writer)
	if _, err := fmt.Fprintf(buffer, "# dirloom-diagram-contract: %d; view: %s; direction: %s\ndirection: %s\n",
		document.ContractVersion, document.View, document.Direction, direction); err != nil {
		return err
	}
	for _, node := range document.Nodes {
		if _, err := fmt.Fprintf(buffer, "%s: \"%s\"\n", node.ID, escapeD2Label(node.Label)); err != nil {
			return err
		}
	}
	for _, edge := range document.Edges {
		if _, err := fmt.Fprintf(buffer, "%s -> %s\n", edge.From, edge.To); err != nil {
			return err
		}
	}
	return buffer.Flush()
}

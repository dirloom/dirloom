package diagram

import "fmt"

// Validate checks the structural invariants required by every encoder.
func Validate(document Document) error {
	if document.ContractVersion != ContractVersion {
		return fmt.Errorf("unsupported diagram contract version %d", document.ContractVersion)
	}
	if document.View != ViewStructure {
		return fmt.Errorf("unsupported diagram view %q", document.View)
	}
	if document.Direction != DirectionTopDown && document.Direction != DirectionLeftRight {
		return fmt.Errorf("unsupported diagram direction %q", document.Direction)
	}
	if len(document.Nodes) == 0 {
		return fmt.Errorf("diagram must contain at least one node")
	}
	if document.Nodes[0].ID != "n_root" {
		return fmt.Errorf("first diagram node must be n_root")
	}

	nodes := make(map[string]struct{}, len(document.Nodes))
	for index, node := range document.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node %d has an empty identifier", index)
		}
		if node.Label == "" {
			return fmt.Errorf("node %q has an empty label", node.ID)
		}
		switch node.Kind {
		case NodeDirectory, NodeFile, NodeSymlink:
		default:
			return fmt.Errorf("node %q has unsupported kind %q", node.ID, node.Kind)
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("duplicate node identifier %q", node.ID)
		}
		nodes[node.ID] = struct{}{}
	}

	if len(document.Edges) != len(document.Nodes)-1 {
		return fmt.Errorf("structure view must contain exactly one edge per non-root node")
	}
	incoming := make(map[string]int, len(document.Nodes))
	for index, edge := range document.Edges {
		if edge.Relation != RelationContains {
			return fmt.Errorf("edge %d has unsupported relation %q", index, edge.Relation)
		}
		if _, exists := nodes[edge.From]; !exists {
			return fmt.Errorf("edge %d references missing source %q", index, edge.From)
		}
		if _, exists := nodes[edge.To]; !exists {
			return fmt.Errorf("edge %d references missing target %q", index, edge.To)
		}
		if edge.From == edge.To {
			return fmt.Errorf("edge %d is self-referential", index)
		}
		incoming[edge.To]++
	}
	if incoming["n_root"] != 0 {
		return fmt.Errorf("root node must not have an incoming edge")
	}
	for _, node := range document.Nodes[1:] {
		if incoming[node.ID] != 1 {
			return fmt.Errorf("node %q must have exactly one incoming edge", node.ID)
		}
	}
	return nil
}

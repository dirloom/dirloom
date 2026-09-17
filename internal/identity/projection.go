package identity

import (
	"sort"

	"github.com/dirloom/dirloom/internal/artifact"
)

const (
	// ProjectionVersion is Identity Projection v1.
	ProjectionVersion = 1
	// EncodingVersion is Canonical Identity Encoding v1.
	EncodingVersion = 1
	// Namespace is the fingerprint URI namespace.
	Namespace = "dlm"
)

// Record is one Identity Projection v1 row.
type Record struct {
	Path   artifact.Path
	Kind   artifact.Kind
	Target string
}

// ProjectV1 flattens a validated artifact into path-sorted identity records.
// It does not mutate the artifact.
func ProjectV1(art artifact.Artifact) ([]Record, error) {
	if err := art.Validate(); err != nil {
		return nil, err
	}
	records := make([]Record, 0, art.CountNodes())
	collect(art.Root, &records)
	sort.Slice(records, func(i, j int) bool {
		return records[i].Path < records[j].Path
	})
	return records, nil
}

func collect(node artifact.Node, records *[]Record) {
	record := Record{Path: node.Path, Kind: node.Kind}
	if node.Kind.HasTarget() {
		record.Target = node.Target
	}
	*records = append(*records, record)
	for _, child := range node.Children {
		collect(child, records)
	}
}

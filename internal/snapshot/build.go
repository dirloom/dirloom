package snapshot

import (
	"sort"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
)

// Build constructs a Snapshot v1 document from a validated artifact and capture
// semantics. It does not read config, filesystem, presentation or provenance.
func Build(art artifact.Artifact, capture CaptureV1) (Document, error) {
	if err := art.Validate(); err != nil {
		return Document{}, invalidArtifact(err)
	}
	proj, err := ProjectV1(art)
	if err != nil {
		return Document{}, err
	}
	fp, err := identity.Compute(art)
	if err != nil {
		return Document{}, invalidArtifact(err)
	}
	capture = capture.Clone()
	capture.NormalizeIgnore()
	if capture.Depth != nil && *capture.Depth < 0 {
		return Document{}, invalidField("capture.depth must be non-negative")
	}
	return Document{
		SchemaVersion:    SchemaVersion,
		ArtifactVersion:  ArtifactVersion,
		Fingerprint:      fp.String(),
		RequiredFeatures: []string{},
		Capture:          capture,
		Artifact:         proj,
	}, nil
}

// CanonicalizeDocument sorts mutable document fields for deterministic encoding
// without mutating caller-owned artifact slices beyond the document itself.
func CanonicalizeDocument(doc *Document) {
	if doc.RequiredFeatures == nil {
		doc.RequiredFeatures = []string{}
	} else {
		features := append([]string(nil), doc.RequiredFeatures...)
		sort.Strings(features)
		doc.RequiredFeatures = features
	}
	doc.Capture.NormalizeIgnore()
	if doc.Artifact.Nodes != nil {
		nodes := append([]NodeV1(nil), doc.Artifact.Nodes...)
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
		doc.Artifact.Nodes = nodes
	} else {
		doc.Artifact.Nodes = []NodeV1{}
	}
}

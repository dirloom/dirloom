package snapshot

import (
	"bytes"
	"encoding/json"
	"io"
)

// wireDocument is the deterministic JSON layout for Snapshot Schema v1.
// Field order is defined by this struct and must remain stable.
type wireDocument struct {
	SchemaVersion    int         `json:"schemaVersion"`
	ArtifactVersion  int         `json:"artifactVersion"`
	Fingerprint      string      `json:"fingerprint"`
	RequiredFeatures []string    `json:"requiredFeatures"`
	Capture          wireCapture `json:"capture"`
	Artifact         ArtifactV1  `json:"artifact"`
}

type wireCapture struct {
	Depth             *int     `json:"depth"`
	DirsOnly          bool     `json:"dirsOnly"`
	Hidden            bool     `json:"hidden"`
	UseDefaultIgnores bool     `json:"useDefaultIgnores"`
	UseGitignore      bool     `json:"useGitignore"`
	Ignore            []string `json:"ignore"`
}

func toWire(doc Document) wireDocument {
	CanonicalizeDocument(&doc)
	ignore := doc.Capture.Ignore
	if ignore == nil {
		ignore = []string{}
	}
	features := doc.RequiredFeatures
	if features == nil {
		features = []string{}
	}
	nodes := doc.Artifact.Nodes
	if nodes == nil {
		nodes = []NodeV1{}
	}
	return wireDocument{
		SchemaVersion:    doc.SchemaVersion,
		ArtifactVersion:  doc.ArtifactVersion,
		Fingerprint:      doc.Fingerprint,
		RequiredFeatures: features,
		Capture: wireCapture{
			Depth:             doc.Capture.Depth,
			DirsOnly:          doc.Capture.DirsOnly,
			Hidden:            doc.Capture.Hidden,
			UseDefaultIgnores: doc.Capture.UseDefaultIgnores,
			UseGitignore:      doc.Capture.UseGitignore,
			Ignore:            ignore,
		},
		Artifact: ArtifactV1{Nodes: nodes},
	}
}

// Encode writes deterministic UTF-8 Snapshot JSON (2-space indent, no HTML
// escaping, final newline). Invalid documents are rejected before any byte
// is written.
func Encode(w io.Writer, doc Document) error {
	if _, err := Validate(doc); err != nil {
		return err
	}
	wire := toWire(doc)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(wire); err != nil {
		return writeFailure(err)
	}
	return nil
}

// Marshal returns deterministic snapshot bytes including the final newline.
func Marshal(doc Document) ([]byte, error) {
	var buf bytes.Buffer
	if err := Encode(&buf, doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

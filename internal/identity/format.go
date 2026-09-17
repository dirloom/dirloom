package identity

import (
	"encoding/json"
	"io"
)

// JSONSchemaVersion is the fingerprint JSON document version.
const JSONSchemaVersion = 1

// JSONDocument is the machine-readable fingerprint CLI contract.
type JSONDocument struct {
	SchemaVersion   int    `json:"schemaVersion"`
	Fingerprint     string `json:"fingerprint"`
	IdentityVersion int    `json:"identityVersion"`
	Algorithm       string `json:"algorithm"`
	NodeCount       int    `json:"nodeCount"`
}

// NewJSONDocument builds the v1 JSON object. NodeCount is metadata and is not
// hashed.
func NewJSONDocument(fp Fingerprint, nodeCount int) JSONDocument {
	return JSONDocument{
		SchemaVersion:   JSONSchemaVersion,
		Fingerprint:     fp.String(),
		IdentityVersion: fp.Version,
		Algorithm:       string(fp.Algorithm),
		NodeCount:       nodeCount,
	}
}

// WriteJSON writes indented JSON plus a trailing newline.
func (document JSONDocument) WriteJSON(writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

// WriteText writes the canonical fingerprint string plus a trailing newline.
func WriteText(writer io.Writer, fp Fingerprint) error {
	_, err := io.WriteString(writer, fp.String()+"\n")
	return err
}

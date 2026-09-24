package snapshot

import (
	"io"

	"github.com/dirloom/dirloom/internal/identity"
)

// Validate checks schema/artifact versions, required features, reconstructs the
// Canonical Structural Artifact, and self-verifies the embedded fingerprint.
func Validate(doc Document) (Validated, error) {
	if doc.SchemaVersion != SchemaVersion {
		return Validated{}, unsupportedSchema(doc.SchemaVersion)
	}
	if doc.ArtifactVersion != ArtifactVersion {
		return Validated{}, unsupportedArtifact(doc.ArtifactVersion)
	}
	if err := validateFeatureList(doc.RequiredFeatures); err != nil {
		return Validated{}, err
	}
	if err := checkSupportedFeatures(doc.RequiredFeatures); err != nil {
		return Validated{}, err
	}
	if doc.Fingerprint == "" {
		return Validated{}, missingField("fingerprint")
	}
	parsed, err := identity.Parse(doc.Fingerprint)
	if err != nil {
		return Validated{}, invalidField("invalid snapshot fingerprint: %v", err)
	}

	art, err := Reconstruct(doc.Artifact)
	if err != nil {
		return Validated{}, err
	}
	if err := art.Validate(); err != nil {
		return Validated{}, invalidArtifact(err)
	}

	computed, err := identity.Compute(art)
	if err != nil {
		return Validated{}, invalidArtifact(err)
	}
	if computed != parsed {
		return Validated{}, fingerprintMismatch(parsed.String(), computed.String())
	}

	canonical := doc
	CanonicalizeDocument(&canonical)
	return Validated{
		Document:    canonical,
		Artifact:    art,
		Fingerprint: computed,
	}, nil
}

// Load decodes and validates a snapshot from r.
func Load(r io.Reader) (Validated, error) {
	doc, err := Decode(r)
	if err != nil {
		return Validated{}, err
	}
	return Validate(doc)
}

// LoadBytes decodes and validates snapshot bytes.
func LoadBytes(data []byte) (Validated, error) {
	doc, err := DecodeBytes(data)
	if err != nil {
		return Validated{}, err
	}
	return Validate(doc)
}

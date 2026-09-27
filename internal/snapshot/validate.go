package snapshot

import (
	"io"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
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
	if err := assertCaptureConsistent(doc.Capture, doc.Artifact.Nodes); err != nil {
		return Validated{}, err
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

// assertCaptureConsistent rejects capture metadata that the persisted nodes
// themselves disprove. gitignore and custom ignores need the original source
// and are not rechecked here.
func assertCaptureConsistent(capture CaptureV1, nodes []NodeV1) error {
	if capture.Depth != nil && *capture.Depth < 0 {
		return invalidField("capture.depth must be non-negative")
	}
	for _, node := range nodes {
		if capture.DirsOnly && artifact.Kind(node.Kind) != artifact.KindDirectory {
			return invalidField("capture.dirsOnly forbids snapshot node %q of kind %q", node.Path, node.Kind)
		}
		if capture.Depth != nil && pathDepth(node.Path) > *capture.Depth {
			return invalidField("capture.depth %d forbids snapshot node %q", *capture.Depth, node.Path)
		}
	}
	return nil
}

// pathDepth counts canonical segments. The root "." is depth 0. '\' is not a separator.
func pathDepth(path string) int {
	if path == "" || path == string(artifact.RootPath) {
		return 0
	}
	return strings.Count(path, "/") + 1
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

package snapshot

import (
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
)

// SchemaVersion is Snapshot Schema v1.
const SchemaVersion = 1

// ArtifactVersion is Snapshot Artifact Projection v1.
const ArtifactVersion = 1

// Document is the in-memory Snapshot Schema v1 document after successful decode
// or build. Field presence is guaranteed by Build/Decode/Validate.
type Document struct {
	SchemaVersion    int
	ArtifactVersion  int
	Fingerprint      string
	RequiredFeatures []string
	Capture          CaptureV1
	Artifact         ArtifactV1
}

// Validated is a self-verified snapshot ready for reuse by verify/diff.
type Validated struct {
	Document    Document
	Artifact    artifact.Artifact
	Fingerprint identity.Fingerprint
}

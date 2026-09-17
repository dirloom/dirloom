package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/dirloom/dirloom/internal/artifact"
)

// Algorithm names the digest function used in a fingerprint.
type Algorithm string

const (
	// AlgorithmSHA256 is the v1 hash algorithm.
	AlgorithmSHA256 Algorithm = "sha256"
)

// Fingerprint is the typed identity of an artifact.
type Fingerprint struct {
	Version   int
	Algorithm Algorithm
	Digest    [32]byte
}

// String formats dlm:v1:sha256:<lowercase-hex>.
func (fp Fingerprint) String() string {
	var b strings.Builder
	b.Grow(8 + 64)
	b.WriteString(Namespace)
	b.WriteByte(':')
	b.WriteByte('v')
	b.WriteString(strconv.Itoa(fp.Version))
	b.WriteByte(':')
	b.WriteString(string(fp.Algorithm))
	b.WriteByte(':')
	b.WriteString(hex.EncodeToString(fp.Digest[:]))
	return b.String()
}

// Compute hashes Identity Projection v1 of art with SHA-256.
func Compute(art artifact.Artifact) (Fingerprint, error) {
	records, err := ProjectV1(art)
	if err != nil {
		return Fingerprint{}, err
	}
	hash := sha256.New()
	if err := EncodeV1(hash, records); err != nil {
		return Fingerprint{}, err
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return Fingerprint{Version: ProjectionVersion, Algorithm: AlgorithmSHA256, Digest: digest}, nil
}

package identity

import (
	"encoding/hex"
	"strings"
	"unicode"

	"github.com/dirloom/dirloom/internal/artifact"
)

// Parse decodes a canonical fingerprint string.
func Parse(value string) (Fingerprint, error) {
	if value == "" {
		return Fingerprint{}, artifact.InvalidFingerprint("fingerprint must not be empty")
	}
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		return Fingerprint{}, artifact.InvalidFingerprint("invalid fingerprint representation %q", value)
	}
	if parts[0] != Namespace {
		return Fingerprint{}, artifact.InvalidFingerprint("unknown fingerprint namespace %q", parts[0])
	}
	if parts[1] != "v1" {
		return Fingerprint{}, artifact.InvalidFingerprint("invalid fingerprint version %q", parts[1])
	}
	if parts[2] != string(AlgorithmSHA256) {
		return Fingerprint{}, artifact.InvalidFingerprint("unknown fingerprint algorithm %q", parts[2])
	}
	digestHex := parts[3]
	if digestHex == "" {
		return Fingerprint{}, artifact.InvalidFingerprint("fingerprint digest must not be empty")
	}
	if len(digestHex) != 64 {
		return Fingerprint{}, artifact.InvalidFingerprint("fingerprint digest has length %d, want 64", len(digestHex))
	}
	for _, r := range digestHex {
		if unicode.IsUpper(r) {
			return Fingerprint{}, artifact.InvalidFingerprint("fingerprint digest must be lowercase hexadecimal")
		}
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return Fingerprint{}, artifact.InvalidFingerprint("fingerprint digest is not hexadecimal")
		}
	}
	decoded, err := hex.DecodeString(digestHex)
	if err != nil || len(decoded) != 32 {
		return Fingerprint{}, artifact.InvalidFingerprint("fingerprint digest is not hexadecimal")
	}
	var digest [32]byte
	copy(digest[:], decoded)
	return Fingerprint{Version: ProjectionVersion, Algorithm: AlgorithmSHA256, Digest: digest}, nil
}

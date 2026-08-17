package diagram

import (
	"crypto/sha256"
	"encoding/hex"
)

type idHasher func(string) string

func stableNodeID(identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return "n_" + hex.EncodeToString(sum[:16])
}

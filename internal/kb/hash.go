package kb

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashOf is the digest the tool records for content it has fetched or captured,
// as `<algorithm>:<digest>`, which is the form every hash field in the format
// takes.
//
// It exists as one function because a recorded digest is only worth something if
// the thing that wrote it and the thing that checks it compute it the same way.
func HashOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

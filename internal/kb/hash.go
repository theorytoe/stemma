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

// ShortHashOf is the head of the same digest, which is what names a file derived
// from a key or a pointer: a captured source, or a fetched document's text.
//
// It carries no algorithm prefix, because it is part of a filename — and it is
// short because it is there to keep two names apart rather than to prove what a
// file holds. Four bytes is enough for the first and no use at all for the
// second, which is what the full digest in the entry is for.
func ShortHashOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:4])
}

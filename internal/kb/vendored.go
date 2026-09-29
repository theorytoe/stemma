package kb

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"path/filepath"
	"strings"
)

// VendoredName is where a key's captured text lives inside the KB.
//
// The name is a function of the key and nothing else, because the entry records
// the capture's hash rather than its path, so the same key always has to resolve
// to the same file. The key is reduced the way a filename is, and the digest
// beside it is what keeps two keys that reduce alike — `smith:2020` and
// `smith-2020` — from claiming one file between them.
func VendoredName(key string) string {
	base := strings.TrimSuffix(fileNameForKey(key), ".bib")
	sum := sha256.Sum256([]byte(key))
	return path.Join(SourcesDir, base+"-"+hex.EncodeToString(sum[:4])+".txt")
}

// VendoredPath is where that file is on disk.
func VendoredPath(root, key string) string {
	return filepath.Join(root, filepath.FromSlash(VendoredName(key)))
}

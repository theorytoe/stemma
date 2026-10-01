package kb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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
	return path.Join(SourcesDir, base+"-"+ShortHashOf([]byte(key))+".txt")
}

// VendoredPath is where that file is on disk.
func VendoredPath(root, key string) string {
	return filepath.Join(root, filepath.FromSlash(VendoredName(key)))
}

// VendoredHashes returns the digest of every capture in sources/, keyed by its
// path inside the KB.
//
// It is deliberately not folded into Hashes. The index stamps pages, and a
// capture is not a page; a running server, on the other hand, has to notice a
// capture changing so the text it serves stays current, and this is how it does.
func VendoredHashes(root string) (map[string]string, error) {
	fsys := os.DirFS(root)
	entries, err := fs.ReadDir(fsys, SourcesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", SourcesDir, err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := path.Join(SourcesDir, e.Name())
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[name] = HashOf(raw)
	}
	return out, nil
}

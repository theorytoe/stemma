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

// VendoredStem is the name a key's captures share, without an extension: the
// extracted text and the original document are one stem with two extensions.
//
// The name is a function of the key and nothing else, because the entry records
// the capture's hash rather than its path, so the same key always has to resolve
// to the same files. The key is reduced the way a filename is, and the digest
// beside it is what keeps two keys that reduce alike — `smith:2020` and
// `smith-2020` — from claiming one name between them.
func VendoredStem(key string) string {
	base := strings.TrimSuffix(fileNameForKey(key), ".bib")
	return path.Join(SourcesDir, base+"-"+ShortHashOf([]byte(key)))
}

// VendoredName is where a key's captured text lives inside the KB.
func VendoredName(key string) string {
	return VendoredStem(key) + ".txt"
}

// VendoredPath is where that file is on disk.
func VendoredPath(root, key string) string {
	return filepath.Join(root, filepath.FromSlash(VendoredName(key)))
}

// OriginalName is where the original document is stored, beside the extracted
// text and under the same stem. The extension is the source's own, so the file
// is served as what it is.
func OriginalName(key, ext string) string {
	return VendoredStem(key) + normalizeCaptureExt(ext)
}

// OriginalPath is where that file is on disk.
func OriginalPath(root, key, ext string) string {
	return filepath.Join(root, filepath.FromSlash(OriginalName(key, ext)))
}

// VendoredArtifact is one file in sources/ belonging to a key.
type VendoredArtifact struct {
	// Name is the path inside the KB, e.g. sources/key-1a2b3c4d.pdf.
	Name string
	// Path is the same file on disk.
	Path string
	// Ext is its extension, lowercased and with the dot.
	Ext string
}

// VendoredFiles returns every file in sources/ belonging to a key. There is
// normally one — the extracted text — and two when an original document was
// captured beside it.
func VendoredFiles(root, key string) ([]VendoredArtifact, error) {
	dir := filepath.Join(root, SourcesDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	prefix := path.Base(VendoredStem(key)) + "."
	var out []VendoredArtifact
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		out = append(out, VendoredArtifact{
			Name: path.Join(SourcesDir, e.Name()),
			Path: filepath.Join(dir, e.Name()),
			Ext:  strings.ToLower(filepath.Ext(e.Name())),
		})
	}
	return out, nil
}

// FindOriginal returns the original document captured for a key.
//
// A capture made before originals were kept has only its extracted text, which
// is a .txt; a source that is itself a text file is the same case. Any other
// extension is an original and wins over the text, because it is the more
// faithful copy.
func FindOriginal(root, key string) (VendoredArtifact, bool) {
	files, err := VendoredFiles(root, key)
	if err != nil {
		return VendoredArtifact{}, false
	}
	var text VendoredArtifact
	for _, f := range files {
		if f.Ext != ".txt" {
			return f, true
		}
		text = f
	}
	return text, text.Name != ""
}

// normalizeCaptureExt turns a file extension into the one stored: lower case,
// with a leading dot, and ".txt" when nothing better is known.
func normalizeCaptureExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ".txt"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
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

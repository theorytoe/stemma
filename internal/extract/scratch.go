package extract

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// ScratchDir is where extracted text is kept for a KB.
//
// It is inside the generated directory because the text is derived from a source
// that is still where it was. It is a cache rather than a copy, so deleting it
// costs nothing but reading the source again, and nothing in it is ever committed
// (`D23`).
func ScratchDir(root string) string {
	return filepath.Join(root, kb.GeneratedDir, "fetched")
}

// TextPath is where one source's text lives.
//
// The name is a function of the pointer and of nothing else, so fetching the same
// source twice lands on the same file and two different sources cannot land on
// one. The slug is there to make a directory listing readable; the digest beside
// it is what actually identifies the file, which is what lets the slug stay lossy.
func TextPath(root, pointer string) string {
	return filepath.Join(ScratchDir(root), slug(pointer)+"-"+kb.ShortHashOf([]byte(pointer))+".txt")
}

// slug reduces a pointer to something recognisable in a directory listing.
//
// A path contributes its base name, because the directories above it are the
// reader's business rather than the source's identity, and a URL contributes its
// host and path, which are.
func slug(pointer string) string {
	readable := pointer
	if !strings.Contains(readable, "://") {
		readable = filepath.Base(readable)
	}
	readable = strings.TrimPrefix(strings.TrimPrefix(readable, "https://"), "http://")
	readable = strings.TrimSuffix(readable, "/")

	var kept []rune
	dash := false
	for _, r := range readable {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_':
			if dash && len(kept) > 0 {
				kept = append(kept, '-')
			}
			dash = false
			kept = append(kept, r)
		case r >= 'A' && r <= 'Z':
			if dash && len(kept) > 0 {
				kept = append(kept, '-')
			}
			dash = false
			kept = append(kept, r+('a'-'A'))
		default:
			// Everything else — a separator, a query, a space, a letter outside
			// ASCII — becomes a dash, and a run of them becomes one dash.
			dash = true
		}
	}
	if len(kept) == 0 {
		return "source"
	}
	// Long enough to read, short enough to type, and the digest is what makes it
	// unique either way.
	if len(kept) > 48 {
		kept = kept[:48]
	}
	// The fallback is decided after the trim rather than before it, because the trim
	// is what can empty a name: a pointer of nothing but dots and dashes reduces to
	// one, and returning it would name the file "-<digest>.txt", which reads as a
	// flag rather than as a file.
	if trimmed := strings.Trim(string(kept), "-."); trimmed != "" {
		return trimmed
	}
	return "source"
}

// Clear empties the scratch area and touches nothing else, and reports how many
// files went. A scratch area that does not exist is already clear.
func Clear(root string) (int, error) {
	if root == "" {
		return 0, errors.New("no KB root to clear scratch in")
	}
	dir := ScratchDir(root)

	files := 0
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files++
		}
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if err := os.RemoveAll(dir); err != nil {
		return 0, err
	}
	return files, nil
}

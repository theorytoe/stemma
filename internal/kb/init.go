package kb

import (
	"fmt"
	"os"
	"path/filepath"
)

// EntryDocument is the page every other page is reachable from.
const EntryDocument = PagesDir + "/index.md"

// Init creates a new KB root: a manifest, an entry document, pages/, inbox/ and
// a bibliography.
//
// It refuses to run over anything that already looks like a KB root. Every file
// it writes is a file it would otherwise be replacing, and the one thing this
// tool must never do is replace a page someone is still using.
func Init(root, title string) error {
	if title == "" {
		title = filepath.Base(absOr(root, root))
	}
	for _, name := range []string{ManifestName, PagesDir, BibliographyName} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if _, err := os.Lstat(full); err == nil {
			return fmt.Errorf("%s already has a %s: %s already looks like a KB root",
				root, name, root)
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	entry, err := NewPage(title, TypeIndex)
	if err != nil {
		return err
	}

	files := []struct {
		name string
		data []byte
	}{
		{ManifestName, manifestFor(title)},
		{EntryDocument, entry.Bytes()},
		{BibliographyName, []byte(bibliographyHeader)},
	}
	for _, f := range files {
		full := filepath.Join(root, filepath.FromSlash(f.name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := WriteFileAtomic(full, f.data); err != nil {
			return err
		}
	}

	// The inbox holds no committed file of its own, so the directory is created
	// rather than implied by one.
	return os.MkdirAll(filepath.Join(root, InboxDir), 0o755)
}

// manifestFor is the manifest written by init. It says what the defaults are
// rather than leaving them to be looked up, because the file is the only place
// a reader will think to look.
func manifestFor(title string) []byte {
	return DefaultManifest(title).TOML()
}

const bibliographyHeader = `% The bibliography for this KB, as BibTeX.
`

// tomlString and tomlList are in manifest.go with the manifest they write.

// absOr returns name if it can be made absolute, and fallback if it cannot.
// It exists so that a KB's default title can be the name of its directory
// without a failure to resolve the path being fatal.
func absOr(name, fallback string) string {
	if abs, err := filepath.Abs(name); err == nil {
		return abs
	}
	return fallback
}

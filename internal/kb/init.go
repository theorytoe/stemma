package kb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	return []byte(fmt.Sprintf(`# The manifest. Optional: a KB with no stemma.toml gets these defaults.
title = %s

# A sentence or two about the KB. Free text; nothing reads it but a person.
description = ""

# Added to the built-in types: topic, concept and note.
types = []

# A built-in formatter. Not a CSL style.
citation_style = %s

# The export family's defaults. Depth is hops; 0 means no limit.
[export]
default_depth = %d
`, tomlString(title), tomlString(DefaultCitationStyle), DefaultExportDepth))
}

const bibliographyHeader = `% The bibliography for this KB, as BibTeX.
`

// tomlString quotes a value the way TOML's basic strings are written. Go's own
// quoting is close but not the same, and a title with a quote or a backslash in
// it has to survive the round trip through a file the tool then reads back.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// absOr returns name if it can be made absolute, and fallback if it cannot.
// It exists so that a KB's default title can be the name of its directory
// without a failure to resolve the path being fatal.
func absOr(name, fallback string) string {
	if abs, err := filepath.Abs(name); err == nil {
		return abs
	}
	return fallback
}

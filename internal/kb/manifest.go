package kb

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Names the format gives to things inside a KB root.
const (
	ManifestName     = "stemma.toml"
	PagesDir         = "pages"
	InboxDir         = "inbox"
	SourcesDir       = "sources"
	BibliographyName = "bibliography.bib"
	BibliographyDir  = "bibliography"
	GeneratedDir     = ".stemma"
)

// DefaultCitationStyle is used when a manifest does not name one.
const DefaultCitationStyle = "author-date"

// DefaultExportDepth is how many hops a scoped extract reaches when neither the
// command line nor the manifest says otherwise (D29).
const DefaultExportDepth = 1

// Export is the manifest's [export] table: defaults for the export family.
type Export struct {
	// DefaultDepth is the depth `export page` uses when --depth is not given.
	DefaultDepth int
}

// Manifest is a KB's stemma.toml.
//
// It is optional: a KB with no manifest is fully usable and gets the defaults
// below. Where it exists it is the discovery marker, and anything it says wins
// over a default.
type Manifest struct {
	// Title is the KB's title. It defaults to the name of the KB root.
	Title string

	// Description is a sentence or two about the KB. Optional, and read by
	// nothing but a person.
	Description string

	// DefaultType is what `new` applies when an author does not choose.
	DefaultType string

	// Types are added to the built-in vocabulary.
	Types []string

	// CitationStyle names a built-in formatter, never a CSL style.
	CitationStyle string

	// Ignore lists paths, relative to the KB root, that are outside the KB.
	Ignore []string

	// Export holds the defaults for the export family.
	Export Export

	// Unknown holds the keys this version of the tool does not read. They are
	// recorded so that a writer knows what it must not drop.
	Unknown []string

	// raw is the file exactly as it was read. The manifest is never re-emitted,
	// so a key this version does not understand survives untouched.
	raw []byte
}

// DefaultManifest is what a KB with no manifest is assumed to have said.
func DefaultManifest(rootName string) Manifest {
	return Manifest{
		Title:         rootName,
		DefaultType:   DefaultType,
		CitationStyle: DefaultCitationStyle,
		Export:        Export{DefaultDepth: DefaultExportDepth},
	}
}

// manifestKeys is the shape toml is decoded into. It is separate from Manifest
// so that the exported type carries the defaults and the raw bytes, and this
// one carries only what the file can say.
type manifestKeys struct {
	Title         string     `toml:"title"`
	Description   string     `toml:"description"`
	DefaultType   string     `toml:"default_type"`
	Types         []string   `toml:"types"`
	CitationStyle string     `toml:"citation_style"`
	Ignore        []string   `toml:"ignore"`
	Export        exportKeys `toml:"export"`
}

// exportKeys is the shape of the [export] table.
type exportKeys struct {
	DefaultDepth int `toml:"default_depth"`
}

// ParseManifest reads a manifest, falling back to the defaults for everything
// it does not say.
//
// It fails only on TOML that does not parse. A manifest naming something the
// tool does not know is not an error: the key is recorded in Unknown and kept,
// because refusing to read a file over a key it could have ignored would be the
// opposite of lenient.
func ParseManifest(rootName string, raw []byte) (Manifest, error) {
	var keys manifestKeys
	md, err := toml.Decode(string(raw), &keys)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", ManifestName, err)
	}

	m := DefaultManifest(rootName)
	m.raw = raw
	if keys.Title != "" {
		m.Title = keys.Title
	}
	if keys.Description != "" {
		m.Description = keys.Description
	}
	if keys.DefaultType != "" {
		m.DefaultType = keys.DefaultType
	}
	if keys.CitationStyle != "" {
		m.CitationStyle = keys.CitationStyle
	}
	if keys.Export.DefaultDepth > 0 {
		m.Export.DefaultDepth = keys.Export.DefaultDepth
	}
	m.Types = keys.Types
	m.Ignore = keys.Ignore
	for _, key := range md.Undecoded() {
		m.Unknown = append(m.Unknown, key.String())
	}
	sort.Strings(m.Unknown)
	return m, nil
}

// Bytes returns the manifest as it was read, so that a writer can splice into
// it rather than re-emit it.
func (m Manifest) Bytes() []byte { return m.raw }

// Vocabulary is the set of types this manifest accepts.
func (m Manifest) Vocabulary() Vocabulary {
	return DefaultVocabulary().Extend(m.Types...)
}

// Ignores reports whether a path relative to the KB root is outside the KB.
func (m Manifest) Ignores(name string) bool {
	for _, pattern := range m.Ignore {
		if matchGlob(pattern, name) {
			return true
		}
	}
	return false
}

// matchGlob reports whether a KB-root-relative path matches an ignore pattern.
//
// Patterns use forward slashes. "*" matches within one segment and "**" matches
// across segments, so "pages/attic/**" covers everything under that directory
// and "pages/attic" covers only that directory itself.
func matchGlob(pattern, name string) bool {
	// An empty pattern is a mistake in a manifest, not a rule that matches
	// everything.
	if pattern == "" {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pattern, name []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			// "**" is the rest of the path, including none of it.
			if len(pattern) == 1 {
				return true
			}
			for i := 0; i <= len(name); i++ {
				if matchSegments(pattern[1:], name[i:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		ok, err := path.Match(pattern[0], name[0])
		if err != nil || !ok {
			return false
		}
		pattern, name = pattern[1:], name[1:]
	}
	return len(name) == 0
}

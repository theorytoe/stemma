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
const DefaultCitationStyle = "numeric"

// DefaultExportDepth is how many hops a scoped extract reaches when neither the
// command line nor the manifest says otherwise (D29). A manifest may set
// [export] default_depth = 0 to ask for no limit at all.
const DefaultExportDepth = 1

// Export is the manifest's [export] table: defaults for the export family.
type Export struct {
	// DefaultDepth is the depth `export page` uses when --depth is not given.
	// It is a hop count, and 0 means no limit: the whole reachable set.
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

// TOML renders a manifest as the file form of a KB that is being made: the one
// `init` writes, and the one an extract gets.
//
// It is for a manifest that does not exist yet, and never for rewriting one that
// does. A manifest that has been read is not re-emitted, which is what preserves
// the keys this version does not know (`D26`, `P4`); rendering a new one from
// the values in hand says what those values are, in the only place a reader will
// think to look.
func (m Manifest) TOML() []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# The manifest. Optional: a KB with no %s gets these defaults.\n", ManifestName)
	fmt.Fprintf(&b, "title = %s\n\n", tomlString(m.Title))
	b.WriteString("# A sentence or two about the KB. Free text; nothing reads it but a person.\n")
	fmt.Fprintf(&b, "description = %s\n\n", tomlString(m.Description))
	b.WriteString("# Added to the built-in types: topic, concept and note.\n")
	fmt.Fprintf(&b, "types = %s\n\n", tomlList(m.Types))
	b.WriteString("# What `new` applies when a page does not choose a type.\n")
	fmt.Fprintf(&b, "default_type = %s\n\n", tomlString(m.DefaultType))
	b.WriteString("# A built-in formatter. Not a CSL style.\n")
	fmt.Fprintf(&b, "citation_style = %s\n\n", tomlString(m.CitationStyle))
	if len(m.Ignore) > 0 {
		b.WriteString("# Paths outside the KB, relative to the KB root. * is one segment, ** crosses them.\n")
		fmt.Fprintf(&b, "ignore = %s\n\n", tomlList(m.Ignore))
	}
	b.WriteString("# The export family's defaults. Depth is hops; 0 means no limit.\n")
	b.WriteString("[export]\n")
	fmt.Fprintf(&b, "default_depth = %d\n", m.Export.DefaultDepth)
	return []byte(b.String())
}

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

// tomlList writes an array of TOML basic strings.
func tomlList(items []string) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, tomlString(s))
	}
	return "[" + strings.Join(parts, ", ") + "]"
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
	// default_depth is overridden only when the manifest actually says it, so
	// that an explicit 0 can mean "no limit" rather than being read as absent.
	if md.IsDefined("export", "default_depth") {
		if keys.Export.DefaultDepth < 0 {
			return Manifest{}, fmt.Errorf("%s: export.default_depth is %d; it is a hop count of zero or more, and zero means no limit",
				ManifestName, keys.Export.DefaultDepth)
		}
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

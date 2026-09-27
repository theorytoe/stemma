package kb

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// Finding codes for the bibliography.
const (
	CodeCitationMissing   = "citation-missing"
	CodeCitationUncited   = "citation-uncited"
	CodeCitationDuplicate = "citation-duplicate"
)

// Citation is one citation key used in a page body.
//
// Only the key is kept. Which form the citation took and any locator it carried
// matter to the renderer and not to lint, so a renderer will need more than
// this type has. Nothing else in the format needs it, so nothing else pays for
// it.
type Citation struct {
	Key  string
	Line int
}

// Citations returns every citation key in the page body, in the order it
// appears, along with the line each is on.
//
// It reads the same prose the link scanner reads, so a citation written inside
// a code span or a fenced block is not a citation, for the same reason a link
// written there is not a link.
func (p *Page) Citations() []Citation {
	var out []Citation
	for _, pl := range p.proseLines() {
		for _, run := range proseRuns(pl.text) {
			for _, key := range findCitationKeys(run) {
				out = append(out, Citation{Key: key, Line: pl.line})
			}
		}
	}
	return out
}

// findCitationKeys returns the citation keys in one run of prose.
//
// All five of the inline forms carry a key in the same shape -- [@key],
// [@a; @b], [@key, p. 33], @key and [-@key] -- so they are found by looking for
// the key rather than by parsing the brackets around it. That is also why this
// stays small: the differences between the forms are the renderer's problem,
// and lint only ever asks whether a key exists.
func findCitationKeys(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '@' {
			continue
		}
		// An "@" with a word character before it is part of an address, not a
		// citation. "user@example.com" names no source.
		if i > 0 && isAddressByte(s[i-1]) {
			continue
		}
		j := i + 1
		if j >= len(s) || !isKeyStart(s[j]) {
			continue
		}
		for j < len(s) && isKeyByte(s[j]) {
			j++
		}
		// A key may contain "." but a sentence ends with one far more often
		// than a key does.
		key := strings.TrimRight(s[i+1:j], ".")
		if key == "" {
			continue
		}
		out = append(out, key)
		i = j - 1
	}
	return out
}

// isAddressByte reports whether b can be the last character of a word before an
// "@", which is what tells an address from a citation.
func isAddressByte(b byte) bool {
	return isKeyStart(b) || b == '.'
}

func isKeyStart(b byte) bool {
	return b == '_' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

// isKeyByte covers the characters a citation key may contain, which is the set
// pandoc documents: alphanumerics and "_ : . # $ % & - + ? < > ~ /".
func isKeyByte(b byte) bool {
	if isKeyStart(b) {
		return true
	}
	return strings.IndexByte(":.#$%&-+?<>~/", b) >= 0
}

// Bibliography is the set of citation keys a KB defines, and which file defined
// each one. A KB may keep its bibliography in one file or in a directory of
// them, so a key has to remember where it came from for a finding to point at
// the file a reader would have to open.
type Bibliography struct {
	keys  map[string]bool
	order []string

	// counts is how many times a key is defined, and from is the first file that
	// defined it. Two files defining one key is a duplicate, exactly as two
	// entries in one file would be.
	counts map[string]int
	from   map[string]string
}

// NewBibliography returns an empty bibliography.
func NewBibliography() *Bibliography {
	return &Bibliography{
		keys:   map[string]bool{},
		counts: map[string]int{},
		from:   map[string]string{},
	}
}

// ParseBibliography reads a BibTeX file far enough to know which keys it
// defines.
//
// This is deliberately not a BibTeX parser. The format's source of truth for
// sources is BibTeX, but lint's only question about it is whether a key exists,
// and answering that needs entry headers and nothing else. Fields, values,
// macros and everything else are the bibliography subsystem's business.
//
// name is the path the file was read from, recorded against each key it defines
// so that a finding can name the file rather than the KB.
//
// It fails on bytes that are not UTF-8 and on an entry that is never closed. In
// both, the tool cannot tell where one entry ends and the next begins, so the
// keys after that point cannot be trusted.
func ParseBibliography(name string, raw []byte) (*Bibliography, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("bibliography is not valid UTF-8")
	}
	b := NewBibliography()
	for i := 0; ; {
		at := bytes.IndexByte(raw[i:], '@')
		if at < 0 {
			return b, nil
		}
		i += at
		start := i

		i++
		for i < len(raw) && isASCIILetter(raw[i]) {
			i++
		}
		entryType := strings.ToLower(string(raw[start+1 : i]))
		for i < len(raw) && isBibSpace(raw[i]) {
			i++
		}
		if i >= len(raw) || (raw[i] != '{' && raw[i] != '(') {
			// An "@" that does not open an entry, such as one inside a field
			// written as unquoted prose.
			i = start + 1
			continue
		}

		open := raw[i]
		closing := byte('}')
		if open == '(' {
			closing = ')'
		}
		body := i + 1
		end := entryEnd(raw, body, open, closing)
		if end < 0 {
			return nil, fmt.Errorf("the @%s entry beginning on line %d is never closed",
				entryType, lineOf(raw, start))
		}
		if !isNotAnEntry(entryType) {
			if key := entryKey(raw[body:end]); key != "" {
				b.counts[key]++
				if !b.keys[key] {
					b.keys[key] = true
					b.order = append(b.order, key)
					b.from[key] = name
				}
			}
		}
		i = end + 1
	}
}

// isNotAnEntry reports whether an entry type is one of the three that carry no
// citation key. @string defines a macro, @preamble injects LaTeX and @comment
// is ignored outright; the name after each is not a key anything can cite.
func isNotAnEntry(entryType string) bool {
	switch entryType {
	case "string", "preamble", "comment":
		return true
	}
	return false
}

// entryEnd returns the offset of the delimiter that closes an entry, or -1.
//
// Quoted values are stepped over rather than counted, because a brace inside
// one is part of a value and not part of the structure around it.
func entryEnd(raw []byte, from int, open, closing byte) int {
	depth := 1
	for i := from; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			i++ // an escaped character is never structural
		case '"':
			i++
			for i < len(raw) && raw[i] != '"' {
				if raw[i] == '\\' {
					i++
				}
				i++
			}
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// entryKey returns the citation key at the start of an entry body.
func entryKey(body []byte) string {
	i := 0
	for i < len(body) && isBibSpace(body[i]) {
		i++
	}
	start := i
	for i < len(body) && body[i] != ',' && body[i] != '}' && body[i] != ')' && !isBibSpace(body[i]) {
		i++
	}
	return string(body[start:i])
}

// Has reports whether the bibliography defines a key.
//
// Keys are matched exactly, including case. A citation key is an identifier
// rather than prose, so unlike a page title it is not folded for comparison.
func (b *Bibliography) Has(key string) bool { return b.keys[key] }

// Keys returns every key the bibliography defines, in the order first seen.
func (b *Bibliography) Keys() []string { return append([]string(nil), b.order...) }

// PathOf returns the file a key was first defined in.
func (b *Bibliography) PathOf(key string) string { return b.from[key] }

// Duplicates returns the keys defined more than once, sorted.
func (b *Bibliography) Duplicates() []string {
	var out []string
	for key, n := range b.counts {
		if n > 1 {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// Len returns the number of distinct keys.
func (b *Bibliography) Len() int { return len(b.keys) }

// Add merges another bibliography into this one, keeping the first file that
// defined each key.
func (b *Bibliography) Add(other *Bibliography) {
	for _, key := range other.order {
		b.counts[key] += other.counts[key]
		if !b.keys[key] {
			b.keys[key] = true
			b.order = append(b.order, key)
			b.from[key] = other.from[key]
		}
	}
}

// CitationFindings reports what is wrong between the pages and the
// bibliography: keys cited that it does not define, keys it defines that
// nothing cites, and keys it defines twice.
//
// A finding about the bibliography itself is attributed to the file that
// actually defines the key, because a KB may keep its bibliography in a
// directory and "the bibliography" is then not one place.
func (g *Graph) CitationFindings(b *Bibliography, mode Mode) []Finding {
	soft := Warning
	if mode == Strict {
		soft = Error
	}
	var out []Finding

	for _, path := range g.Paths() {
		for _, c := range g.citations[path] {
			if !b.Has(c.Key) {
				out = append(out, Finding{
					Severity: soft,
					Code:     CodeCitationMissing,
					Path:     path,
					Line:     c.Line,
					Message:  "the bibliography has no entry for " + quote(c.Key),
				})
			}
		}
	}

	for _, key := range b.Keys() {
		if len(g.cited[key]) == 0 {
			out = append(out, Finding{
				Severity: soft,
				Code:     CodeCitationUncited,
				Path:     b.PathOf(key),
				Message:  quote(key) + " is never cited",
			})
		}
	}

	for _, key := range b.Duplicates() {
		out = append(out, Finding{
			Severity: soft,
			Code:     CodeCitationDuplicate,
			Path:     b.PathOf(key),
			Message:  quote(key) + " is defined more than once",
		})
	}

	sortFindings(out)
	return out
}

func isASCIILetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isBibSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func lineOf(raw []byte, offset int) int {
	return bytes.Count(raw[:offset], []byte("\n")) + 1
}

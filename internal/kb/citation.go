package kb

import (
	"sort"
	"strings"
)

// Finding codes for the bibliography.
const (
	CodeCitationMissing   = "citation-missing"
	CodeCitationUncited   = "citation-uncited"
	CodeCitationDuplicate = "citation-duplicate"
)

// Citation is one key cited in a page body, with the form it was written in.
//
// The form matters to the renderer and not to lint, but it is parsed here
// rather than there: reading the same prose twice to learn two things about one
// citation would be the only reason the renderer needed to know the syntax at
// all. Nothing that only asks whether a key exists has to look past Key.
type Citation struct {
	// Key is the BibTeX citation key, exactly as written.
	Key string
	// Line is the body line the citation is on.
	Line int

	// Group is the citation group the key belongs to, in the order groups
	// appear. A key written bare as "@key" is a group of one.
	Group int
	// Position is the key's index within its group.
	Position int

	// Prefix is the text before the key that belongs to it, such as "see".
	Prefix string
	// Locator is what follows the key's comma, such as "p. 33".
	Locator string
	// SuppressAuthor is the "-" before "@", which hides the author's name.
	SuppressAuthor bool
	// Narrative is a citation written outside brackets, where the author is the
	// subject of the sentence rather than a parenthetical aside.
	Narrative bool
}

// Citations returns every citation in the page body, in the order it appears,
// along with the line each is on and the form it was written in.
//
// It reads the same prose the link scanner reads, so a citation written inside
// a code span or a fenced block is not a citation, for the same reason a link
// written there is not a link.
func (p *Page) Citations() []Citation {
	var out []Citation
	base := 0
	for _, pl := range p.proseLines() {
		for _, run := range proseRuns(pl.text) {
			cites, groups := parseCitations(run)
			for _, c := range cites {
				c.Line = pl.line
				c.Group += base
				out = append(out, c)
			}
			base += groups
		}
	}
	return out
}

// parseCitations reads one run of prose, returning the citations in it and how
// many groups they formed. Groups are numbered from zero within the run, so the
// caller can offset them to number a whole page.
//
// A citation is either a bracketed group, which may hold several keys separated
// by ";", or a bare "@key" in the prose. Brackets that hold no key are not a
// citation at all, which is what keeps "[user@example.com]" from being read as
// one.
func parseCitations(s string) ([]Citation, int) {
	var out []Citation
	groups := 0
	for i := 0; i < len(s); {
		switch {
		case s[i] == '[':
			close := strings.IndexByte(s[i+1:], ']')
			if close < 0 {
				i++
				continue
			}
			end := i + 1 + close
			items := parseGroup(s[i+1 : end])
			if len(items) == 0 {
				i++
				continue
			}
			for j := range items {
				items[j].Group = groups
				items[j].Position = j
			}
			out = append(out, items...)
			groups++
			i = end + 1

		case s[i] == '@':
			// An "@" with a word character before it is part of an address, not
			// a citation. "user@example.com" names no source.
			if i > 0 && isAddressByte(s[i-1]) {
				i++
				continue
			}
			c, n := parseBare(s[i:])
			if n == 0 {
				i++
				continue
			}
			c.Group = groups
			c.Position = 0
			out = append(out, c)
			groups++
			i += n

		default:
			i++
		}
	}
	return out, groups
}

// parseGroup reads the inside of a bracketed citation, which is one or more
// ";"-separated items.
func parseGroup(body string) []Citation {
	var out []Citation
	for _, seg := range strings.Split(body, ";") {
		if c, ok := parseItem(seg); ok {
			out = append(out, c)
		}
	}
	return out
}

// parseItem reads one item of a bracketed group: an optional prefix, an optional
// "-", the key, and an optional locator after a comma.
func parseItem(seg string) (Citation, bool) {
	at := indexOfKeyAt(seg)
	if at < 0 {
		return Citation{}, false
	}
	prefix := strings.TrimSpace(seg[:at])
	suppress := false
	if strings.HasSuffix(prefix, "-") {
		suppress = true
		prefix = strings.TrimSpace(strings.TrimSuffix(prefix, "-"))
	}
	key, rest := scanCitationKey(seg[at+1:])
	if key == "" {
		return Citation{}, false
	}
	c := Citation{Key: key, Prefix: prefix, SuppressAuthor: suppress}
	if r := strings.TrimSpace(rest); strings.HasPrefix(r, ",") {
		c.Locator = strings.TrimSpace(r[1:])
	}
	return c, true
}

// parseBare reads a narrative "@key" and returns how much of the string it
// consumed. A locator on a narrative citation is written in its own brackets,
// so only the key itself is consumed here.
func parseBare(s string) (Citation, int) {
	if len(s) < 2 || s[0] != '@' || !isKeyStart(s[1]) {
		return Citation{}, 0
	}
	i := 1
	for i < len(s) && isKeyByte(s[i]) {
		i++
	}
	key := strings.TrimRight(s[1:i], ".")
	if key == "" {
		return Citation{}, 0
	}
	return Citation{Key: key, Narrative: true}, i
}

// indexOfKeyAt finds the "@" that begins a key, skipping one that is part of an
// address.
func indexOfKeyAt(seg string) int {
	for i := 0; i < len(seg); i++ {
		if seg[i] != '@' {
			continue
		}
		if i+1 >= len(seg) || !isKeyStart(seg[i+1]) {
			continue
		}
		if i > 0 && isAddressByte(seg[i-1]) {
			continue
		}
		return i
	}
	return -1
}

// scanCitationKey reads a key and returns it with whatever followed it. A key
// may contain "." but a sentence ends with one far more often than a key does,
// so a trailing run of them is not part of the key.
func scanCitationKey(s string) (key, rest string) {
	i := 0
	if i >= len(s) || !isKeyStart(s[i]) {
		return "", s
	}
	for i < len(s) && isKeyByte(s[i]) {
		i++
	}
	return strings.TrimRight(s[:i], "."), s[i:]
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

	// entries is the record each key defines -- the first one, when a key is
	// defined more than once. Keeping it here rather than in a parallel index
	// is what lets a citation resolve to an entry and a lint finding point at
	// the same key without the two being able to disagree.
	entries map[string]*BibEntry

	// counts is how many times a key is defined, and from is the first file that
	// defined it. Two files defining one key is a duplicate, exactly as two
	// entries in one file would be.
	counts map[string]int
	from   map[string]string
}

// NewBibliography returns an empty bibliography.
func NewBibliography() *Bibliography {
	return &Bibliography{
		keys:    map[string]bool{},
		entries: map[string]*BibEntry{},
		counts:  map[string]int{},
		from:    map[string]string{},
	}
}

// ParseBibliography reads a BibTeX file far enough to know which keys it
// defines.
//
// It parses the whole file, but lint's only question about a bibliography is
// whether a key exists, so only the entries are kept here and the rest is left
// to whoever asks for it through ParseBibFile.
//
// name is the path the file was read from, recorded against each key it defines
// so that a finding can name the file rather than the KB.
func ParseBibliography(name string, raw []byte) (*Bibliography, error) {
	f, err := ParseBibFile(name, raw)
	if err != nil {
		return nil, err
	}
	b := NewBibliography()
	for _, e := range f.Entries() {
		key := e.Key()
		if key == "" {
			continue
		}
		b.counts[key]++
		if !b.keys[key] {
			b.keys[key] = true
			b.order = append(b.order, key)
			b.from[key] = name
			b.entries[key] = e
		}
	}
	return b, nil
}

// Has reports whether the bibliography defines a key.
//
// Keys are matched exactly, including case. A citation key is an identifier
// rather than prose, so unlike a page title it is not folded for comparison.
func (b *Bibliography) Has(key string) bool { return b.keys[key] }

// Keys returns every key the bibliography defines, in the order first seen.
func (b *Bibliography) Keys() []string { return append([]string(nil), b.order...) }

// SortedKeys returns every key, sorted. A listing wants an order that is a
// property of the data rather than of the order the files were read in.
func (b *Bibliography) SortedKeys() []string {
	out := append([]string(nil), b.order...)
	sort.Strings(out)
	return out
}

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
			if e, ok := other.entries[key]; ok {
				b.entries[key] = e
			}
		}
	}
}

// Entry returns the record a key defines, and whether there is one.
func (b *Bibliography) Entry(key string) (*BibEntry, bool) {
	e, ok := b.entries[key]
	return e, ok
}

// Entries returns every record, in key order. An export wants a listing's order
// rather than the order the files happened to be read in.
func (b *Bibliography) Entries() []*BibEntry {
	out := make([]*BibEntry, 0, len(b.order))
	for _, key := range b.SortedKeys() {
		if e, ok := b.entries[key]; ok {
			out = append(out, e)
		}
	}
	return out
}

// Reference is one source in a page's reference list: the record, and the
// citation that first put it there.
type Reference struct {
	Citation Citation
	Entry    *BibEntry
}

// References returns the sources a page cites, in the order of the page's first
// citation of each, and the citations whose keys nothing defines.
//
// Both are deduplicated by key. A work cited three times is one entry in the
// reference list, and a key nothing defines is reported once however many times
// it was written. A key that cannot be resolved is not dropped: the caller needs
// to know the list is short.
func (g *Graph) References(path string, b *Bibliography) (refs []Reference, missing []Citation) {
	seen := map[string]bool{}
	for _, c := range g.Citations(path) {
		if seen[c.Key] {
			continue
		}
		seen[c.Key] = true
		if b != nil {
			if e, ok := b.Entry(c.Key); ok {
				refs = append(refs, Reference{Citation: c, Entry: e})
				continue
			}
		}
		missing = append(missing, c)
	}
	return refs, missing
}

// References returns the sources one page cites, resolved against the KB's
// bibliography. It is the shape every reader wants: the page's reference list,
// and the keys that could not join it.
func (k *KB) References(path string) ([]Reference, []Citation) {
	return k.Graph.References(path, k.Bibliography)
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

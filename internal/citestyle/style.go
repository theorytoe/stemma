// Package citestyle turns a citation and a bibliography entry into the text a
// reader sees.
//
// It is deliberately small. The common failure of a bibliography format is
// growing a partial CSL implementation by accident, so this package offers two
// named styles and nothing else: no style language, no locale, no field
// mapping, no processor. A KB that needs a style this package does not have
// needs a different tool, and saying so is more honest than half-implementing
// CSL.
package citestyle

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// Style names a built-in formatter. A name is never a CSL style identifier.
type Style string

const (
	// AuthorDate writes "(Bush 1945)" and "Bush, Vannevar. 1945. As We May
	// Think. The Atlantic Monthly."
	AuthorDate Style = "author-date"
	// Numeric writes "[1]" and "[1] Bush, Vannevar. As We May Think. The
	// Atlantic Monthly. 1945.", numbering by order of first citation.
	Numeric Style = "numeric"
)

// Names returns the styles this build has, in the order they are documented.
func Names() []string { return []string{string(AuthorDate), string(Numeric)} }

// Piece is one citation's rendered text and the key it came from, so that a
// host can wrap each key's text -- with a link to its source page, say --
// without the formatter knowing anything about HTML.
type Piece struct {
	Key  string
	Text string
}

// Layout is a citation group kept in pieces: what stands before the items, the
// items in order, what separates them, and what stands after. It is the same
// rendering as Cite, only not yet joined, which is what lets a host that links
// each source wrap the pieces individually.
type Layout struct {
	Before string
	Pieces []Piece
	Sep    string
	After  string
}

// Text is the group as one string, which is exactly what Cite returns.
func (l Layout) Text() string {
	parts := make([]string, len(l.Pieces))
	for i, p := range l.Pieces {
		parts[i] = p.Text
	}
	return l.Before + strings.Join(parts, l.Sep) + l.After
}

// Formatter renders a page's citations and its reference list in one style.
type Formatter interface {
	// Cite renders the citations that share one group, in order. A single
	// narrative citation is a group of one.
	Cite(group []kb.Citation, refs []kb.Reference) string

	// Layout renders the same group, kept in pieces. A host that renders plain
	// text uses Cite; one that links each key to its source page uses this and
	// wraps the pieces.
	Layout(group []kb.Citation, refs []kb.Reference) Layout

	// Entry renders one reference-list entry. number is its 1-based place in
	// the list, which a numeric style uses and the others ignore.
	Entry(r kb.Reference, number int) string
}

// Parse returns the formatter a manifest's citation_style names. An empty name
// is the default style, which is what a KB with no manifest has.
//
// An unknown name is an error and not a fallback: rendering in a style nobody
// asked for is worse than refusing, because the output looks right.
func Parse(name string) (Formatter, error) {
	switch Style(strings.TrimSpace(name)) {
	case "", Numeric:
		return numeric{}, nil
	case AuthorDate:
		return authorDate{}, nil
	}
	return nil, fmt.Errorf("unknown citation style %q; the styles this build has are %s",
		name, strings.Join(Names(), ", "))
}

// Groups splits a page's citations into the groups they were written in, in
// order. Consecutive citations sharing a group number form one group, which is
// what makes "[@a; @b]" render as one parenthetical rather than two.
func Groups(citations []kb.Citation) [][]kb.Citation {
	var out [][]kb.Citation
	for _, c := range citations {
		if n := len(out); n > 0 && out[n-1][0].Group == c.Group {
			out[n-1] = append(out[n-1], c)
			continue
		}
		out = append(out, []kb.Citation{c})
	}
	return out
}

// entryFor finds the entry a citation key resolved to. A key that resolved to
// nothing has no entry here, which is why rendering falls back to the key.
func entryFor(key string, refs []kb.Reference) (*kb.BibEntry, bool) {
	for _, r := range refs {
		if r.Entry.Key() == key {
			return r.Entry, true
		}
	}
	return nil, false
}

// authorDate is the author-date formatter.
type authorDate struct{}

func (authorDate) Cite(group []kb.Citation, refs []kb.Reference) string {
	return authorDate{}.Layout(group, refs).Text()
}

func (authorDate) Layout(group []kb.Citation, refs []kb.Reference) Layout {
	if len(group) == 1 && group[0].Narrative {
		key := group[0].Key
		e, ok := entryFor(key, refs)
		if !ok {
			return Layout{Pieces: []Piece{{Key: key, Text: key}}}
		}
		label := authorLabel(e)
		if label == "" {
			return Layout{Pieces: []Piece{{Key: key, Text: year(e)}}}
		}
		return Layout{Pieces: []Piece{{Key: key, Text: label + " (" + year(e) + ")"}}}
	}

	pieces := make([]Piece, 0, len(group))
	for _, c := range group {
		pieces = append(pieces, Piece{Key: c.Key, Text: authorDateItem(c, refs)})
	}
	return Layout{Before: "(", Pieces: pieces, Sep: "; ", After: ")"}
}

func authorDateItem(c kb.Citation, refs []kb.Reference) string {
	e, ok := entryFor(c.Key, refs)
	if !ok {
		return c.Key
	}
	label := strings.TrimSpace(authorLabel(e) + " " + year(e))
	if c.SuppressAuthor {
		label = year(e)
	}
	if c.Locator != "" {
		label += ", " + c.Locator
	}
	if c.Prefix != "" {
		label = c.Prefix + " " + label
	}
	return label
}

func (authorDate) Entry(r kb.Reference, _ int) string {
	return joinSentences(
		authorList(r.Entry),
		year(r.Entry),
		plain(r.Entry.Title()),
		location(r.Entry),
		link(r.Entry),
	)
}

// numeric is the numbered formatter. Its number is a key's place in the page's
// reference list, which is in order of first citation.
type numeric struct{}

func (numeric) Cite(group []kb.Citation, refs []kb.Reference) string {
	return numeric{}.Layout(group, refs).Text()
}

func (numeric) Layout(group []kb.Citation, refs []kb.Reference) Layout {
	pieces := make([]Piece, 0, len(group))
	for _, c := range group {
		label := number(c.Key, refs)
		if c.Locator != "" {
			label += ", " + c.Locator
		}
		if c.Prefix != "" {
			label = c.Prefix + " " + label
		}
		pieces = append(pieces, Piece{Key: c.Key, Text: label})
	}
	return Layout{Before: "[", Pieces: pieces, Sep: "; ", After: "]"}
}

func (numeric) Entry(r kb.Reference, number int) string {
	return "[" + strconv.Itoa(number) + "] " + joinSentences(
		authorList(r.Entry),
		plain(r.Entry.Title()),
		location(r.Entry),
		year(r.Entry),
		link(r.Entry),
	)
}

func number(key string, refs []kb.Reference) string {
	for i, r := range refs {
		if r.Entry.Key() == key {
			return strconv.Itoa(i + 1)
		}
	}
	return key
}

// authorLabel is the short form an in-text citation uses: one surname, two
// joined by "and", and three or more collapsed to "et al.".
func authorLabel(e *kb.BibEntry) string {
	authors := e.Authors()
	switch len(authors) {
	case 0:
		return ""
	case 1:
		return surname(authors[0])
	case 2:
		return surname(authors[0]) + " and " + surname(authors[1])
	default:
		return surname(authors[0]) + " et al."
	}
}

// authorList is the reference list's author field, every author written
// "Family, Given".
func authorList(e *kb.BibEntry) string {
	authors := e.Authors()
	if len(authors) == 0 {
		return ""
	}
	parts := make([]string, len(authors))
	for i, a := range authors {
		parts[i] = formatName(a)
	}
	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
	}
}

// formatName writes a name as "Family, Given". A name already in that form is
// left alone; a name written "Given Family" is turned around.
func formatName(author string) string {
	n, ok := kb.SplitName(author)
	if !ok {
		return plain(author)
	}
	return n.Plain()
}

// surname is the family name alone, which is what an in-text label shows.
func surname(author string) string {
	return plain(kb.Surname(author))
}

// year is the record's year, or the abbreviation for one it does not have.
func year(e *kb.BibEntry) string {
	if y := e.Year(); y != "" {
		return y
	}
	return "n.d."
}

// location is the container, volume, issue and pages of a record, as much of
// them as it has.
func location(e *kb.BibEntry) string {
	container := plain(firstValue(e, "journal", "booktitle", "publisher"))
	volume := plain(firstValue(e, "volume"))
	issue := plain(firstValue(e, "issue", "number"))
	pages := plain(firstValue(e, "pages"))

	var b strings.Builder
	b.WriteString(container)
	if volume != "" {
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		b.WriteString(volume)
		if issue != "" {
			b.WriteString("(" + issue + ")")
		}
	}
	if pages != "" {
		if b.Len() > 0 {
			b.WriteString(": ")
		}
		b.WriteString(pages)
	}
	return b.String()
}

// link is where a record can be found: its DOI as a URL when it has one, and
// otherwise its URL.
func link(e *kb.BibEntry) string {
	if doi := strings.TrimSpace(firstValue(e, "doi")); doi != "" {
		return "https://doi.org/" + doi
	}
	return strings.TrimSpace(firstValue(e, "url"))
}

func firstValue(e *kb.BibEntry, names ...string) string {
	for _, name := range names {
		if v, ok := e.Value(name); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// joinSentences joins non-empty pieces into one string, each closed with a
// single full stop. It is what keeps a missing publisher from leaving a stray
// ".." in the middle of a reference.
func joinSentences(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, strings.TrimSuffix(p, ".")+".")
	}
	return strings.Join(out, " ")
}

// plain removes the braces a value uses to protect capitalisation, because they
// are markup and not part of what a reader sees.
func plain(s string) string {
	return strings.TrimSpace(kb.StripBraces(s))
}

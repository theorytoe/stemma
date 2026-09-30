package kb

import (
	"sort"
	"strings"
)

// Inline is one inline construct in a page body: a wikilink or a citation
// group, together with the byte range the construct occupies.
//
// Start and End are offsets into Body(), covering the construct's own brackets.
// Exactly one of Link and Cites is set. A citation group is one construct
// however many keys it holds, because "[@a; @b]" is one parenthetical and has
// to be read and rewritten as one unit.
type Inline struct {
	Start, End int
	Link       *Link
	Cites      []Citation
}

// Inlines returns every inline construct in the page body, in the order it
// appears.
//
// It is the one scan that links and citations are both read from, so a
// construct lint can see is exactly a construct a renderer is asked to replace,
// and the two cannot drift apart. The prose-and-code decision is the same one
// Links and Citations make: a fenced block or an inline code span is not prose,
// so nothing inside one is a construct.
//
// A wikilink and a citation can, in a deliberately awkward body, cover the same
// bytes: "[[@key]]" is a link to "@key" and, inside it, a citation of "key".
// Both are returned, because lint reports both. A caller that rewrites the body
// takes the first construct and skips any that overlaps it.
func (p *Page) Inlines() []Inline {
	// Byte offset of each body line, so a range within one line becomes a range
	// within Body(). The terminators count, because Body() is the lines joined.
	lines := p.doc.body()
	offset := make([]int, len(lines)+1)
	for i, l := range lines {
		offset[i+1] = offset[i] + len(l.text) + len(l.eol)
	}

	var out []Inline
	group := 0
	for _, pl := range p.proseLines() {
		lineStart := offset[pl.index-p.doc.bodyStart]
		for _, run := range proseSpans(pl.text) {
			text := pl.text[run.start:run.end]
			base := lineStart + run.start

			for _, sp := range wikilinkSpans(text) {
				target := strings.TrimSpace(text[sp.start+2 : sp.end-2])
				out = append(out, Inline{
					Start: base + sp.start,
					End:   base + sp.end,
					Link:  &Link{Target: target, Name: Normalize(target), Line: pl.line},
				})
			}

			for _, g := range scanCitationGroups(text) {
				cites := make([]Citation, len(g.cites))
				for i, c := range g.cites {
					c.Line = pl.line
					c.Group = group
					c.Position = i
					cites[i] = c
				}
				out = append(out, Inline{
					Start: base + g.span.start,
					End:   base + g.span.end,
					Cites: cites,
				})
				group++
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

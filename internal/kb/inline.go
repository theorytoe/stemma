package kb

import (
	"bytes"
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

// RewriteBody replaces the inline constructs in a page body.
//
// rewrite is offered each construct together with the bytes it occupies, and
// returns the text to put in its place, or false to leave it as it is. Unlike
// RewriteLinks, which changes only the target inside a wikilink's brackets, this
// replaces the whole construct, which is what pruning needs: a link that cannot
// be kept becomes its anchor text and the brackets go with it.
//
// Constructs are offered in order, and one that overlaps a construct already
// replaced is skipped, because a body may cover the same bytes twice and only
// one of the two can win. Everything outside the replaced spans comes back byte
// for byte; the report says whether anything changed, so a caller can leave a
// file it has no reason to write alone.
func (p *Page) RewriteBody(rewrite func(in Inline, text string) (string, bool)) bool {
	body := p.Body()
	var out bytes.Buffer
	last, changed := 0, false
	for _, in := range p.Inlines() {
		if in.Start < last || in.End > len(body) {
			continue
		}
		replacement, ok := rewrite(in, string(body[in.Start:in.End]))
		if !ok {
			continue
		}
		out.Write(body[last:in.Start])
		out.WriteString(replacement)
		last = in.End
		changed = true
	}
	if !changed {
		return false
	}
	out.Write(body[last:])

	// Only the body is replaced, so the spans that separate it from the
	// frontmatter still describe the lines they did: a prefix of the line slice
	// is still that prefix, however many lines the new body has.
	p.doc.lines = append(p.doc.lines[:p.doc.bodyStart:p.doc.bodyStart], splitLines(out.Bytes())...)
	return true
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

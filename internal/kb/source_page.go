package kb

import (
	"fmt"
	"strings"
)

// SourcePage returns the virtual page for one bibliography entry.
//
// A source page is generated and never committed (D45). It carries the reserved
// source type, the key the entry defines, and a body that exposes the record:
// every field as written, the retrieval date and content hash it carries, and
// the pages that cite it. It answers to no name in the link graph, so a real
// page whose title happens to equal a citation key cannot collide with it.
func SourcePage(e *BibEntry, citedBy []string) (*Page, error) {
	return newSourcePage(sourceTitle(e), e.Key(), sourceBody(e, citedBy))
}

// SourcePages returns one virtual page per cited entry, in key order.
//
// Only an entry a page cites gets a page: an exported source page exists
// because a page points at it. Ordering by key rather than by file order keeps
// the generated set the same however the bibliography is stored.
func (k *KB) SourcePages() ([]*Page, error) {
	if k.Bibliography == nil {
		return nil, nil
	}
	var out []*Page
	for _, key := range k.Bibliography.SortedKeys() {
		cited := k.Graph.CitedBy(key)
		if len(cited) == 0 {
			continue
		}
		e, ok := k.Bibliography.Entry(key)
		if !ok {
			continue
		}
		p, err := SourcePage(e, k.titlesOf(cited))
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// titlesOf turns citing page paths into the names a reader follows, falling back
// to the path when a page is not in the graph.
func (k *KB) titlesOf(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if p, ok := k.Graph.Page(path); ok {
			out = append(out, p.Title())
			continue
		}
		out = append(out, path)
	}
	return out
}

// sourceTitle is the title a source page is shown under: the record's own title,
// or its key when it has none.
func sourceTitle(e *BibEntry) string {
	if t := strings.TrimSpace(StripBraces(e.Title())); t != "" {
		return t
	}
	return e.Key()
}

// sourceBody is what a source page says: the record field by field, then the
// pages that cite it.
//
// The body is generated, which is the one place the tool writes prose. A source
// page is not an authored page, so nothing here is content a person wrote and
// could lose.
func sourceBody(e *BibEntry, citedBy []string) string {
	var b strings.Builder
	fields := e.Fields()
	if len(fields) == 0 {
		b.WriteString("This record has no fields.\n")
	}
	for _, f := range fields {
		v, _ := e.Value(f.Name)
		fmt.Fprintf(&b, "- **%s**: %s\n", StripBraces(f.Name), StripBraces(v))
	}

	b.WriteString("\n## Cited by\n\n")
	if len(citedBy) == 0 {
		b.WriteString("Nothing in this knowledge base cites this source.\n")
		return b.String()
	}
	for _, title := range citedBy {
		fmt.Fprintf(&b, "- [[%s]]\n", title)
	}
	return b.String()
}

// newSourcePage builds the page the tool owns. It is not NewPage because it
// writes a body, and NewPage deliberately does not: the difference is that this
// page is generated, while an authored page's body belongs to its author.
func newSourcePage(title, key, body string) (*Page, error) {
	p, err := NewPage(title, TypeSource)
	if err != nil {
		return nil, err
	}
	if err := p.Set(FieldKey, key); err != nil {
		return nil, err
	}

	d := &p.doc
	lines := splitLines([]byte(body))
	next := make([]line, 0, d.bodyStart+len(lines))
	next = append(next, d.lines[:d.bodyStart]...)
	next = append(next, lines...)
	d.lines = next
	if err := d.locate(); err != nil {
		return nil, err
	}
	return p, p.load()
}

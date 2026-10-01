// Package render turns a KB's pages into HTML.
//
// Rendering and lint read the same resolution: a wikilink is resolved by the
// page graph and a citation by the bibliography, through the same functions and
// over the same scan. A page that lints clean therefore renders without a
// dangling link, and a page that does not has the broken link marked rather
// than dropped.
//
// The format leaves the HTML shape open, so what is fixed here is the site's
// choice: a resolved wikilink points at its page and shows that page's title; a
// link that does not resolve keeps the text the author wrote inside a span a
// stylesheet can reach; and a citation links each key to that key's source
// page.
package render

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"strings"

	blackfriday "github.com/russross/blackfriday/v2"

	"github.com/theorytoe/stemma/internal/citestyle"
	"github.com/theorytoe/stemma/internal/kb"
)

// PageURL is the URL a page is served at in the generated site.
//
// A path under pages/ loses that prefix and its ".md" and gains ".html":
// "pages/index.md" is "index.html" and "pages/notes/one.md" is "notes/one.html".
// Each segment is escaped, so a file name with a space in it becomes a URL that
// still works.
func PageURL(path string) string {
	p := strings.TrimPrefix(path, kb.PagesDir+"/")
	p = strings.TrimSuffix(p, ".md")
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/") + ".html"
}

// SourceURL is the URL a citation key's virtual source page is served at.
func SourceURL(key string) string {
	return kb.SourcesDir + "/" + url.PathEscape(key) + ".html"
}

// SourceTextURL is the URL a vendored capture is served at.
//
// It is derived from the key and not from the capture's file name, so
// re-vendoring the text does not move its address, and it is a separate document
// from the source's record page so the record stays small however long the
// capture is.
func SourceTextURL(key string) string {
	return kb.SourcesDir + "/" + url.PathEscape(key) + "-text.html"
}

// FilePath is the unescaped form of a site URL: the name a build writes to
// disk and the name a server matches a decoded request against. A URL carries
// escapes and a file name does not — a page with a space in its name is served
// at "two%20words.html" and stored as "two words.html" — so a consumer that
// puts a URL on disk, or looks one up from an incoming request, decodes each
// segment first. Both entry points decode here, so the site built and the site
// served are the same site.
func FilePath(pageURL string) (string, error) {
	segs := strings.Split(pageURL, "/")
	for i, s := range segs {
		decoded, err := url.PathUnescape(s)
		if err != nil {
			return "", fmt.Errorf("unreadable URL %s: %w", pageURL, err)
		}
		segs[i] = decoded
	}
	return strings.Join(segs, "/"), nil
}

// Renderer renders one KB's pages in one citation style.
type Renderer struct {
	kb        *kb.KB
	formatter citestyle.Formatter
	templates *template.Template
	title     string
	home      string
	search    bool

	// backCounts and outCounts are each page's backlinks and its distinct
	// resolved outgoing links; topDegree is the largest of their sum. They are
	// the metric the whole-KB graph sizes nodes by, computed once so rendering
	// stays linear in the KB.
	backCounts map[string]int
	outCounts  map[string]int
	topDegree  int
}

// New returns a renderer for k, using the citation style its manifest names. An
// empty name is the default style. An unknown one is an error, because
// rendering in a style nobody asked for is worse than refusing.
func New(k *kb.KB) (*Renderer, error) {
	f, err := citestyle.Parse(k.Manifest.CitationStyle)
	if err != nil {
		return nil, err
	}
	t, err := parseTemplates()
	if err != nil {
		return nil, err
	}
	back, out, top := nodeCounts(k)
	return &Renderer{
		kb:         k,
		formatter:  f,
		templates:  t,
		title:      k.Manifest.Title,
		home:       homeURL(),
		backCounts: back,
		outCounts:  out,
		topDegree:  top,
	}, nil
}

// Formatter is the citation formatter in use, so that a caller rendering a
// page's reference list renders it in the same style as the page's citations.
func (r *Renderer) Formatter() citestyle.Formatter { return r.formatter }

// Body renders the body of the page at path to HTML, and returns the findings
// the page's links and citations raise under mode.
//
// The findings are the ones lint reports for the page, read from the same
// helpers, so a page cannot be clean to one and broken to the other.
func (r *Renderer) Body(mode kb.Mode, path string) ([]byte, []kb.Finding, error) {
	page, ok := r.kb.Graph.Page(path)
	if !ok {
		return nil, nil, fmt.Errorf("no page at %s", path)
	}
	refs, _ := r.kb.References(path)
	return r.bodyHTML(page, refs, PageURL(path)), r.kb.FindingsFor(path, mode), nil
}

// bodyHTML renders one page's body as an HTML fragment. docURL is the page's
// own address, which every link in the fragment is written relative to.
func (r *Renderer) bodyHTML(page *kb.Page, refs []kb.Reference, docURL string) []byte {
	src := r.expand(page, refs, docURL)
	return blackfriday.Run(src, blackfriday.WithExtensions(blackfriday.CommonExtensions))
}

// expand replaces every wikilink and citation in the page body with the HTML it
// stands for, leaving everything else, code included, exactly as it was.
//
// Replacing the constructs before the markdown parse, rather than rewriting the
// tree after it, is what keeps rendering and lint in step. Blackfriday does not
// keep the "[[" or the "[@", so a tree walk would see a link the scanner had
// split or re-spelled -- "[[a *b* c]]" is three nodes, not one link. The body
// the scanner read is the body that is expanded.
func (r *Renderer) expand(page *kb.Page, refs []kb.Reference, docURL string) []byte {
	body := page.Body()
	var buf bytes.Buffer
	last := 0
	for _, in := range page.Inlines() {
		if in.Start < last {
			// A construct overlapping one already replaced. The scanner reports
			// both because lint does; the first one written wins here.
			continue
		}
		buf.Write(body[last:in.Start])
		if in.Link != nil {
			buf.WriteString(r.link(in.Link, docURL))
		} else {
			buf.WriteString(r.citations(in.Cites, refs, docURL))
		}
		last = in.End
	}
	buf.Write(body[last:])
	return buf.Bytes()
}

// link renders one wikilink. A resolved link points at its page and shows that
// page's title; a link that resolves to nothing, or to several pages, keeps its
// written text inside a span so the prose still reads and a stylesheet can mark
// it.
func (r *Renderer) link(l *kb.Link, docURL string) string {
	switch res := r.kb.Graph.Resolve(l.Name); res.Kind {
	case kb.Resolved:
		text := l.Target
		if target, ok := r.kb.Graph.Page(res.Path); ok && target.Title() != "" {
			text = target.Title()
		}
		return anchor(rel(docURL, PageURL(res.Path)), text)
	case kb.Ambiguous:
		return marked("stemma-ambiguous", l.Target)
	default:
		return marked("stemma-unresolved", l.Target)
	}
}

// citations renders one citation group. Each key the bibliography defines
// becomes a link to that key's source page; a key it does not define stays
// text, and the page's findings already name it.
func (r *Renderer) citations(cites []kb.Citation, refs []kb.Reference, docURL string) string {
	l := r.formatter.Layout(cites, refs)
	var b strings.Builder
	b.WriteString(html.EscapeString(l.Before))
	for i, p := range l.Pieces {
		if i > 0 {
			b.WriteString(html.EscapeString(l.Sep))
		}
		if r.defined(p.Key) {
			b.WriteString(anchor(rel(docURL, SourceURL(p.Key)), p.Text))
		} else {
			b.WriteString(html.EscapeString(singleLine(p.Text)))
		}
	}
	b.WriteString(html.EscapeString(l.After))
	return b.String()
}

// defined reports whether the bibliography has an entry for a key, and so
// whether that key has a source page to point at.
func (r *Renderer) defined(key string) bool {
	return r.kb.Bibliography != nil && r.kb.Bibliography.Has(key)
}

// anchor is a link with its URL and its text escaped. The text is flattened to
// one line, because an anchor's text cannot be a block.
func anchor(href, text string) string {
	return `<a href="` + html.EscapeString(href) + `">` + html.EscapeString(singleLine(text)) + `</a>`
}

// marked is text that stands where a link would have, marked with a class.
func marked(class, text string) string {
	return `<span class="` + class + `">` + html.EscapeString(singleLine(text)) + `</span>`
}

// singleLine keeps a value that may have come from frontmatter out of the
// line structure of the HTML. A title can be written as a block scalar, and
// neither an anchor's text nor a link's label can carry a newline.
func singleLine(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
}

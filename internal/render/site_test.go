package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

func siteRenderer(t *testing.T) *Renderer {
	t.Helper()
	r, err := New(testKB(t))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPageDocumentGolden(t *testing.T) {
	r := siteRenderer(t)
	got, err := r.Page("pages/sample.md")
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "page.golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s", golden)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading the golden file (run `go test ./internal/render -update` to write it): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("the page changed.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestDocumentsAreUniqueAndComplete checks the whole site is produced, and that
// no two documents claim one URL.
func TestDocumentsAreUniqueAndComplete(t *testing.T) {
	r := siteRenderer(t)
	docs, err := r.Documents()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range docs {
		if seen[d.URL] {
			t.Errorf("two documents share the URL %s", d.URL)
		}
		seen[d.URL] = true
	}
	for _, want := range []string{
		"index.html",             // the home page
		"all.html",               // the generated page list
		"types.html",             // the generated type list
		"tags.html",              // the generated tag list
		"graph.html",             // the generated whole-KB graph
		"sample.html",            // an authored page
		"odd%20name.html",        // a file name with a space
		"types/concept.html",     // a type index
		"types/index.html",       // the reserved index type still gets one
		"tags/attention.html",    // a tag index
		"tags/architecture.html", // another tag
		"sources/vaswani2017.html",
		"assets/style.css",
		"assets/theme.js",
		"assets/graph.js",
	} {
		if !seen[want] {
			t.Errorf("Documents did not produce %s", want)
		}
	}
}

// TestScriptsAreProgressive is the P9 gate at the template level. JavaScript is
// polish, so no document carries inline code, and the one control that needs a
// script -- the theme toggle -- is hidden until the script shows it. A reader
// with JavaScript off therefore sees no dead control and the OS theme applies.
func TestScriptsAreProgressive(t *testing.T) {
	r := siteRenderer(t)
	docs, err := r.Documents()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if !strings.Contains(d.Type, "html") {
			continue
		}
		body := string(d.Body)
		if strings.Contains(body, "<script>") {
			t.Errorf("%s carries an inline script", d.URL)
		}
		if !strings.Contains(body, "theme.js") {
			t.Errorf("%s does not load the theme script", d.URL)
		}
		if strings.Contains(body, "data-theme-toggle") && !strings.Contains(body, "data-theme-toggle hidden>") {
			t.Errorf("%s shows the theme toggle without a script to drive it", d.URL)
		}
	}
}

func TestPageDocumentHasChrome(t *testing.T) {
	r := siteRenderer(t)
	got, err := r.Page("pages/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<link rel="stylesheet" href="assets/style.css">`,
		`<a class="site-title" href="index.html">test</a>`,
		`<a href="all.html">Index</a>`,
		`<a href="types.html">Types</a>`,
		`<a href="tags.html">Tags</a>`,
		`<h1>Sample</h1>`,
		`<section class="references">`,
		`Built with stemma.`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("the page is missing %q:\n%s", want, got)
		}
	}

	// A page nothing links to has no backlinks section; a page something links
	// to has one, pointing at the page that links there.
	linked, err := r.Page("pages/attention.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(linked), `<nav class="backlinks">`) {
		t.Errorf("a linked page has no backlinks section:\n%s", linked)
	}
	if !strings.Contains(string(linked), `<a href="sample.html">Sample</a>`) {
		t.Errorf("the backlink does not point at the linking page:\n%s", linked)
	}
}

func TestMetaLine(t *testing.T) {
	r := siteRenderer(t)

	attention, err := r.Page("pages/attention.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<a href="types/concept.html">concept</a>`,
		`also known as: transformer paper`,
	} {
		if !strings.Contains(string(attention), want) {
			t.Errorf("the meta line is missing %q:\n%s", want, attention)
		}
	}

	transformer, err := r.Page("pages/transformer.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(transformer), `tags: <a href="tags/architecture.html">architecture</a>`) {
		t.Errorf("the meta line does not list tags as links:\n%s", transformer)
	}
}

// TestTypesAndTagsAreSeparatePages checks the lists were split apart: the page
// list carries pages and neither of the other two, and each of the others
// carries only its own.
func TestTypesAndTagsAreSeparatePages(t *testing.T) {
	r := siteRenderer(t)

	index, err := r.All()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `class="page-list"`) {
		t.Errorf("the page list is missing from the Index:\n%s", index)
	}
	for _, unwanted := range []string{`class="type-list"`, `class="tag-list"`} {
		if strings.Contains(string(index), unwanted) {
			t.Errorf("the Index still carries %s:\n%s", unwanted, index)
		}
	}

	types, err := r.Types()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(types), "<h1>Types</h1>") || !strings.Contains(string(types), ">concept<") {
		t.Errorf("the types page is missing a type:\n%s", types)
	}
	if strings.Contains(string(types), `class="page-list"`) {
		t.Errorf("the types page carries the page list:\n%s", types)
	}

	tags, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tags), "<h1>Tags</h1>") || !strings.Contains(string(tags), ">architecture<") {
		t.Errorf("the tags page is missing a tag:\n%s", tags)
	}
}

// TestPageGraph checks the baseline the script upgrades: the SVG is drawn, its
// nodes are real links, and the embedded JSON is the same graph.
func TestPageGraph(t *testing.T) {
	r := siteRenderer(t)
	got, err := r.Page("pages/attention.md")
	if err != nil {
		t.Fatal(err)
	}
	page := string(got)
	for _, want := range []string{
		`<section class="graph">`,
		`<h2>Local graph</h2>`,
		`<svg class="graph-svg"`,
		`<g class="graph-node graph-self">`,
		`<a class="graph-node" href="sample.html">`,
		`<script type="application/json" class="graph-data">`,
		`assets/graph.js`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the graph is missing %q:\n%s", want, page)
		}
	}

	const open = `<script type="application/json" class="graph-data">`
	i := strings.Index(page, open)
	j := strings.Index(page[i:], "</script>")
	if i < 0 || j < 0 {
		t.Fatalf("no embedded graph JSON:\n%s", page)
	}
	raw := page[i+len(open) : i+j]

	var data GraphData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("the embedded graph is not JSON: %v\n%s", err, raw)
	}
	if data.Center.Title != "Attention Is All You Need" {
		t.Errorf("center = %q", data.Center.Title)
	}
	if len(data.Backlinks) != 1 || data.Backlinks[0].Title != "Sample" {
		t.Errorf("backlinks = %+v", data.Backlinks)
	}
	// The local graph carries backlinks for the hover tip and no degree: it is
	// drawn uniform.
	if data.Center.Backlinks != 1 {
		t.Errorf("the centre has %d backlinks, want 1", data.Center.Backlinks)
	}
	if data.Center.Degree != 0 {
		t.Errorf("the local graph set a degree (%d); it should not size nodes", data.Center.Degree)
	}
}

// TestGraphWithoutNeighbours checks a page with no links still gets a graph:
// itself, and no others.
func TestGraphWithoutNeighbours(t *testing.T) {
	r := siteRenderer(t)
	got, err := r.Page("pages/transformer.md")
	if err != nil {
		t.Fatal(err)
	}
	page := string(got)
	if !strings.Contains(page, `<g class="graph-node graph-self">`) {
		t.Errorf("a lone page has no self node:\n%s", page)
	}
	if strings.Contains(page, `<a class="graph-node"`) {
		t.Errorf("a page with no neighbours has neighbour nodes:\n%s", page)
	}
}

// TestWholeKBGraph checks the graph page draws every page and every link, and
// that its embedded JSON is the same graph the SVG drew.
func TestWholeKBGraph(t *testing.T) {
	r := siteRenderer(t)
	got, err := r.AllGraph()
	if err != nil {
		t.Fatal(err)
	}
	page := string(got)
	for _, want := range []string{
		`<h1>Graph</h1>`,
		`<svg class="graph-svg"`,
		`<script type="application/json" class="graph-data">`,
		`assets/graph.js`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the graph page is missing %q:\n%s", want, page)
		}
	}

	const open = `<script type="application/json" class="graph-data">`
	i := strings.Index(page, open)
	j := strings.Index(page[i:], "</script>")
	if i < 0 || j < 0 {
		t.Fatalf("no embedded graph JSON:\n%s", page)
	}
	var g GraphJSON
	if err := json.Unmarshal([]byte(page[i+len(open):i+j]), &g); err != nil {
		t.Fatalf("the embedded graph is not JSON: %v", err)
	}

	if len(g.Nodes) != r.kb.Graph.Len() {
		t.Errorf("nodes = %d, want one per page (%d)", len(g.Nodes), r.kb.Graph.Len())
	}
	// The only link in the fixture is Sample's several links to Attention,
	// which are one edge once deduplicated.
	if len(g.Edges) != 1 {
		t.Errorf("edges = %d, want 1: %+v", len(g.Edges), g.Edges)
	}

	top := 0
	for _, n := range g.Nodes {
		if n.Degree > top {
			top = n.Degree
		}
	}
	if top != g.MaxDegree {
		t.Errorf("the busiest node has degree %d, but max_degree is %d", top, g.MaxDegree)
	}

	// Every node is tinted from the palette, and the static SVG uses the same
	// colour the script will.
	for _, n := range g.Nodes {
		if n.Color < 0 || n.Color >= graphColors {
			t.Errorf("node %q has colour %d, out of range", n.Title, n.Color)
		}
	}
}

func TestTypeAndTagIndexes(t *testing.T) {
	r := siteRenderer(t)

	concept, err := r.TypeIndex("concept")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Attention Is All You Need", "Transformer"} {
		if !strings.Contains(string(concept), want) {
			t.Errorf("the concept index is missing %q", want)
		}
	}

	tagged, err := r.TagIndex("attention")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tagged), "Transformer") {
		t.Errorf("the attention tag index is missing Transformer:\n%s", tagged)
	}
	if strings.Contains(string(tagged), "Odd Name") {
		t.Errorf("the attention tag index has a page that does not carry the tag:\n%s", tagged)
	}
}

func TestSourcePage(t *testing.T) {
	r := siteRenderer(t)
	got, err := r.Source("vaswani2017")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Attention Is All You Need",
		`key: vaswani2017`,
		"Cited by",
		`<a href="../sample.html">Sample</a>`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("the source page is missing %q:\n%s", want, got)
		}
	}
}

// TestCollisionIsReported checks that a page whose name would sit on top of a
// generated index is an error rather than a silent overwrite.
func TestCollisionIsReported(t *testing.T) {
	k := testKB(t)
	p, err := kb.ParsePage([]byte("---\ntitle: All\ntype: note\n---\nClashes with the Index page.\n"))
	if err != nil {
		t.Fatal(err)
	}
	k.Graph.Add("pages/all.md", p)

	r, err := New(k)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Documents(); err == nil {
		t.Error("a page colliding with the Index page was accepted")
	}
}

// TestLinksAreRelativeToTheDocument checks the property the build needs: a page
// addresses everything relative to itself, so a document one level down climbs
// out with "../" and the output directory can be moved without rewriting links.
func TestLinksAreRelativeToTheDocument(t *testing.T) {
	r := siteRenderer(t)

	root, err := r.Page("pages/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root), `href="attention.html"`) {
		t.Errorf("a root page should link to a sibling by name:\n%s", root)
	}

	typ, err := r.TypeIndex("concept")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`href="../attention.html"`,   // a page, one level up
		`href="../assets/style.css"`, // the stylesheet, one level up
		`href="../index.html"`,       // the home page, one level up
	} {
		if !strings.Contains(string(typ), want) {
			t.Errorf("the type index is missing %q:\n%s", want, typ)
		}
	}
}

// TestURLsAreNotDoubleEscaped checks that an already-escaped page URL survives
// the template's own URL escaping.
func TestURLsAreNotDoubleEscaped(t *testing.T) {
	r := siteRenderer(t)
	got, err := r.All()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `href="odd%20name.html"`) {
		t.Errorf("the space in a page URL was not escaped once:\n%s", got)
	}
	if strings.Contains(string(got), "%2520") {
		t.Errorf("the page URL was escaped twice:\n%s", got)
	}
}

// vendoredRenderer builds a renderer over a KB on disk with one cited source
// whose text is vendored. The capture is a file in the KB, so an in-memory KB
// cannot carry one.
func vendoredRenderer(t *testing.T) *Renderer {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	text := "the captured text\n"
	write("pages/index.md", "---\ntitle: Home\ntype: index\n---\nSee [@bush1945].\n")
	write("bibliography.bib", "@article{bush1945,\n"+
		"  title = {As We May Think},\n"+
		"  path = {/nowhere/not-read.txt},\n"+
		"  stemma-vendored-hash = {"+kb.HashOf([]byte(text))+"},\n"+
		"}\n")
	write(kb.VendoredName("bush1945"), text)

	k, err := kb.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(k)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestVendoredSourceIsServedAndLinked is the point of the whole path: a capture
// in sources/ becomes a document of the site, and the record page points at it.
func TestVendoredSourceIsServedAndLinked(t *testing.T) {
	r := vendoredRenderer(t)
	docs, err := r.Documents()
	if err != nil {
		t.Fatal(err)
	}
	byURL := map[string]Document{}
	for _, d := range docs {
		byURL[d.URL] = d
	}

	url := SourceTextURL("bush1945")
	doc, ok := byURL[url]
	if !ok {
		t.Fatalf("Documents did not produce %s", url)
	}
	if !strings.Contains(string(doc.Body), "the captured text") {
		t.Errorf("the capture is not on the text page:\n%s", doc.Body)
	}
	if !strings.Contains(string(doc.Body), "captured text of") {
		t.Errorf("the text page does not name its record:\n%s", doc.Body)
	}

	record, ok := byURL[SourceURL("bush1945")]
	if !ok {
		t.Fatal("Documents did not produce the record page")
	}
	if !strings.Contains(string(record.Body), url) {
		t.Errorf("the record page does not link to %s:\n%s", url, record.Body)
	}
}

// TestVendoredTextIsEscaped checks that a capture is shown as text. An
// extraction can contain anything, including something that looks like markup.
func TestVendoredTextIsEscaped(t *testing.T) {
	r := vendoredRenderer(t)
	art, ok := kb.FindOriginal(r.kb.Root, "bush1945")
	if !ok {
		t.Fatal("no capture to render")
	}
	if err := os.WriteFile(art.Path, []byte("<b>not markup</b>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := r.SourceText("bush1945", art)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "&lt;b&gt;not markup&lt;/b&gt;") {
		t.Errorf("the capture was not escaped:\n%s", got)
	}
	if strings.Contains(string(got), "<b>not markup</b>") {
		t.Errorf("the capture was rendered as markup:\n%s", got)
	}
}

// writeOriginal puts an original document beside the vendored key's extracted
// text, which is what a capture made from a local file now has.
func writeOriginal(t *testing.T, r *Renderer, ext string, body []byte) {
	t.Helper()
	full := kb.OriginalPath(r.kb.Root, "bush1945", ext)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestVendoredMarkdownIsRendered checks that a markdown original is a document,
// not a preformatted file; that its frontmatter is dropped as metadata; and that
// raw HTML in it is dropped.
func TestVendoredMarkdownIsRendered(t *testing.T) {
	r := vendoredRenderer(t)
	writeOriginal(t, r, ".md", []byte("---\ntitle: Meta\n---\n# Heading\n\nA [link](https://example.com) and <script>bad()</script>.\n"))

	art, ok := kb.FindOriginal(r.kb.Root, "bush1945")
	if !ok || art.Ext != ".md" {
		t.Fatalf("FindOriginal = %+v, %v; want the markdown", art, ok)
	}
	got, err := r.SourceText("bush1945", art)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "<h1>Heading</h1>") {
		t.Errorf("the markdown was not rendered:\n%s", got)
	}
	if strings.Contains(string(got), "title: Meta") {
		t.Errorf("the frontmatter was rendered as content:\n%s", got)
	}
	if !strings.Contains(string(got), `<a href="https://example.com">link</a>`) {
		t.Errorf("a markdown link was not rendered:\n%s", got)
	}
	if strings.Contains(string(got), "<script>") {
		t.Errorf("raw HTML in a capture reached the page:\n%s", got)
	}
}

// TestVendoredPDFIsEmbeddedAndServed checks the PDF path: the view frames the
// browser's own viewer, and the bytes are served as a PDF.
func TestVendoredPDFIsEmbeddedAndServed(t *testing.T) {
	r := vendoredRenderer(t)
	writeOriginal(t, r, ".pdf", []byte("%PDF-1.4\nnot really a pdf\n"))

	docs, err := r.Documents()
	if err != nil {
		t.Fatal(err)
	}
	byURL := map[string]Document{}
	for _, d := range docs {
		byURL[d.URL] = d
	}

	rawURL := SourceRawURL("bush1945", ".pdf")
	raw, ok := byURL[rawURL]
	if !ok {
		t.Fatalf("Documents did not produce the raw PDF at %s", rawURL)
	}
	if raw.Type != "application/pdf" {
		t.Errorf("raw type = %q, want application/pdf", raw.Type)
	}

	view, ok := byURL[SourceTextURL("bush1945")]
	if !ok {
		t.Fatal("Documents did not produce the view page")
	}
	if !strings.Contains(string(view.Body), `class="capture-pdf"`) {
		t.Errorf("the PDF is not embedded:\n%s", view.Body)
	}
	if !strings.Contains(string(view.Body), rawURL) {
		t.Errorf("the view does not point at the raw PDF:\n%s", view.Body)
	}
}

// TestSourceTextURLIsStableAcrossReVendoring pins the address to the key rather
// than to the capture's file name.
func TestSourceTextURLIsStableAcrossReVendoring(t *testing.T) {
	if got, want := SourceTextURL("vaswani2017"), "sources/vaswani2017-text.html"; got != want {
		t.Errorf("SourceTextURL = %q, want %q", got, want)
	}
	if got, want := SourceTextURL("a/b:c"), "sources/a%2Fb:c-text.html"; got != want {
		t.Errorf("SourceTextURL = %q, want %q", got, want)
	}
}

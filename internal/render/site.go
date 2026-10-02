package render

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed assets/*
var assetsFS embed.FS

const (
	// htmlType is the media type of every document the templates produce.
	htmlType = "text/html; charset=utf-8"

	// assetDir is where the static files live, in the build and in the embed.
	assetDir = "assets"

	// indexURL is the generated list of every page. It is not "index.html"
	// because that is the home page, the authored entry document the whole site
	// is rooted at.
	indexURL = "all.html"

	// typesURL and tagsURL are the generated lists of the types and the tags,
	// each kept apart from the page list and from each other.
	typesURL = "types.html"
	tagsURL  = "tags.html"

	// graphURL is the generated graph of the whole KB.
	graphURL = "graph.html"
)

// SiteData is what every document knows about the site around it. Every address
// is written relative to the document it is used in.
type SiteData struct {
	Title          string
	HomeURL        string
	IndexURL       string
	TypesURL       string
	TagsURL        string
	GraphURL       string
	AssetURL       string
	ScriptURL      string
	GraphScriptURL string
	SearchURL      string
}

// common is embedded in each document's data so the shared partials can reach
// the site and the title that goes in the browser tab.
type common struct {
	Site     SiteData
	DocTitle string
}

// MetaItem is one part of a page's meta line: a bare fact, or a labelled one,
// optionally linking somewhere.
type MetaItem struct {
	Label string
	Text  string
	URL   string
}

// PageLink is a page as a list shows it.
type PageLink struct {
	Title string
	URL   string
	Type  string
}

// TypeLink is a type with the number of pages of it.
type TypeLink struct {
	Name  string
	URL   string
	Count int
}

// TagLink is a tag with the number of pages carrying it.
type TagLink struct {
	Name  string
	URL   string
	Count int
}

// ReferenceData is one entry in a page's reference list: the formatted citation,
// and the URL of the source page it belongs to.
type ReferenceData struct {
	Text string
	URL  string
}

// PageData is a page rendered as a whole document.
type PageData struct {
	common
	Title      string
	Meta       []MetaItem
	Action     *PageAction
	Body       template.HTML
	References []ReferenceData
	Backlinks  []PageLink
	Graph      template.HTML
	GraphJSON  template.JS
}

// PageAction is a link at the top of a page, such as the full text of a
// vendored source. An authored page has none.
type PageAction struct {
	Text string
	URL  string
}

// SourceTextData is a vendored capture rendered as a page: markdown as HTML, a
// PDF in a frame, or anything else as text. The original bytes are always a link
// away.
type SourceTextData struct {
	common
	Title     string
	Key       string
	RecordURL string
	RawURL    string
	PDFURL    string
	Body      template.HTML
}

// ListingData is an index page: a heading and one list under it. A page list,
// a type list and a tag list are the same shape, so they share this and the
// listing template; each page sets only the one it is.
type ListingData struct {
	common
	Heading string
	Pages   []PageLink
	Types   []TypeLink
	Tags    []TagLink
	Empty   string
}

// GraphPageData is the whole-KB graph page.
type GraphPageData struct {
	common
	Heading   string
	Summary   string
	Graph     template.HTML
	GraphJSON template.JS
}

// SearchHit is one result on the search page.
type SearchHit struct {
	Title   string
	URL     string
	Snippet template.HTML
}

// SearchData is the search page: the query and the results it found.
type SearchData struct {
	common
	Query    string
	Searched bool
	Hits     []SearchHit
}

// Document is one file the site serves or writes.
type Document struct {
	URL  string
	Body []byte
	Type string
}

// TypeURL is the URL of a type's index page.
func TypeURL(typ string) string { return "types/" + url.PathEscape(typ) + ".html" }

// TagURL is the URL of a tag's index page. A tag's identity is its normalised
// form, so every spelling of it shares one page.
func TagURL(tag string) string { return "tags/" + url.PathEscape(kb.Normalize(tag)) + ".html" }

func parseTemplates() (*template.Template, error) {
	t, err := template.New("stemma").ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}
	return t, nil
}

// homeURL is the address of the site's home page, which is always the generated
// landing page.
//
// It is the entry document's address, and that is the point of the whole design:
// when a KB has an entry document, the landing page is rendered at that address
// and the entry document's body is the page's opening, so the page the author
// wrote and the page a reader arrives at are one document rather than two wanting
// the same URL. When a KB has no entry document, the address is still this one
// and the landing page is generated alone.
func homeURL() string { return PageURL(kb.EntryDocument) }

// rel is a link from one document to another, written relative to the first.
// Every address the templates and the renderer emit goes through it, so a page
// under types/ points at ../attention.html and a page at the root points at
// attention.html. That is what lets the output directory be moved or opened
// from disk without rewriting a link.
func rel(from, to string) string {
	dir := path.Dir(from)
	if dir == "." || dir == "/" {
		return to
	}
	// One "../" per segment of the document's own directory. A link that
	// climbs and comes back down resolves the same as a shorter one, so this
	// does not try to be minimal.
	depth := strings.Count(dir, "/") + 1
	return strings.Repeat("../", depth) + to
}

// siteFor is the site data one document sees: the same site, addressed relative
// to where the document sits.
func (r *Renderer) siteFor(docURL string) SiteData {
	s := SiteData{
		Title:          r.title,
		HomeURL:        rel(docURL, r.home),
		IndexURL:       rel(docURL, indexURL),
		TypesURL:       rel(docURL, typesURL),
		TagsURL:        rel(docURL, tagsURL),
		GraphURL:       rel(docURL, graphURL),
		AssetURL:       rel(docURL, assetDir+"/style.css"),
		ScriptURL:      rel(docURL, assetDir+"/theme.js"),
		GraphScriptURL: rel(docURL, assetDir+"/graph.js"),
	}
	if r.search {
		s.SearchURL = rel(docURL, "search")
	}
	return s
}

// EnableSearch adds the search form. Only a server can answer it, so a built
// site leaves it off rather than shipping a form that submits to nothing.
func (r *Renderer) EnableSearch() { r.search = true }

// execute runs one named template over some data.
func (r *Renderer) execute(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Page renders the page at pagePath as a whole HTML document.
func (r *Renderer) Page(pagePath string) ([]byte, error) {
	page, ok := r.kb.Graph.Page(pagePath)
	if !ok {
		return nil, fmt.Errorf("no page at %s", pagePath)
	}
	refs, _ := r.kb.References(pagePath)
	return r.execute("page", r.pageData(pagePath, page, refs))
}

// All renders the Index page: every page, then the types and the tags.
// All renders the list of every page.
func (r *Renderer) All() ([]byte, error) {
	docURL := indexURL
	pages := r.pageLinks(r.kb.Graph.Paths(), docURL)
	data := ListingData{
		common:  common{Site: r.siteFor(docURL), DocTitle: tabTitle("Index", r.title)},
		Heading: "Index",
		Pages:   pages,
	}
	if len(pages) == 0 {
		data.Empty = "No pages yet."
	}
	return r.execute("listing", data)
}

// Types renders the list of every type that has pages.
func (r *Renderer) Types() ([]byte, error) {
	docURL := typesURL
	types := r.typeLinks(docURL)
	data := ListingData{
		common:  common{Site: r.siteFor(docURL), DocTitle: tabTitle("Types", r.title)},
		Heading: "Types",
		Types:   types,
	}
	if len(types) == 0 {
		data.Empty = "No types yet."
	}
	return r.execute("listing", data)
}

// Tags renders the list of every tag in use.
func (r *Renderer) Tags() ([]byte, error) {
	docURL := tagsURL
	tags := r.tagLinks(docURL)
	data := ListingData{
		common:  common{Site: r.siteFor(docURL), DocTitle: tabTitle("Tags", r.title)},
		Heading: "Tags",
		Tags:    tags,
	}
	if len(tags) == 0 {
		data.Empty = "No tags yet."
	}
	return r.execute("listing", data)
}

// TypeIndex renders the index of one type.
func (r *Renderer) TypeIndex(typ string) ([]byte, error) {
	docURL := TypeURL(typ)
	data := ListingData{
		common:  common{Site: r.siteFor(docURL), DocTitle: tabTitle(typ, r.title)},
		Heading: typ,
		Pages:   r.pageLinks(r.pathsOfType(typ), docURL),
	}
	return r.execute("listing", data)
}

// TagIndex renders the index of one tag, named by its normalised form.
func (r *Renderer) TagIndex(norm string) ([]byte, error) {
	docURL := TagURL(norm)
	data := ListingData{
		common:  common{Site: r.siteFor(docURL), DocTitle: tabTitle(norm, r.title)},
		Heading: "Tagged: " + norm,
		Pages:   r.pageLinks(r.pathsWithTag(norm), docURL),
	}
	return r.execute("listing", data)
}

// AllGraph renders the whole-KB graph page.
func (r *Renderer) AllGraph() ([]byte, error) {
	docURL := graphURL
	g := r.allGraph(docURL)
	data := GraphPageData{
		common:    common{Site: r.siteFor(docURL), DocTitle: tabTitle("Graph", r.title)},
		Heading:   "Graph",
		Summary:   fmt.Sprintf("%d pages, %d links", len(g.Nodes), len(g.Edges)),
		Graph:     allGraphSVG(g),
		GraphJSON: allGraphJSON(g),
	}
	return r.execute("graph", data)
}

// Search renders the search page. hits are the results already found; the
// renderer shows them, it does not search.
func (r *Renderer) Search(query string, hits []SearchHit) ([]byte, error) {
	docURL := "search"
	for i := range hits {
		hits[i].URL = rel(docURL, hits[i].URL)
	}
	data := SearchData{
		common:   common{Site: r.siteFor(docURL), DocTitle: tabTitle("Search", r.title)},
		Query:    query,
		Searched: query != "",
		Hits:     hits,
	}
	return r.execute("search", data)
}

// Source renders the virtual source page for a citation key.
func (r *Renderer) Source(key string) ([]byte, error) {
	page, err := r.sourcePage(key)
	if err != nil {
		return nil, err
	}
	docURL := SourceURL(key)
	g := r.sourceGraph(key, page.Title(), docURL)
	data := PageData{
		common: common{Site: r.siteFor(docURL), DocTitle: tabTitle(page.Title(), r.title)},
		Title:  page.Title(),
		Meta:   []MetaItem{{Text: kb.TypeSource}, {Label: "key", Text: page.Key()}},
		Body:   template.HTML(r.bodyHTML(page, nil, docURL)),

		Graph:     graphSVG(g),
		GraphJSON: graphJSON(g),
	}
	// A capture the KB holds is a link under the title, not a line of meta: it is
	// the reason a reader opened the record. A capture that is claimed but missing
	// is not linked at all, because `cite check` reports it and the site should not
	// offer a page that is not there.
	if _, ok := kb.FindOriginal(r.kb.Root, key); ok {
		data.Action = &PageAction{
			Text: "Read the full text",
			URL:  rel(docURL, SourceTextURL(key)),
		}
	}
	return r.execute("page", data)
}

// SourceText renders a capture as a page: a PDF embedded, markdown rendered, and
// anything else shown as text. Whatever it is, the original bytes stay a link
// away, so the rendered view never replaces the evidence.
func (r *Renderer) SourceText(key string, art kb.VendoredArtifact) ([]byte, error) {
	page, err := r.sourcePage(key)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(art.Path)
	if err != nil {
		return nil, err
	}
	docURL := SourceTextURL(key)
	view := SourceTextData{
		common:    common{Site: r.siteFor(docURL), DocTitle: tabTitle(page.Title()+" (full text)", r.title)},
		Title:     page.Title(),
		Key:       key,
		RecordURL: rel(docURL, SourceURL(key)),
		RawURL:    rel(docURL, SourceRawURL(key, art.Ext)),
	}
	switch art.Ext {
	case ".md", ".markdown":
		view.Body = template.HTML(markdownHTML(data))
	case ".pdf":
		view.PDFURL = view.RawURL
	default:
		view.Body = template.HTML(`<pre class="capture">` + html.EscapeString(string(data)) + `</pre>`)
	}
	return r.execute("source-text", view)
}

// captureType is the media type a captured original is served as. Only a PDF is
// safe to serve as itself; markdown and text are read as text, and anything else
// is a download rather than something the browser might execute.
func captureType(ext string) string {
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md", ".markdown":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// Documents returns every document in the site, in a stable order: the pages,
// the Index page, the type indexes, the tag indexes, the source pages, and the
// static assets. It is the whole site, and both entry points read it — build
// writes it out and serve answers from it.
//
// A URL produced twice is an error rather than an overwrite, so a page whose
// name collides with a generated index is reported instead of losing one of
// them.
func (r *Renderer) Documents() ([]Document, error) {
	var docs []Document
	addHTML := func(url string, body []byte) {
		docs = append(docs, Document{URL: url, Body: body, Type: htmlType})
	}

	for _, p := range r.kb.Graph.Paths() {
		// The entry document is not written as a page of its own. Its address is
		// the home page's and it is rendered there, as the landing page's
		// opening, so that a link to it arrives where a reader starts.
		if p == kb.EntryDocument {
			continue
		}
		body, err := r.Page(p)
		if err != nil {
			return nil, err
		}
		addHTML(PageURL(p), body)
	}

	body, err := r.Home()
	if err != nil {
		return nil, err
	}
	addHTML(homeURL(), body)

	body, err = r.All()
	if err != nil {
		return nil, err
	}
	addHTML(indexURL, body)

	if body, err = r.Types(); err != nil {
		return nil, err
	}
	addHTML(typesURL, body)

	if body, err = r.Tags(); err != nil {
		return nil, err
	}
	addHTML(tagsURL, body)

	if body, err = r.AllGraph(); err != nil {
		return nil, err
	}
	addHTML(graphURL, body)

	for _, typ := range r.typeNames() {
		body, err := r.TypeIndex(typ)
		if err != nil {
			return nil, err
		}
		addHTML(TypeURL(typ), body)
	}

	for _, tag := range r.tagNames() {
		body, err := r.TagIndex(tag)
		if err != nil {
			return nil, err
		}
		addHTML(TagURL(tag), body)
	}

	for _, key := range r.sourceKeys() {
		body, err := r.Source(key)
		if err != nil {
			return nil, err
		}
		addHTML(SourceURL(key), body)

		// A vendored capture is a document of the site, so a reader who follows a
		// citation to its source can go on to read what was captured. There is no
		// document when nothing was vendored, which is the ordinary case.
		art, ok := kb.FindOriginal(r.kb.Root, key)
		if !ok {
			continue
		}
		body, err = r.SourceText(key, art)
		if err != nil {
			return nil, err
		}
		addHTML(SourceTextURL(key), body)

		raw, err := os.ReadFile(art.Path)
		if err != nil {
			return nil, err
		}
		docs = append(docs, Document{URL: SourceRawURL(key, art.Ext), Body: raw, Type: captureType(art.Ext)})
	}

	assets, err := r.assets()
	if err != nil {
		return nil, err
	}
	docs = append(docs, assets...)

	if err := checkUnique(docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// assets reads the embedded static files.
func (r *Renderer) assets() ([]Document, error) {
	entries, err := fs.ReadDir(assetsFS, assetDir)
	if err != nil {
		return nil, err
	}
	out := make([]Document, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := fs.ReadFile(assetsFS, assetDir+"/"+e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, Document{
			URL:  assetDir + "/" + e.Name(),
			Body: body,
			Type: assetType(e.Name()),
		})
	}
	return out, nil
}

// checkUnique reports a URL produced twice. Two documents at one URL means one
// would overwrite the other, which is a name an author chose colliding with a
// generated index.
func checkUnique(docs []Document) error {
	seen := map[string]bool{}
	for _, d := range docs {
		if seen[d.URL] {
			return fmt.Errorf("two documents share the URL %s; a page name collides with a generated index", d.URL)
		}
		seen[d.URL] = true
	}
	return nil
}

// pageData builds the view of one authored page.
func (r *Renderer) pageData(pagePath string, page *kb.Page, refs []kb.Reference) PageData {
	docURL := PageURL(pagePath)
	g := r.localGraph(pagePath, docURL)
	return PageData{
		common:     common{Site: r.siteFor(docURL), DocTitle: tabTitle(page.Title(), r.title)},
		Title:      page.Title(),
		Meta:       r.meta(page, docURL),
		Body:       template.HTML(r.bodyHTML(page, refs, docURL)),
		References: r.referenceData(refs, docURL),
		Backlinks:  r.backlinks(pagePath, docURL),
		Graph:      graphSVG(g),
		GraphJSON:  graphJSON(g),
	}
}

// meta is the line under a page's title: what the frontmatter says, in the
// order a reader would ask for it. A page with only a title and a type shows
// only those.
func (r *Renderer) meta(page *kb.Page, docURL string) []MetaItem {
	var out []MetaItem
	if typ := page.Type(); typ != "" {
		out = append(out, MetaItem{Text: typ, URL: rel(docURL, TypeURL(typ))})
	}
	if page.Status() == kb.StatusArchived {
		text := "archived"
		if reason := page.ArchiveReason(); reason != "" {
			text = "archived: " + reason
		}
		out = append(out, MetaItem{Text: text})
	}
	for i, tag := range page.Tags() {
		label := ""
		if i == 0 {
			label = "tags"
		}
		out = append(out, MetaItem{Label: label, Text: tag, URL: rel(docURL, TagURL(tag))})
	}
	for i, alias := range page.Aliases() {
		label := ""
		if i == 0 {
			label = "also known as"
		}
		out = append(out, MetaItem{Label: label, Text: alias})
	}
	return out
}

func (r *Renderer) referenceData(refs []kb.Reference, docURL string) []ReferenceData {
	out := make([]ReferenceData, 0, len(refs))
	for i, ref := range refs {
		out = append(out, ReferenceData{
			Text: r.formatter.Entry(ref, i+1),
			URL:  rel(docURL, SourceURL(ref.Entry.Key())),
		})
	}
	return out
}

func (r *Renderer) backlinks(pagePath, docURL string) []PageLink {
	paths := r.kb.Graph.Backlinks(pagePath)
	out := make([]PageLink, 0, len(paths))
	for _, p := range paths {
		if page, ok := r.kb.Graph.Page(p); ok {
			out = append(out, PageLink{Title: page.Title(), URL: rel(docURL, PageURL(p)), Type: page.Type()})
		}
	}
	return out
}

func (r *Renderer) pageLinks(paths []string, docURL string) []PageLink {
	out := make([]PageLink, 0, len(paths))
	for _, p := range paths {
		if page, ok := r.kb.Graph.Page(p); ok {
			out = append(out, PageLink{Title: page.Title(), URL: rel(docURL, PageURL(p)), Type: page.Type()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func (r *Renderer) pathsOfType(typ string) []string {
	var out []string
	for _, p := range r.kb.Graph.Paths() {
		if page, ok := r.kb.Graph.Page(p); ok && page.Type() == typ {
			out = append(out, p)
		}
	}
	return out
}

func (r *Renderer) pathsWithTag(norm string) []string {
	var out []string
	for _, p := range r.kb.Graph.Paths() {
		page, ok := r.kb.Graph.Page(p)
		if !ok {
			continue
		}
		for _, tag := range page.Tags() {
			if kb.Normalize(tag) == norm {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// typeNames returns the types that have pages, sorted. A type with no page
// gets no index, because an empty index is noise rather than navigation.
func (r *Renderer) typeNames() []string {
	return sortedKeys(r.typeCounts())
}

func (r *Renderer) typeCounts() map[string]int {
	counts := map[string]int{}
	for _, p := range r.kb.Graph.Paths() {
		if page, ok := r.kb.Graph.Page(p); ok {
			if typ := page.Type(); typ != "" {
				counts[typ]++
			}
		}
	}
	return counts
}

func (r *Renderer) typeLinks(docURL string) []TypeLink {
	counts := r.typeCounts()
	out := make([]TypeLink, 0, len(counts))
	for _, name := range sortedKeys(counts) {
		out = append(out, TypeLink{Name: name, URL: rel(docURL, TypeURL(name)), Count: counts[name]})
	}
	return out
}

// tagNames returns the normalised form of every tag in use, sorted.
func (r *Renderer) tagNames() []string {
	return sortedKeys(r.tagCounts())
}

// tagCounts counts pages per normalised tag, keeping the first spelling
// written, because the format keeps the author's spelling rather than the
// normalised form.
func (r *Renderer) tagCounts() map[string]int {
	counts := map[string]int{}
	for _, p := range r.kb.Graph.Paths() {
		page, ok := r.kb.Graph.Page(p)
		if !ok {
			continue
		}
		seen := map[string]bool{}
		for _, tag := range page.Tags() {
			n := kb.Normalize(tag)
			if n == "" || seen[n] {
				continue
			}
			seen[n] = true
			counts[n]++
		}
	}
	return counts
}

func (r *Renderer) tagLinks(docURL string) []TagLink {
	counts := r.tagCounts()
	out := make([]TagLink, 0, len(counts))
	for _, norm := range sortedKeys(counts) {
		out = append(out, TagLink{Name: r.tagSpelling(norm), URL: rel(docURL, TagURL(norm)), Count: counts[norm]})
	}
	return out
}

// tagSpelling returns the spelling an author first used for a normalised tag,
// so an index shows the prose rather than the slug.
func (r *Renderer) tagSpelling(norm string) string {
	for _, p := range r.kb.Graph.Paths() {
		page, ok := r.kb.Graph.Page(p)
		if !ok {
			continue
		}
		for _, tag := range page.Tags() {
			if kb.Normalize(tag) == norm {
				return tag
			}
		}
	}
	return norm
}

// sourceKeys returns the keys that are both defined and cited, which are the
// ones that get a source page.
func (r *Renderer) sourceKeys() []string {
	if r.kb.Bibliography == nil {
		return nil
	}
	var out []string
	for _, key := range r.kb.Bibliography.SortedKeys() {
		if len(r.kb.Graph.CitedBy(key)) > 0 {
			out = append(out, key)
		}
	}
	return out
}

// sourcePage materialises the virtual page for one key.
func (r *Renderer) sourcePage(key string) (*kb.Page, error) {
	if r.kb.Bibliography == nil {
		return nil, fmt.Errorf("no bibliography for %s", key)
	}
	entry, ok := r.kb.Bibliography.Entry(key)
	if !ok {
		return nil, fmt.Errorf("the bibliography has no entry for %s", key)
	}
	cited := r.kb.Graph.CitedBy(key)
	titles := make([]string, 0, len(cited))
	for _, p := range cited {
		if page, ok := r.kb.Graph.Page(p); ok {
			titles = append(titles, page.Title())
			continue
		}
		titles = append(titles, p)
	}
	return kb.SourcePage(entry, titles)
}

// tabTitle is what goes in the browser tab: the document's own title, and the
// site's when they differ.
func tabTitle(title, site string) string {
	switch {
	case title == "":
		return site
	case site == "", title == site:
		return title
	}
	return title + " · " + site
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// assetType is the media type of a static file, so a server sends a stylesheet
// as a stylesheet.
func assetType(name string) string {
	switch {
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

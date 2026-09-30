package render

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/url"
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

	// indexURL is the generated Index page: every page grouped by type, then
	// the tags. It is not "index.html" because that is the home page, the
	// authored entry document the whole site is rooted at.
	indexURL = "all.html"
)

// SiteData is what every document knows about the site around it.
type SiteData struct {
	Title    string
	HomeURL  string
	IndexURL string
	AssetURL string
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

// ReferenceData is one entry in a page's reference list.
type ReferenceData struct {
	Text string
}

// PageData is a page rendered as a whole document.
type PageData struct {
	common
	Title      string
	Meta       []MetaItem
	Body       template.HTML
	References []ReferenceData
	Backlinks  []PageLink
}

// ListingData is an index page: a heading and the pages under it, and on the
// Index page also the types and tags.
type ListingData struct {
	common
	Heading string
	Pages   []PageLink
	Types   []TypeLink
	Tags    []TagLink
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

func siteData(k *kb.KB) SiteData {
	home := indexURL
	if p := kb.PagesDir + "/index.md"; hasPage(k, p) {
		home = PageURL(p)
	}
	return SiteData{
		Title:    k.Manifest.Title,
		HomeURL:  home,
		IndexURL: indexURL,
		AssetURL: assetDir + "/style.css",
	}
}

func hasPage(k *kb.KB, p string) bool {
	_, ok := k.Graph.Page(p)
	return ok
}

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
func (r *Renderer) All() ([]byte, error) {
	data := ListingData{
		common:  common{Site: r.site, DocTitle: tabTitle("Index", r.site.Title)},
		Heading: "Index",
		Pages:   r.pageLinks(r.kb.Graph.Paths()),
		Types:   r.typeLinks(),
		Tags:    r.tagLinks(),
	}
	return r.execute("listing", data)
}

// TypeIndex renders the index of one type.
func (r *Renderer) TypeIndex(typ string) ([]byte, error) {
	data := ListingData{
		common:  common{Site: r.site, DocTitle: tabTitle(typ, r.site.Title)},
		Heading: typ,
		Pages:   r.pageLinks(r.pathsOfType(typ)),
	}
	return r.execute("listing", data)
}

// TagIndex renders the index of one tag, named by its normalised form.
func (r *Renderer) TagIndex(norm string) ([]byte, error) {
	data := ListingData{
		common:  common{Site: r.site, DocTitle: tabTitle(norm, r.site.Title)},
		Heading: "Tagged: " + norm,
		Pages:   r.pageLinks(r.pathsWithTag(norm)),
	}
	return r.execute("listing", data)
}

// Source renders the virtual source page for a citation key.
func (r *Renderer) Source(key string) ([]byte, error) {
	page, err := r.sourcePage(key)
	if err != nil {
		return nil, err
	}
	meta := []MetaItem{{Text: kb.TypeSource}, {Label: "key", Text: page.Key()}}
	data := PageData{
		common: common{Site: r.site, DocTitle: tabTitle(page.Title(), r.site.Title)},
		Title:  page.Title(),
		Meta:   meta,
		Body:   template.HTML(r.bodyHTML(page, nil)),
	}
	return r.execute("page", data)
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
		body, err := r.Page(p)
		if err != nil {
			return nil, err
		}
		addHTML(PageURL(p), body)
	}

	body, err := r.All()
	if err != nil {
		return nil, err
	}
	addHTML(indexURL, body)

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
	return PageData{
		common:     common{Site: r.site, DocTitle: tabTitle(page.Title(), r.site.Title)},
		Title:      page.Title(),
		Meta:       r.meta(page),
		Body:       template.HTML(r.bodyHTML(page, refs)),
		References: r.referenceData(refs),
		Backlinks:  r.backlinks(pagePath),
	}
}

// meta is the line under a page's title: what the frontmatter says, in the
// order a reader would ask for it. A page with only a title and a type shows
// only those.
func (r *Renderer) meta(page *kb.Page) []MetaItem {
	var out []MetaItem
	if typ := page.Type(); typ != "" {
		out = append(out, MetaItem{Text: typ, URL: TypeURL(typ)})
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
		out = append(out, MetaItem{Label: label, Text: tag, URL: TagURL(tag)})
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

func (r *Renderer) referenceData(refs []kb.Reference) []ReferenceData {
	out := make([]ReferenceData, 0, len(refs))
	for i, ref := range refs {
		out = append(out, ReferenceData{Text: r.formatter.Entry(ref, i+1)})
	}
	return out
}

func (r *Renderer) backlinks(pagePath string) []PageLink {
	paths := r.kb.Graph.Backlinks(pagePath)
	out := make([]PageLink, 0, len(paths))
	for _, p := range paths {
		if page, ok := r.kb.Graph.Page(p); ok {
			out = append(out, PageLink{Title: page.Title(), URL: PageURL(p), Type: page.Type()})
		}
	}
	return out
}

func (r *Renderer) pageLinks(paths []string) []PageLink {
	out := make([]PageLink, 0, len(paths))
	for _, p := range paths {
		if page, ok := r.kb.Graph.Page(p); ok {
			out = append(out, PageLink{Title: page.Title(), URL: PageURL(p), Type: page.Type()})
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

func (r *Renderer) typeLinks() []TypeLink {
	counts := r.typeCounts()
	out := make([]TypeLink, 0, len(counts))
	for _, name := range sortedKeys(counts) {
		out = append(out, TypeLink{Name: name, URL: TypeURL(name), Count: counts[name]})
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

func (r *Renderer) tagLinks() []TagLink {
	counts := r.tagCounts()
	out := make([]TagLink, 0, len(counts))
	for _, norm := range sortedKeys(counts) {
		out = append(out, TagLink{Name: r.tagSpelling(norm), URL: TagURL(norm), Count: counts[norm]})
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

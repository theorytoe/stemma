package render

import (
	"fmt"
	"html/template"
	"sort"

	"github.com/theorytoe/stemma/internal/kb"
)

// homeBlockLimit is how many items a landing-page block lists before it stops.
// The home page is a way in, not a report; each block is here to say what kind
// of thing the KB holds and where to read the whole of it, and the whole of it is
// one link away.
const homeBlockLimit = 5

// Count is one number in the landing page's header, and where to read more.
type Count struct {
	Text string
	URL  string
}

// SourceLink is one source on the landing page: the entry, its date, and the
// pages that lean on it.
type SourceLink struct {
	Title     string
	URL       string
	Year      string
	Retrieved string
	CitedBy   []PageLink
}

// Attention is one kind of thing a KB would like fixed, in the words FORMAT.md
// names it by, with how many there are and which pages they are about.
type Attention struct {
	Text  string
	Count int
	Pages []PageLink
}

// HomeData is the landing page: what the KB is, an opening, and the blocks that
// are counted from what it holds.
type HomeData struct {
	common

	// Heading is the entry document's title when the KB has one, and the KB's
	// title when it does not.
	Heading string

	// Description is the manifest's, when it has one.
	Description string

	// Counts is the header line: pages, sources, types, tags.
	Counts []Count

	// Opening is the entry document's body. It is the one part of the page a
	// program did not write, and it is empty when the KB has no entry document.
	Opening template.HTML

	Types     []TypeLink
	Tags      []TagLink
	Sources   []SourceLink
	Attention []Attention
}

// Home renders the landing page: what the KB holds, in one screen.
//
// The page is generated, and only its opening is not. Everything else is counted
// from the KB, so it cannot go stale, and nothing on it needed an editor: the
// counts, the types, the tags, the newest sources and the things to fix are all
// consequences of what the KB contains. The opening is the exception on purpose —
// the entry document's own body — because "where should I start" is judgement,
// and a list of pages with a reason to read each is the one thing a program
// cannot produce. Wikipedia's Main Page works the same way: an authored page
// whose blocks are generated.
//
// A KB without an entry document gets the generated page and nothing else, which
// is why that page is optional rather than structural. Deleting it is a
// supported thing to do, not a broken KB.
func (r *Renderer) Home() ([]byte, error) {
	docURL := homeURL()
	data := HomeData{
		common:      common{Site: r.siteFor(docURL), DocTitle: tabTitle("", r.title)},
		Heading:     r.title,
		Description: r.kb.Manifest.Description,
		Counts:      r.counts(docURL),
		Types:       topTypes(r.typeLinks(docURL)),
		Tags:        topTags(r.tagLinks(docURL)),
		Sources:     r.newestSources(docURL),
		Attention:   r.attention(docURL),
	}
	if page, ok := r.kb.Graph.Page(kb.EntryDocument); ok {
		if title := page.Title(); title != "" {
			data.Heading = title
		}
		refs, _ := r.kb.References(kb.EntryDocument)
		data.Opening = template.HTML(r.bodyHTML(page, refs, docURL))
	}
	return r.execute("home", data)
}

// counts is the header line: what the KB holds, and where to read the list.
func (r *Renderer) counts(docURL string) []Count {
	out := []Count{{Text: plural(len(r.kb.Graph.Paths()), "page"), URL: rel(docURL, indexURL)}}
	if n := r.sourceCount(); n > 0 {
		out = append(out, Count{Text: plural(n, "source")})
	}
	if n := len(r.typeCounts()); n > 0 {
		out = append(out, Count{Text: plural(n, "type"), URL: rel(docURL, typesURL)})
	}
	if n := len(r.tagCounts()); n > 0 {
		out = append(out, Count{Text: plural(n, "tag"), URL: rel(docURL, tagsURL)})
	}
	return out
}

// sourceCount is how many sources the bibliography defines.
func (r *Renderer) sourceCount() int {
	if r.kb.Bibliography == nil {
		return 0
	}
	return r.kb.Bibliography.Len()
}

// newestSources is what the bibliography can say about recency: the sources with
// a retrieval date, newest first, and the pages that lean on them.
//
// It is the only recency a KB has. Pages carry no dates at all — the format
// leaves "when" to git — so a "recently changed" list would be a lie the first
// time a KB was cloned or untarred, when every file arrives with the same
// timestamp. A source is different: it is read from somewhere on a day, and
// `cite add` and `fetch` write that day into the entry, where it travels with the
// KB.
//
// Only cited sources are here, because only they have a page to link to. A source
// nothing cites is a thing to fix, and it is reported as one.
func (r *Renderer) newestSources(docURL string) []SourceLink {
	if r.kb.Bibliography == nil {
		return nil
	}
	var out []SourceLink
	for _, key := range r.kb.Bibliography.SortedKeys() {
		cited := r.kb.Graph.CitedBy(key)
		if len(cited) == 0 {
			continue
		}
		entry, ok := r.kb.Bibliography.Entry(key)
		if !ok {
			continue
		}
		retrieved, _ := entry.Value(kb.FieldRetrieved)
		out = append(out, SourceLink{
			Title:     entry.Title(),
			URL:       rel(docURL, SourceURL(key)),
			Year:      entry.Year(),
			Retrieved: retrieved,
			CitedBy:   r.pageLinks(cited, docURL),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Retrieved != out[j].Retrieved {
			return out[i].Retrieved > out[j].Retrieved
		}
		return out[i].Title < out[j].Title
	})
	if len(out) > homeBlockLimit {
		out = out[:homeBlockLimit]
	}
	return out
}

// attention is the landing page's maintenance block: what lint would report,
// gathered by what kind of thing it is.
//
// It is the one block about the KB rather than in it, and the reason the page is
// useful to an author and not only to a reader. It renders as nothing when there
// is nothing to fix, so a clean KB has no such block at all — which is the state
// the example wiki is kept in, and the reason this can sit on a published page
// without publishing a to-do list.
func (r *Renderer) attention(docURL string) []Attention {
	byCode := map[string]*Attention{}
	var codes []string
	for _, f := range r.kb.Lint(kb.Lenient) {
		a := byCode[f.Code]
		if a == nil {
			a = &Attention{Text: attentionText(f.Code)}
			byCode[f.Code] = a
			codes = append(codes, f.Code)
		}
		a.Count++
		if page, ok := r.kb.Graph.Page(f.Path); ok {
			a.Pages = append(a.Pages, PageLink{Title: page.Title(), URL: rel(docURL, PageURL(f.Path))})
		}
	}
	sort.Strings(codes)
	out := make([]Attention, 0, len(codes))
	for _, code := range codes {
		a := byCode[code]
		sort.SliceStable(a.Pages, func(i, j int) bool { return a.Pages[i].Title < a.Pages[j].Title })
		if len(a.Pages) > homeBlockLimit {
			a.Pages = a.Pages[:homeBlockLimit]
		}
		out = append(out, *a)
	}
	return out
}

// attentionText names a finding the way FORMAT.md names it, so the page and the
// document that specifies the rule cannot drift into saying different things. A
// code with no phrase yet is shown as itself rather than hidden: a reader seeing
// "vendored-drift" is told something true, and whoever adds a code gets a visible
// nudge to describe it.
func attentionText(code string) string {
	switch code {
	case kb.CodeMissingField:
		return "pages missing a title or a type"
	case kb.CodeMalformedField:
		return "fields holding the wrong shape for their value"
	case kb.CodeUnknownType:
		return "pages with a type the vocabulary does not know"
	case kb.CodeInvalidStatus:
		return "pages with an invalid status"
	case kb.CodeReservedType:
		return "pages declaring a type the tool owns"
	case kb.CodeReservedField:
		return "pages carrying a field the tool owns"
	case kb.CodeDuplicateTag:
		return "pages listing the same tag twice"
	case kb.CodeNameCollision:
		return "names more than one page claims"
	case kb.CodeUnresolvedLink:
		return "wikilinks that name nothing"
	case kb.CodeAmbiguousLink:
		return "wikilinks that name more than one page"
	case kb.CodeOrphanPage:
		return "pages nothing links to"
	case kb.CodeCitationMissing:
		return "cited keys the bibliography does not define"
	case kb.CodeCitationUncited:
		return "sources nothing cites"
	case kb.CodeCitationDuplicate:
		return "citation keys the bibliography defines twice"
	default:
		return code
	}
}

// topTypes is the types with the most pages, busiest first: a landing-page block
// shows a handful, and the handful worth showing is the one a reader is most
// likely to find something in. The whole list is one link away.
func topTypes(links []TypeLink) []TypeLink {
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].Count != links[j].Count {
			return links[i].Count > links[j].Count
		}
		return links[i].Name < links[j].Name
	})
	return firstN(links)
}

// topTags is the tags with the most pages, busiest first.
func topTags(links []TagLink) []TagLink {
	sort.SliceStable(links, func(i, j int) bool {
		if links[i].Count != links[j].Count {
			return links[i].Count > links[j].Count
		}
		return links[i].Name < links[j].Name
	})
	return firstN(links)
}

// firstN keeps as much of a list as a landing-page block shows.
func firstN[T any](xs []T) []T {
	if len(xs) > homeBlockLimit {
		return xs[:homeBlockLimit]
	}
	return xs
}

// plural writes a number and its noun, singular or plural.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

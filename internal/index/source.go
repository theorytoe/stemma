package index

import (
	"sort"

	"github.com/theorytoe/stemma/internal/kb"
)

// Tier says which of the two retrieval sources answered a question.
type Tier int

const (
	// TierKB is Tier 0, the KB read directly. It needs no setup.
	TierKB Tier = iota
	// TierIndex is Tier 1, the SQLite cache.
	TierIndex
)

func (t Tier) String() string {
	if t == TierIndex {
		return "index"
	}
	return "kb"
}

// PageMeta is a page as retrieval sees it for filtering and display. Its tags
// are normalised, because that is the form a filter compares (D63); the
// author's spelling lives in the page and is not retrieval's to keep.
type PageMeta struct {
	Path   string
	Title  string
	Type   string
	Status string
	Tags   []string
}

// CitationRef is a citation as retrieval needs it: the key and where it was
// written. The syntax around a key is the renderer's to read, and a citation
// map only needs what identifies the citation.
type CitationRef struct {
	Key  string
	Line int
}

// Source is the retrieval interface that both tiers implement.
//
// It is the data half of retrieval, not the algorithm half: a Source says what
// the corpus holds and how to find candidates, and the search and graph
// functions in this package are written once against it. That is what makes the
// two tiers agree by construction rather than by coincidence — there is no
// second implementation of ranking or traversal to drift from the first.
//
// The methods are deliberately small. A Source that answered "search" would be
// free to rank differently on each tier, which is the failure the parity tests
// exist to catch.
type Source interface {
	// Tier reports which tier this is.
	Tier() Tier

	// Pages returns every page's metadata, sorted by path.
	Pages() ([]PageMeta, error)

	// Doc returns a page's tokenized fields. A path that is not in the corpus
	// yields a zero Doc and no error.
	Doc(path string) (Doc, error)

	// Body returns a page's markdown body, which is what a snippet is cut from.
	Body(path string) (string, error)

	// Match returns the paths whose tokens include any of the terms, sorted.
	Match(terms []string) ([]string, error)

	// DF returns how many pages contain a term.
	DF(term string) (int, error)

	// AvgLength is the mean weighted document length, or 0 for an empty corpus.
	AvgLength() (float64, error)

	// Claimants returns the pages answering to a name, sorted.
	Claimants(name string) ([]string, error)

	// Links returns a page's outgoing links, in order.
	Links(path string) ([]kb.Link, error)

	// Backlinks returns the pages linking to a page, sorted, excluding the page
	// itself.
	Backlinks(path string) ([]string, error)

	// Citations returns a page's citations, in order.
	Citations(path string) ([]CitationRef, error)

	// CitedBy returns the pages citing a key, sorted.
	CitedBy(key string) ([]string, error)

	// Orphans returns the pages nothing links to, sorted. Index and source
	// pages are left out, and so are archived pages, matching lint's rule.
	Orphans() ([]string, error)

	// DeadEnds returns the pages with no outgoing links, sorted. Source pages
	// are left out, and so are archived pages.
	DeadEnds() ([]string, error)

	// Close releases whatever the source holds open.
	Close() error
}

// NewSource returns the best source for a KB: the index when it is fresh, the
// KB itself otherwise.
//
// A missing, stale or unreadable index is never an error. Every command must
// work without one (D23, P1), so a downgrade changes only which source answers.
// onDowngrade, when it is not nil, is called at most once with the reason the
// index was not used: the fallback is silent by default, and a caller that wants
// a diagnostic gets one line, never one per query.
func NewSource(k *kb.KB, onDowngrade func(reason string)) Source {
	state, err := CheckKB(k)
	if err == nil && state == Fresh {
		if store, openErr := openReadOnly(k.Root); openErr == nil && store != nil {
			return newIndexSource(store)
		}
	}
	if onDowngrade != nil {
		onDowngrade(downgradeReason(state, err))
	}
	return newKBSource(k)
}

// NewSourceAt is NewSource for a caller that has only a KB root.
//
// It is the form a command uses. When the index is fresh no page is parsed: the
// freshness check reads and hashes the page bytes, and the index answers from
// there. When there is no usable index the KB is loaded, which is the cost Tier
// 0 always pays and the cost the index exists to avoid.
func NewSourceAt(root string, onDowngrade func(reason string)) (Source, error) {
	state, err := Check(root)
	if err == nil && state == Fresh {
		if store, openErr := openReadOnly(root); openErr == nil && store != nil {
			return newIndexSource(store), nil
		}
	}
	k, loadErr := kb.Load(root)
	if loadErr != nil {
		return nil, loadErr
	}
	if onDowngrade != nil {
		onDowngrade(downgradeReason(state, err))
	}
	return newKBSource(k), nil
}

// downgradeReason names why the index did not answer, in the words the index
// state uses.
func downgradeReason(state State, err error) string {
	switch {
	case err != nil:
		return "unreadable"
	case state == Absent:
		return "absent"
	default:
		return state.String()
	}
}

// MakeDoc tokenizes a document's fields the way both tiers do. It is exported
// for the callers that supply documents the source does not hold, such as inbox
// drafts, so those rank on the same tokens as a page.
func MakeDoc(title string, tags []string, body string) Doc {
	return Doc{Title: Tokenize(title), Tags: tagTokens(tags), Body: Tokenize(body)}
}

// Document is one searchable item supplied outside a source — an inbox draft.
// Drafts are read directly rather than indexed (D49), so they are handed to
// Search alongside the pages and ranked with them.
type Document struct {
	Meta PageMeta
	Doc  Doc
	Body string
}

// NewDocument builds a searchable document from raw fields, normalising its
// tags and tokenizing its text the same way a page's are.
func NewDocument(path, title, typ, status string, tags []string, body string) Document {
	return Document{
		Meta: PageMeta{Path: path, Title: title, Type: typ, Status: status, Tags: normalTags(tags)},
		Doc:  MakeDoc(title, tags, body),
		Body: body,
	}
}

// metaOf describes a page for retrieval, normalising its tags.
func metaOf(path string, page *kb.Page) PageMeta {
	return PageMeta{
		Path:   path,
		Title:  page.Title(),
		Type:   page.Type(),
		Status: page.Status(),
		Tags:   normalTags(page.Tags()),
	}
}

// normalTags returns the normalised, de-duplicated, sorted tags of a page. It
// returns nil rather than an empty slice when there are none, so that a page
// with no tags is represented the same way whichever tier produced it.
func normalTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		n := kb.Normalize(tag)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// tagTokens tokenizes a page's tags the way both tiers do: by their normalised
// form, so the ranking sees the same tokens whichever tier produced the Doc.
func tagTokens(tags []string) []string {
	var out []string
	for _, tag := range tags {
		if n := kb.Normalize(tag); n != "" {
			out = append(out, Tokenize(n)...)
		}
	}
	return out
}

// termSet turns a query's tokens into a set.
func termSet(terms []string) map[string]bool {
	out := make(map[string]bool, len(terms))
	for _, t := range terms {
		if t != "" {
			out[t] = true
		}
	}
	return out
}

// docHasAny reports whether any of a document's tokens is in want.
func docHasAny(d Doc, want map[string]bool) bool {
	for _, field := range [][]string{d.Title, d.Tags, d.Body} {
		for _, t := range field {
			if want[t] {
				return true
			}
		}
	}
	return false
}

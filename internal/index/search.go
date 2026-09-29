package index

import (
	"sort"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// SearchRequest is one full-text query with the filters that narrow it.
type SearchRequest struct {
	// Query is the text as typed. It is tokenized by the same tokenizer the
	// index was built with, so a query cannot match a token the corpus does not
	// have.
	Query string

	// Type, Status and Tags narrow the results. Tags are compared by their
	// normalised form, and every wanted tag must be present.
	Type   string
	Status string
	Tags   []string

	// Dir is a KB-relative directory the pages must be under. It is a
	// convenience for narrowing a listing and never a semantic boundary (P10):
	// it filters, it does not scope identity or resolution.
	Dir string

	// Limit caps the number of results. Zero means no cap.
	Limit int
}

// Hit is one search result, ranked and ready to print.
type Hit struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Type    string   `json:"type"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags,omitempty"`
	Score   float64  `json:"score"`
	Snippet string   `json:"snippet,omitempty"`
}

// Search ranks a source's pages, together with any extra documents, against a
// query.
//
// The extra documents are inbox drafts, which are outside the source because
// they are outside the knowledge proper (D49); passing them in lets one query
// reach both without giving the index a second, subtly different notion of what
// a document is.
//
// Ranking is BM25 over the whole corpus, then the filters, then the cap. The
// corpus statistics come from the source, so both tiers score against the same
// numbers — an extra document shifts the average for both, not one.
func Search(src Source, extra []Document, req SearchRequest) ([]Hit, error) {
	terms := Tokenize(req.Query)
	extraByPath := make(map[string]Document, len(extra))
	for _, d := range extra {
		extraByPath[d.Meta.Path] = d
	}

	pages, err := src.Pages()
	if err != nil {
		return nil, err
	}
	stats, err := corpusStats(src, pages, extra, terms)
	if err != nil {
		return nil, err
	}

	meta := make(map[string]PageMeta, len(pages)+len(extra))
	for _, m := range pages {
		meta[m.Path] = m
	}
	for path, d := range extraByPath {
		meta[path] = d.Meta
	}

	candidates, err := candidatesOf(src, terms, pages, extra)
	if err != nil {
		return nil, err
	}
	want := termSet(terms)

	hits := make([]Hit, 0, len(candidates))
	for _, path := range candidates {
		m, ok := meta[path]
		if !ok || !passes(m, req) {
			continue
		}

		doc, body, err := docAndBody(src, extraByPath, path)
		if err != nil {
			return nil, err
		}
		hit := Hit{
			Path:   m.Path,
			Title:  m.Title,
			Type:   m.Type,
			Status: m.Status,
			Tags:   m.Tags,
			Score:  Score(terms, doc, stats),
		}
		if len(want) > 0 {
			hit.Snippet = Snippet(body, terms, 0)
		}
		hits = append(hits, hit)
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
	if req.Limit > 0 && len(hits) > req.Limit {
		hits = hits[:req.Limit]
	}
	return hits, nil
}

// candidatesOf returns the paths that might match, before filtering: the source
// matches plus the extra documents that contain a term. An empty query means
// "everything", so a filter-only search lists rather than ranks.
func candidatesOf(src Source, terms []string, pages []PageMeta, extra []Document) ([]string, error) {
	if len(terms) == 0 {
		out := make([]string, 0, len(pages)+len(extra))
		for _, m := range pages {
			out = append(out, m.Path)
		}
		for _, d := range extra {
			out = append(out, d.Meta.Path)
		}
		return out, nil
	}

	out, err := src.Match(terms)
	if err != nil {
		return nil, err
	}
	want := termSet(terms)
	for _, d := range extra {
		if docHasAny(d.Doc, want) {
			out = append(out, d.Meta.Path)
		}
	}
	return out, nil
}

// docAndBody returns a candidate's tokenized fields and raw body, from the
// extra documents when it is one of them and from the source otherwise.
func docAndBody(src Source, extra map[string]Document, path string) (Doc, string, error) {
	if d, ok := extra[path]; ok {
		return d.Doc, d.Body, nil
	}
	doc, err := src.Doc(path)
	if err != nil {
		return Doc{}, "", err
	}
	body, err := src.Body(path)
	if err != nil {
		return Doc{}, "", err
	}
	return doc, body, nil
}

// corpusStats builds the BM25 statistics over the pages and the extra
// documents together.
func corpusStats(src Source, pages []PageMeta, extra []Document, terms []string) (Stats, error) {
	n := len(pages) + len(extra)
	avg, err := src.AvgLength()
	if err != nil {
		return Stats{}, err
	}
	total := avg * float64(len(pages))
	for _, d := range extra {
		total += d.Doc.Length()
	}
	mean := 0.0
	if n > 0 {
		mean = total / float64(n)
	}

	df := make(map[string]int, len(terms))
	seen := map[string]bool{}
	for _, t := range terms {
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		count, err := src.DF(t)
		if err != nil {
			return Stats{}, err
		}
		df[t] = count
	}
	for _, d := range extra {
		have := docTerms(d.Doc)
		for t := range seen {
			if have[t] {
				df[t]++
			}
		}
	}
	return Stats{N: n, AvgLen: mean, DF: df}, nil
}

// passes reports whether a page survives a request's filters.
func passes(m PageMeta, req SearchRequest) bool {
	if req.Type != "" && m.Type != req.Type {
		return false
	}
	if req.Status != "" && m.Status != req.Status {
		return false
	}
	if req.Dir != "" && m.Path != req.Dir && !strings.HasPrefix(m.Path, req.Dir+"/") {
		return false
	}
	return carriesEvery(m.Tags, req.Tags)
}

// carriesEvery reports whether each wanted tag is present, comparing the
// normalised forms so a spelling difference is not a miss.
func carriesEvery(have, want []string) bool {
	for _, w := range want {
		n := kb.Normalize(w)
		if n == "" {
			continue
		}
		found := false
		for _, h := range have {
			if h == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

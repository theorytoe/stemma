package index

import (
	"github.com/theorytoe/stemma/internal/kb"
)

// kbSource is Tier 0: the KB itself, tokenized once in memory.
//
// It reads every page at construction, which is the tier's whole character:
// there is no setup and no cache, so the corpus is the corpus in memory. A
// command that reaches for it has already paid the cost of loading the KB.
type kbSource struct {
	k     *kb.KB
	pages []PageMeta
	docs  map[string]Doc
	df    map[string]int
	avg   float64
}

func newKBSource(k *kb.KB) *kbSource {
	s := &kbSource{k: k, docs: map[string]Doc{}, df: map[string]int{}}
	total := 0.0
	for _, path := range k.Graph.Paths() {
		page, ok := k.Graph.Page(path)
		if !ok {
			continue
		}
		doc := Doc{
			Title: Tokenize(page.Title()),
			Tags:  tagTokens(page.Tags()),
			Body:  Tokenize(string(page.Body())),
		}
		s.docs[path] = doc
		s.pages = append(s.pages, metaOf(path, page))
		total += doc.Length()
		for term := range docTerms(doc) {
			s.df[term]++
		}
	}
	if len(s.pages) > 0 {
		s.avg = total / float64(len(s.pages))
	}
	return s
}

func (s *kbSource) Tier() Tier { return TierKB }

func (s *kbSource) Pages() ([]PageMeta, error) { return s.pages, nil }

func (s *kbSource) Doc(path string) (Doc, error) { return s.docs[path], nil }

func (s *kbSource) Body(path string) (string, error) {
	page, ok := s.k.Graph.Page(path)
	if !ok {
		return "", nil
	}
	return string(page.Body()), nil
}

func (s *kbSource) Match(terms []string) ([]string, error) {
	want := termSet(terms)
	if len(want) == 0 {
		return nil, nil
	}
	var out []string
	for _, m := range s.pages {
		if docHasAny(s.docs[m.Path], want) {
			out = append(out, m.Path)
		}
	}
	return out, nil
}

func (s *kbSource) DF(term string) (int, error) { return s.df[term], nil }

func (s *kbSource) AvgLength() (float64, error) { return s.avg, nil }

func (s *kbSource) Claimants(name string) ([]string, error) {
	return s.k.Graph.Claimants(name), nil
}

func (s *kbSource) Links(path string) ([]kb.Link, error) {
	return s.k.Graph.Links(path), nil
}

func (s *kbSource) Backlinks(path string) ([]string, error) {
	return s.k.Graph.Backlinks(path), nil
}

func (s *kbSource) Citations(path string) ([]CitationRef, error) {
	cites := s.k.Graph.Citations(path)
	out := make([]CitationRef, 0, len(cites))
	for _, c := range cites {
		out = append(out, CitationRef{Key: c.Key, Line: c.Line})
	}
	return out, nil
}

func (s *kbSource) CitedBy(key string) ([]string, error) {
	return s.k.Graph.CitedBy(key), nil
}

func (s *kbSource) Close() error { return nil }

// docTerms returns the distinct tokens in a document.
func docTerms(d Doc) map[string]bool {
	out := map[string]bool{}
	for _, field := range [][]string{d.Title, d.Tags, d.Body} {
		for _, t := range field {
			out[t] = true
		}
	}
	return out
}

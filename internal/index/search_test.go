package index

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

func searchKB(t *testing.T) (*kb.KB, *Store) {
	t.Helper()
	k := buildKB(t, map[string]string{
		"pages/index.md":      pageFile("Index", "type: index", "see [[Alpha]]\n"),
		"pages/retrieval.md":  pageFile("Retrieval", "type: concept\ntags: [Machine-Learning]", "a short page\n"),
		"pages/alpha.md":      pageFile("Alpha", "type: concept\ntags: [machine-learning, graphs]", "retrieval retrieval retrieval and more text\n"),
		"pages/beta.md":       pageFile("Beta", "type: note", "nothing of note, but archived later\n"),
		"pages/deep/gamma.md": pageFile("Gamma", "type: concept", "a retrieval page\n"),
	})
	store := open(t, k.Root)
	if _, err := store.Populate(k); err != nil {
		t.Fatal(err)
	}
	return k, store
}

func pathsOf(hits []Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Path
	}
	return out
}

func TestSearchRanksTitleBeforeBody(t *testing.T) {
	k, store := searchKB(t)
	for _, src := range []Source{newKBSource(k), newIndexSource(store)} {
		hits, err := Search(src, nil, SearchRequest{Query: "retrieval"})
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 3 {
			t.Fatalf("tier %v: %d hits, want 3: %v", src.Tier(), len(hits), pathsOf(hits))
		}
		if hits[0].Path != "pages/retrieval.md" {
			t.Errorf("tier %v: top hit = %s, want pages/retrieval.md", src.Tier(), hits[0].Path)
		}

		// A snippet is cut from the body, so a title-only match has none and a
		// body match is marked.
		var alpha *Hit
		for i := range hits {
			if hits[i].Path == "pages/alpha.md" {
				alpha = &hits[i]
			}
		}
		if alpha == nil || !strings.Contains(alpha.Snippet, "[retrieval]") {
			t.Errorf("tier %v: body snippet = %v, want the match marked", src.Tier(), alpha)
		}
		if hits[0].Snippet != "" {
			t.Errorf("tier %v: a title-only match has a body snippet %q", src.Tier(), hits[0].Snippet)
		}
	}
}

// A query returns the same pages, in the same order, whichever tier answers.
func TestSearchAgreesAcrossTiers(t *testing.T) {
	k, store := searchKB(t)
	for _, req := range []SearchRequest{
		{Query: "retrieval"},
		{Query: "retrieval", Type: "concept"},
		{Query: "retrieval", Tags: []string{"machine-learning"}},
		{Query: "retrieval", Dir: "pages/deep"},
		{Query: "retrieval", Limit: 1},
		{Query: ""},
		{Query: "notes"},
	} {
		a, err := Search(newKBSource(k), nil, req)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Search(newIndexSource(store), nil, req)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(pathsOf(a), pathsOf(b)) {
			t.Errorf("request %+v: tiers disagree: %v vs %v", req, pathsOf(a), pathsOf(b))
		}
		for i := range a {
			if math.Abs(a[i].Score-b[i].Score) > 1e-9 {
				t.Errorf("request %+v: score for %s differs: %v vs %v", req, a[i].Path, a[i].Score, b[i].Score)
			}
		}
	}
}

func TestSearchFilters(t *testing.T) {
	k, _ := searchKB(t)
	src := newKBSource(k)
	for _, tc := range []struct {
		name string
		req  SearchRequest
		want []string
	}{
		{"by type", SearchRequest{Query: "retrieval", Type: "concept"}, []string{"pages/retrieval.md", "pages/alpha.md", "pages/deep/gamma.md"}},
		{"by tag", SearchRequest{Query: "retrieval", Tags: []string{"Machine-Learning"}}, []string{"pages/retrieval.md", "pages/alpha.md"}},
		{"two tags are required together", SearchRequest{Query: "retrieval", Tags: []string{"machine-learning", "graphs"}}, []string{"pages/alpha.md"}},
		{"by directory", SearchRequest{Query: "retrieval", Dir: "pages/deep"}, []string{"pages/deep/gamma.md"}},
		{"by status", SearchRequest{Query: "retrieval", Status: "archived"}, nil},
		{"a filter alone lists", SearchRequest{Type: "note"}, []string{"pages/beta.md"}},
		{"limit", SearchRequest{Query: "retrieval", Limit: 2}, []string{"pages/retrieval.md", "pages/alpha.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := Search(src, nil, tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if got := pathsOf(hits); !reflect.DeepEqual(got, nonNilStrings(tc.want)) {
				t.Errorf("hits = %v, want %v", got, tc.want)
			}
		})
	}
}

// Drafts are outside the source, so they arrive as extra documents and are
// ranked with the pages.
func TestSearchReachesDrafts(t *testing.T) {
	k, _ := searchKB(t)
	draft := NewDocument("inbox/idea.md", "An Idea", "concept", "active", []string{"sketch"}, "a draft about retrieval\n")
	hits, err := Search(newKBSource(k), []Document{draft}, SearchRequest{Query: "retrieval"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range hits {
		if h.Path == "inbox/idea.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("the draft was not searched: %v", pathsOf(hits))
	}

	// Its tags filter like a page's.
	hits, err = Search(newKBSource(k), []Document{draft}, SearchRequest{Tags: []string{"sketch"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := pathsOf(hits); !reflect.DeepEqual(got, []string{"inbox/idea.md"}) {
		t.Errorf("filter by a draft tag = %v", got)
	}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

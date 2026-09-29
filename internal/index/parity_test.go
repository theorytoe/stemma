package index

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// Tier parity is the property the index is built to preserve: a query returns
// the same answer whether it is answered from the KB or from the cache. These
// tests run every query the retrieval surface has against both tiers and assert
// they agree, on synthetic corpora and on the project's own example wiki.

func TestTierParityOnSyntheticCorpora(t *testing.T) {
	for _, n := range []int{1, 5, 40} {
		t.Run(fmt.Sprintf("pages=%d", n), func(t *testing.T) {
			dir := t.TempDir()
			k := synthParityKB(t, dir, n)
			store := open(t, dir)
			if _, err := store.Populate(k); err != nil {
				t.Fatal(err)
			}
			assertTierParity(t, k, store)
		})
	}
}

// The example wiki is the real corpus: pages with aliases, tags, links,
// citations and unresolved links, which is where a disagreement would hide.
func TestTierParityOnTheExampleWiki(t *testing.T) {
	dir := exampleWiki(t)
	k, err := kb.Load(dir)
	if err != nil {
		t.Fatalf("load example wiki: %v", err)
	}
	store := open(t, dir)
	if _, err := store.Populate(k); err != nil {
		t.Fatal(err)
	}
	assertTierParity(t, k, store)
}

// assertTierParity runs the whole retrieval surface against both tiers.
func assertTierParity(t *testing.T, k *kb.KB, store *Store) {
	t.Helper()
	a, b := newKBSource(k), newIndexSource(store)

	pa, err := a.Pages()
	if err != nil {
		t.Fatal(err)
	}
	pb, err := b.Pages()
	if err != nil {
		t.Fatal(err)
	}
	if !sameSlice(pa, pb) {
		t.Fatalf("Pages differ:\n kb  = %+v\n idx = %+v", pa, pb)
	}

	for _, m := range pa {
		if da, db := docOf(t, a, m.Path), docOf(t, b, m.Path); !reflect.DeepEqual(da, db) {
			t.Errorf("Doc(%s) differs:\n kb  = %+v\n idx = %+v", m.Path, da, db)
		}
		if la, lb := linksOf(t, a, m.Path), linksOf(t, b, m.Path); !sameSlice(la, lb) {
			t.Errorf("Links(%s) differ: %+v vs %+v", m.Path, la, lb)
		}
		if ba, bb := backlinksOf(t, a, m.Path), backlinksOf(t, b, m.Path); !sameSlice(ba, bb) {
			t.Errorf("Backlinks(%s) differ: %v vs %v", m.Path, ba, bb)
		}
		if ca, cb := citesOf(t, a, m.Path), citesOf(t, b, m.Path); !sameSlice(ca, cb) {
			t.Errorf("Citations(%s) differ: %+v vs %+v", m.Path, ca, cb)
		}
	}

	for _, req := range paritySearches(pa) {
		ha, err := Search(a, nil, req)
		if err != nil {
			t.Fatal(err)
		}
		hb, err := Search(b, nil, req)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(pathsOf(ha), pathsOf(hb)) {
			t.Errorf("Search(%+v) differs: %v vs %v", req, pathsOf(ha), pathsOf(hb))
			continue
		}
		for i := range ha {
			if math.Abs(ha[i].Score-hb[i].Score) > 1e-9 {
				t.Errorf("Search(%+v): score for %s differs: %v vs %v", req, ha[i].Path, ha[i].Score, hb[i].Score)
			}
		}
	}

	for _, m := range pa {
		for _, dirs := range [][]Direction{{Out}, {In}, {In, Out}} {
			for _, depth := range []int{0, 1, 2} {
				na, err := Neighbors(a, m.Path, dirs, depth)
				if err != nil {
					t.Fatal(err)
				}
				nb, err := Neighbors(b, m.Path, dirs, depth)
				if err != nil {
					t.Fatal(err)
				}
				if !sameSlice(na, nb) {
					t.Errorf("Neighbors(%s, %v, %d) differs: %+v vs %+v", m.Path, dirs, depth, na, nb)
				}
			}
		}
	}

	oa, _ := a.Orphans()
	ob, err := b.Orphans()
	if err != nil {
		t.Fatal(err)
	}
	if !sameSlice(oa, ob) {
		t.Errorf("Orphans differ: %v vs %v", oa, ob)
	}
	da, _ := a.DeadEnds()
	db, err := b.DeadEnds()
	if err != nil {
		t.Fatal(err)
	}
	if !sameSlice(da, db) {
		t.Errorf("DeadEnds differ: %v vs %v", da, db)
	}

	for _, name := range parityNames(k) {
		ca, _ := a.Claimants(name)
		cb, err := b.Claimants(name)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(ca, cb) {
			t.Errorf("Claimants(%q) differ: %v vs %v", name, ca, cb)
		}
	}

	for _, key := range parityKeys(a, pa) {
		ca, _ := a.CitedBy(key)
		cb, err := b.CitedBy(key)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(ca, cb) {
			t.Errorf("CitedBy(%q) differ: %v vs %v", key, ca, cb)
		}
	}

	la, _ := a.AvgLength()
	lb, err := b.AvgLength()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(la-lb) > 1e-9 {
		t.Errorf("AvgLength differs: %v vs %v", la, lb)
	}
}

// synthParityKB writes a corpus with the features a disagreement would hide in:
// aliases, tags, links, citations, an archived page, a source page and an
// accented word.
func synthParityKB(t *testing.T, dir string, n int) *kb.KB {
	t.Helper()
	writeFile(t, dir, "stemma.toml", "title = \"Parity\"\n")
	writeFile(t, dir, "bibliography.bib", "@article{k1,}\n@article{k2,}\n@article{k3,}\n")
	writeFile(t, dir, "pages/index.md", pageFile("Index", "type: index", "start at [[Hub]] and [[Page 0]]\n"))
	writeFile(t, dir, "pages/hub.md", pageFile("Hub",
		"type: concept\ntags: [Machine-Learning]\naliases: [Start, Home]",
		"the hub cites [@k1] and discusses retrieval and graphs\n"))
	writeFile(t, dir, "pages/a-source.md", pageFile("A Source", "type: source\nkey: k9", "generated text\n"))

	for i := 0; i < n; i++ {
		typ := "concept"
		if i%2 == 1 {
			typ = "note"
		}
		extra := fmt.Sprintf("type: %s\ntags: [topic-%d]", typ, i%3)
		if i == 0 {
			extra += "\nstatus: archived"
		}
		body := fmt.Sprintf(
			"page %d mentions retrieval and index and café; it links to [[Hub]] and [[Page %d]] and cites [@k%d].\n",
			i, (i+1)%n, i%3+1)
		writeFile(t, dir, fmt.Sprintf("pages/page-%d.md", i),
			pageFile(fmt.Sprintf("Page %d", i), extra, body))
	}

	k, err := kb.Load(dir)
	if err != nil {
		t.Fatalf("load parity KB: %v", err)
	}
	return k
}

// paritySearches derives a battery of queries from the corpus, so the queries
// cover what the corpus actually holds rather than what a fixture assumed.
func paritySearches(pages []PageMeta) []SearchRequest {
	reqs := []SearchRequest{
		{Query: ""},
		{Query: "", Limit: 2},
		{Query: "the"},
		{Query: "retrieval"},
		{Query: "cafe"},
		{Query: "zzzznomatch"},
	}
	for _, m := range pages {
		if m.Type != "" {
			reqs = append(reqs, SearchRequest{Query: "", Type: m.Type})
			break
		}
	}
	for _, m := range pages {
		if dir := path.Dir(m.Path); dir != "pages" {
			reqs = append(reqs, SearchRequest{Query: "", Dir: dir})
			break
		}
	}
	for _, m := range pages {
		if len(m.Tags) > 0 {
			reqs = append(reqs, SearchRequest{Tags: []string{m.Tags[0]}})
			break
		}
	}
	for _, m := range pages {
		if words := Tokenize(m.Title); len(words) > 0 {
			reqs = append(reqs, SearchRequest{Query: words[0], Limit: 3})
			break
		}
	}
	return reqs
}

func parityNames(k *kb.KB) []string {
	out := append([]string{}, k.Graph.Names()...)
	return append(out, "no-such-name")
}

func parityKeys(src Source, pages []PageMeta) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range pages {
		cs, err := src.Citations(m.Path)
		if err != nil {
			continue
		}
		for _, c := range cs {
			if !seen[c.Key] {
				seen[c.Key] = true
				out = append(out, c.Key)
			}
		}
	}
	return out
}

// exampleWiki copies the project's example wiki into a temporary directory, so
// the index is built somewhere writable and the repository is left alone. The
// generated .stemma/ directory is skipped: it is a cache, not corpus.
func exampleWiki(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "wiki")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("the example wiki is not present: %v", err)
	}
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == ".stemma" || strings.HasPrefix(rel, ".stemma"+string(filepath.Separator)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatalf("copy the example wiki: %v", err)
	}
	return dst
}

func docOf(t *testing.T, src Source, path string) Doc {
	t.Helper()
	doc, err := src.Doc(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func linksOf(t *testing.T, src Source, path string) []kb.Link {
	t.Helper()
	links, err := src.Links(path)
	if err != nil {
		t.Fatal(err)
	}
	return links
}

func backlinksOf(t *testing.T, src Source, path string) []string {
	t.Helper()
	back, err := src.Backlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return back
}

func citesOf(t *testing.T, src Source, path string) []CitationRef {
	t.Helper()
	cites, err := src.Citations(path)
	if err != nil {
		t.Fatal(err)
	}
	return cites
}

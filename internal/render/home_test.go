package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// houseKB builds a KB in memory for the landing page. It has the two things the
// page can only get from a bibliography — dated sources, so the newest-sources
// block has an order to get wrong, and a source that can be left uncited, so the
// attention block has something to report when it should. entry is whether the KB
// has an entry document, because that is the whole question the page has to
// answer either way.
func houseKB(t *testing.T, entry, citeAll bool) *kb.KB {
	t.Helper()
	g := kb.NewGraph()
	add := func(path, raw string) {
		p, err := kb.ParsePage([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		g.Add(path, p)
	}
	if entry {
		add("pages/index.md", "---\ntitle: Start Here\ntype: index\n---\nRead [[Alpha]] first, then [[Beta]].\n")
	}
	add("pages/alpha.md", "---\ntitle: Alpha\ntype: concept\ntags: [site]\n---\nAlpha rests on [@old2020] and [@new2024].\n")
	beta := "---\ntitle: Beta\ntype: note\ntags: [site]\n---\nBeta rests on [@new2024].\n"
	if citeAll {
		beta = "---\ntitle: Beta\ntype: note\ntags: [site]\n---\nBeta rests on [@new2024] and [@uncited2019].\n"
	}
	add("pages/beta.md", beta)

	bib, err := kb.ParseBibliography("bibliography.bib", []byte(`@article{new2024,
  title = {A Newer Work},
  year = {2024},
  stemma-retrieved = {2024-05-01},
}
@article{old2020,
  title = {An Older Work},
  year = {2020},
  stemma-retrieved = {2020-01-02},
}
@article{uncited2019,
  title = {Never Cited},
  year = {2019},
  stemma-retrieved = {2019-03-03},
}
`))
	if err != nil {
		t.Fatal(err)
	}
	m := kb.DefaultManifest("House")
	m.Description = "A KB for the landing-page tests."
	return &kb.KB{
		Graph:        g,
		Bibliography: bib,
		Vocabulary:   kb.DefaultVocabulary(),
		Manifest:     m,
	}
}

func home(t *testing.T, k *kb.KB) string {
	t.Helper()
	r, err := New(k)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Home()
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

// The page says what the KB holds, opens with the entry document, and fills each
// block from the KB rather than from anything an author had to write.
func TestHomeShowsWhatTheKBHolds(t *testing.T) {
	got := home(t, houseKB(t, true, false))

	for _, want := range []string{
		"<h1>Start Here</h1>",
		"A KB for the landing-page tests.",
		"3 pages",
		"3 sources",
		"1 tag",
		`<a href="all.html">3 pages</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the landing page is missing %q", want)
		}
	}

	// The entry document is the opening: its prose is on the page, and its links
	// have been resolved like any other page's.
	opening := between(t, got, `<article class="opening">`, `</article>`)
	if !strings.Contains(opening, "Read ") || !strings.Contains(opening, `href="alpha.html"`) {
		t.Errorf("the opening is not the entry document's body:\n%s", opening)
	}

	// Browse is counted, busiest first.
	browse := between(t, got, "<h2>Browse</h2>", "</section>")
	for _, want := range []string{"By type", "By tag", `href="types.html"`, `href="tags.html"`} {
		if !strings.Contains(browse, want) {
			t.Errorf("the browse block is missing %q:\n%s", want, browse)
		}
	}

	// Newest sources are the dated ones, newest first, and only the cited ones —
	// a source nothing cites has no page to link to and is reported as a thing to
	// fix instead.
	sources := between(t, got, "<h2>Newest sources</h2>", "</section>")
	newer, older := strings.Index(sources, "A Newer Work"), strings.Index(sources, "An Older Work")
	if newer < 0 || older < 0 || newer > older {
		t.Errorf("the sources are not newest first:\n%s", sources)
	}
	if strings.Contains(sources, "Never Cited") {
		t.Errorf("an uncited source is listed among the cited ones:\n%s", sources)
	}
	if !strings.Contains(sources, "cited by ") || !strings.Contains(sources, `href="beta.html"`) {
		t.Errorf("a source does not name the pages that cite it:\n%s", sources)
	}

	// The one thing to fix: a source nothing cites.
	attention := between(t, got, `<section class="home-block attention">`, "</section>")
	if !strings.Contains(attention, "sources nothing cites") {
		t.Errorf("the attention block does not report the uncited source:\n%s", attention)
	}
}

// The entry document is optional, which is what makes deleting it a supported
// thing to do: the page is generated either way, and without one it simply has no
// opening.
func TestHomeWithoutAnEntryDocument(t *testing.T) {
	got := home(t, houseKB(t, false, false))

	if strings.Contains(got, `class="opening"`) {
		t.Error("a KB with no entry document still has an opening")
	}
	if !strings.Contains(got, "<h1>House</h1>") {
		t.Error("the heading is not the KB's own title when there is no entry document")
	}
	for _, want := range []string{"Browse", "Newest sources", "sources nothing cites"} {
		if !strings.Contains(got, want) {
			t.Errorf("the generated page is missing %q without an entry document", want)
		}
	}
}

// A clean KB has nothing to attend to, so the block that says so is absent rather
// than empty. This is what makes it safe on a published page, and it is the state
// the example wiki is kept in.
func TestHomeSaysNothingWhenThereIsNothingToFix(t *testing.T) {
	got := home(t, houseKB(t, true, true))
	if strings.Contains(got, "Needs attention") {
		t.Errorf("a KB with nothing to fix still shows the block:\n%s", got)
	}
	if !strings.Contains(got, "Newest sources") {
		t.Error("the other blocks are gone too")
	}
}

// The entry document is not written as a page of its own: it is the home page.
// Two documents claiming one URL would be an error the writer raises, and the
// absence of that error is not enough — the home has to be the landing page.
func TestTheEntryDocumentIsTheHomePage(t *testing.T) {
	r, err := New(houseKB(t, true, false))
	if err != nil {
		t.Fatal(err)
	}
	docs, err := r.Documents()
	if err != nil {
		t.Fatal(err)
	}
	homes := 0
	for _, d := range docs {
		if d.URL != "index.html" {
			continue
		}
		homes++
		if !strings.Contains(string(d.Body), `class="counts"`) {
			t.Error("index.html is not the landing page")
		}
		if !strings.Contains(string(d.Body), `class="opening"`) {
			t.Error("index.html does not carry the entry document's opening")
		}
	}
	if homes != 1 {
		t.Errorf("the site has %d documents at index.html, want 1", homes)
	}
}

func TestHomeGolden(t *testing.T) {
	got := home(t, houseKB(t, true, false))

	golden := filepath.Join("testdata", "home.golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s", golden)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading the golden file (run `go test ./internal/render -update` to write it): %v", err)
	}
	if got != string(want) {
		t.Errorf("the landing page changed.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// between is the text between two markers, for asserting on one block.
func between(t *testing.T, s, from, to string) string {
	t.Helper()
	i := strings.Index(s, from)
	if i < 0 {
		t.Fatalf("%q is not in the page", from)
	}
	rest := s[i+len(from):]
	j := strings.Index(rest, to)
	if j < 0 {
		return rest
	}
	return rest[:j]
}

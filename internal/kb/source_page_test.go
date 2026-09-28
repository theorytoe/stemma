package kb

import (
	"strings"
	"testing"
)

const sourceBib = `@article{bush1945,
  author = {Bush, Vannevar},
  title = {As We May Think},
  journal = {The Atlantic Monthly},
  year = {1945},
  stemma-retrieved = {2026-09-28},
  stemma-content-hash = {sha256:ab12},
}
`

func TestSourcePage(t *testing.T) {
	e := parseOne(t, sourceBib)
	p, err := SourcePage(e, []string{"A", "B"})
	if err != nil {
		t.Fatal(err)
	}

	if got := p.Type(); got != TypeSource {
		t.Errorf("type = %q, want %q", got, TypeSource)
	}
	if got := p.Key(); got != "bush1945" {
		t.Errorf("key = %q", got)
	}
	if got := p.Title(); got != "As We May Think" {
		t.Errorf("title = %q", got)
	}

	body := string(p.Body())
	for _, want := range []string{
		"- **author**: Bush, Vannevar",
		"- **title**: As We May Think",
		"- **stemma-retrieved**: 2026-09-28",
		"- **stemma-content-hash**: sha256:ab12",
		"## Cited by",
		"[[A]]",
		"[[B]]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q:\n%s", want, body)
		}
	}
}

func TestSourcePageWithoutATitleUsesTheKey(t *testing.T) {
	e := parseOne(t, "@misc{onlyakey, year = {2020}}\n")
	p, err := SourcePage(e, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Title(); got != "onlyakey" {
		t.Errorf("title = %q, want the key", got)
	}
	if body := string(p.Body()); !strings.Contains(body, "Nothing in this knowledge base cites this source.") {
		t.Errorf("body = %q", body)
	}
}

func TestSourcePagesCoversOnlyCitedEntries(t *testing.T) {
	b, err := ParseBibliography("b.bib", []byte(
		"@article{cited, title = {Cited}}\n"+
			"@article{uncited, title = {Uncited}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	g := NewGraph()
	g.Add("pages/p.md", pageWithBody(t, "see [@cited]\n"))
	k := &KB{Graph: g, Bibliography: b}

	pages, err := k.SourcePages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	if got := pages[0].Key(); got != "cited" {
		t.Errorf("key = %q, want cited", got)
	}
}

func TestSourcePageIsExemptFromNamesAndOrphans(t *testing.T) {
	e := parseOne(t, sourceBib)
	src, err := SourcePage(e, nil)
	if err != nil {
		t.Fatal(err)
	}

	g := NewGraph()
	g.Add("sources/bush1945.md", src)
	// A real page whose title happens to equal the source page's title. If the
	// source page claimed a name, this would be a collision.
	g.Add("pages/as-we-may-think.md", page(t, "As We May Think", "type: concept", "an authored page\n"))
	// And a page that links the title, which must not become a backlink to the
	// source page.
	g.Add("pages/linker.md", page(t, "Linker", "type: concept", "see [[As We May Think]]\n"))

	claimants := g.Claimants("As We May Think")
	if len(claimants) != 1 || claimants[0] != "pages/as-we-may-think.md" {
		t.Errorf("claimants = %q, want only the authored page", claimants)
	}

	for _, f := range g.Findings(Lenient) {
		if f.Code == CodeNameCollision {
			t.Errorf("a source page caused a name collision: %+v", f)
		}
	}

	for _, path := range g.Orphans() {
		if path == "sources/bush1945.md" {
			t.Error("a source page was reported as an orphan")
		}
	}

	if got := g.Backlinks("sources/bush1945.md"); len(got) != 0 {
		t.Errorf("a source page has backlinks: %q", got)
	}
}

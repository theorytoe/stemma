package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// updateGolden rewrites the golden file from what the renderer currently does.
// It is how a deliberate change to the HTML is recorded, and the only way the
// file changes: a test that rewrote its own expectation would prove nothing.
var updateGolden = flag.Bool("update", false, "rewrite the golden files from the current behaviour")

// sampleBody exercises the cases the core has to get right: a link to a page by
// title, an alias, a link that resolves to nothing, citations present and
// missing, a multi-key group, code that must not be touched, and a link inside
// emphasis.
const sampleBody = "The method [[attention-is-all-you-need]] and [[Nowhere]] and the alias [[transformer paper]].\n" +
	"\n" +
	"See [@vaswani2017] and [@missing2020] and a group [@vaswani2017; @missing2020].\n" +
	"\n" +
	"Inline `[[attention]]` and `[@vaswani2017]` stay code.\n" +
	"\n" +
	"*[[attention-is-all-you-need]]* in emphasis.\n" +
	"\n" +
	"```\n" +
	"[[attention]]\n" +
	"[@vaswani2017]\n" +
	"```\n"

// testKB builds a small KB in memory so the tests do not depend on a fixture on
// disk: a home page, a page that links are named after, a page an alias points
// at, and one cited entry.
func testKB(t *testing.T) *kb.KB {
	t.Helper()
	g := kb.NewGraph()
	add := func(path, raw string) {
		p, err := kb.ParsePage([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		g.Add(path, p)
	}
	add("pages/index.md", "---\ntitle: Home\ntype: index\n---\nStart here.\n")
	add("pages/attention.md", "---\ntitle: Attention Is All You Need\ntype: concept\naliases:\n  - transformer paper\n---\nThe paper.\n")
	add("pages/transformer.md", "---\ntitle: Transformer\ntype: concept\ntags: [architecture, attention]\n---\nThe architecture.\n")
	add("pages/odd name.md", "---\ntitle: Odd Name\ntype: note\n---\nA file name with a space.\n")
	add("pages/sample.md", "---\ntitle: Sample\ntype: note\n---\n"+sampleBody)

	bib, err := kb.ParseBibliography("bibliography.bib", []byte(`@article{vaswani2017,
  title = {Attention Is All You Need},
  author = {Vaswani, Ashish and Shazeer, Noam},
  year = {2017},
  journal = {NeurIPS},
}
`))
	if err != nil {
		t.Fatal(err)
	}
	// The renderer is pinned to one style on purpose. Which style is the default
	// is a manifest question, and letting it decide what the renderer's golden
	// holds would make a change to the default silently rewrite this test. The
	// example wiki exercises the default itself.
	manifest := kb.DefaultManifest("test")
	manifest.CitationStyle = "author-date"
	return &kb.KB{
		Graph:        g,
		Bibliography: bib,
		Vocabulary:   kb.DefaultVocabulary(),
		Manifest:     manifest,
	}
}

func renderSample(t *testing.T, mode kb.Mode) (string, []kb.Finding) {
	t.Helper()
	r, err := New(testKB(t))
	if err != nil {
		t.Fatal(err)
	}
	out, findings, err := r.Body(mode, "pages/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(out), findings
}

func TestBodyGolden(t *testing.T) {
	got, findings := renderSample(t, kb.Lenient)
	if len(findings) == 0 {
		t.Fatal("the sample page should raise findings for its broken link and missing key")
	}

	golden := filepath.Join("testdata", "body.golden")
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
	if !bytes.Equal([]byte(got), want) {
		t.Errorf("output changed.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestResolvedLinkShowsTheTitle(t *testing.T) {
	got, _ := renderSample(t, kb.Lenient)
	want := `<a href="attention.html">Attention Is All You Need</a>`
	// The slug spelling [[attention-is-all-you-need]], the alias
	// [[transformer paper]], and the link inside emphasis all resolve to the
	// same page and read alike: the title, not the spelling that was typed.
	if n := strings.Count(got, want); n != 3 {
		t.Errorf("the resolved link appears %d times, want 3:\n%s", n, got)
	}
}

func TestBrokenLinkKeepsItsText(t *testing.T) {
	got, _ := renderSample(t, kb.Lenient)
	if !strings.Contains(got, `<span class="stemma-unresolved">Nowhere</span>`) {
		t.Errorf("the unresolved link is not marked:\n%s", got)
	}
}

func TestCitationsLinkEachKey(t *testing.T) {
	got, _ := renderSample(t, kb.Lenient)
	if !strings.Contains(got, `<a href="sources/vaswani2017.html">Vaswani and Shazeer 2017</a>`) {
		t.Errorf("a defined key is not linked to its source page:\n%s", got)
	}
	if strings.Contains(got, `href="sources/missing2020.html"`) {
		t.Errorf("a key the bibliography does not define was linked anyway:\n%s", got)
	}
	if !strings.Contains(got, `; missing2020)`) {
		t.Errorf("a missing key in a group should stay text:\n%s", got)
	}
}

func TestCodeIsNotRendered(t *testing.T) {
	got, _ := renderSample(t, kb.Lenient)
	for _, want := range []string{
		"<code>[[attention]]</code>",
		"<code>[@vaswani2017]</code>",
		"<pre><code>[[attention]]\n[@vaswani2017]\n</code></pre>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("code was altered; want %q in:\n%s", want, got)
		}
	}
}

// TestFindingsMatchLint is the agreement the delegate asks for: what rendering
// reports about a page is what lint reports about it, text and severity alike.
func TestFindingsMatchLint(t *testing.T) {
	k := testKB(t)
	r, err := New(k)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []kb.Mode{kb.Lenient, kb.Strict} {
		_, got, err := r.Body(mode, "pages/sample.md")
		if err != nil {
			t.Fatal(err)
		}
		var want []kb.Finding
		for _, f := range k.Lint(mode) {
			if f.Path != "pages/sample.md" {
				continue
			}
			switch f.Code {
			case kb.CodeUnresolvedLink, kb.CodeAmbiguousLink, kb.CodeCitationMissing:
				want = append(want, f)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("mode %v: render findings =\n%+v\nlint findings =\n%+v", mode, got, want)
		}
	}
}

func TestNewRejectsAnUnknownStyle(t *testing.T) {
	k := testKB(t)
	k.Manifest.CitationStyle = "apa"
	if _, err := New(k); err == nil {
		t.Error("an unknown citation style was accepted")
	}
}

func TestPageURL(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"pages/index.md", "index.html"},
		{"pages/attention.md", "attention.html"},
		{"pages/notes/one.md", "notes/one.html"},
		{"pages/a b.md", "a%20b.html"},
	} {
		if got := PageURL(tc.in); got != tc.want {
			t.Errorf("PageURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSourceURL(t *testing.T) {
	if got, want := SourceURL("vaswani2017"), "sources/vaswani2017.html"; got != want {
		t.Errorf("SourceURL = %q, want %q", got, want)
	}
	if got, want := SourceURL("a/b:c"), "sources/a%2Fb:c.html"; got != want {
		t.Errorf("SourceURL = %q, want %q", got, want)
	}
}

// TestBodyDoesNotFractionizeIdentifiers pins the SmartypantsFractions fix. A
// DOI is a run of digits around a slash, which Smartypants otherwise renders as
// a superscript numerator and a subscript denominator.
func TestBodyDoesNotFractionizeIdentifiers(t *testing.T) {
	g := kb.NewGraph()
	p, err := kb.ParsePage([]byte("---\ntitle: Identifiers\ntype: note\n---\n" +
		"The DOI is 10.1145/2568225.2568322 and the date is 2024/09/30.\n"))
	if err != nil {
		t.Fatal(err)
	}
	g.Add("pages/ids.md", p)
	k := &kb.KB{
		Graph:        g,
		Bibliography: kb.NewBibliography(),
		Vocabulary:   kb.DefaultVocabulary(),
		Manifest:     kb.DefaultManifest("test"),
	}
	r, err := New(k)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := r.Body(kb.Lenient, "pages/ids.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"<sup>", "<sub>", "&frasl;"} {
		if strings.Contains(string(got), bad) {
			t.Errorf("an identifier was rendered as a fraction (%s):\n%s", bad, got)
		}
	}
	for _, want := range []string{"10.1145/2568225.2568322", "2024/09/30"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

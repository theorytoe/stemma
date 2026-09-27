package kb

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func page(t *testing.T, title string, extra string, body string) *Page {
	t.Helper()
	var b strings.Builder
	b.WriteString("---\ntitle: " + title + "\n")
	if extra != "" {
		b.WriteString(extra)
		b.WriteString("\n")
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return mustParse(t, b.String())
}

// kb is a small graph helper: add pages by path, titled after the path.
func kb(t *testing.T, pages map[string]string) *Graph {
	t.Helper()
	g := NewGraph()
	for path, title := range pages {
		g.Add(path, page(t, title, "type: concept", ""))
	}
	return g
}

// withCode keeps only the findings of one kind, so that a test about links does
// not have to restate the orphan findings every graph raises.
func withCode(fs []Finding, code string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

func TestResolveByName(t *testing.T) {
	g := kb(t, map[string]string{
		"pages/a.md": "Attention Is All You Need",
		"pages/b.md": "Transformer",
	})

	for _, name := range []string{
		"Attention Is All You Need",
		"attention is all you need",
		"attention-is-all-you-need",
		"ATTENTION IS ALL YOU NEED",
	} {
		got := g.Resolve(name)
		if got.Kind != Resolved || got.Path != "pages/a.md" {
			t.Errorf("Resolve(%q) = %+v, want pages/a.md", name, got)
		}
	}

	if got := g.Resolve("Nobody"); got.Kind != Unresolved {
		t.Errorf("Resolve(Nobody) = %+v, want unresolved", got)
	}
}

func TestResolveByAlias(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "Attention Is All You Need", "aliases:\n  - transformer paper", ""))

	if got := g.Resolve("Transformer Paper"); got.Kind != Resolved || got.Path != "pages/a.md" {
		t.Errorf("Resolve by alias = %+v", got)
	}
}

// An alias that normalises to the page's own title is the same name, not a page
// colliding with itself.
func TestAliasMayRestateTheTitle(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "Go", "aliases:\n  - go", ""))

	if got := g.Resolve("Go"); got.Kind != Resolved {
		t.Errorf("Resolve(Go) = %+v, want resolved", got)
	}
	if got := g.Findings(Lenient); len(withCode(got, CodeNameCollision)) != 0 {
		t.Errorf("name collision = %+v, want none", got)
	}
}

// Two pages claiming one name is what the format refuses to guess about.
func TestAmbiguityIsAHardErrorInBothModes(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "Go", "type: concept", ""))
	g.Add("pages/b.md", page(t, "Golang", "type: concept\naliases:\n  - go", "Link: [[Go]].\n"))

	if got := g.Resolve("go"); got.Kind != Ambiguous {
		t.Fatalf("Resolve(go) = %+v, want ambiguous", got)
	} else if !reflect.DeepEqual(got.Matches, []string{"pages/a.md", "pages/b.md"}) {
		t.Errorf("matches = %q", got.Matches)
	}

	for _, mode := range []Mode{Lenient, Strict} {
		found := false
		for _, f := range g.Findings(mode) {
			if f.Code == CodeAmbiguousLink {
				found = true
				if f.Severity != Error {
					t.Errorf("mode %v: ambiguous link severity = %q, want %q", mode, f.Severity, Error)
				}
			}
		}
		if !found {
			t.Errorf("mode %v: no ambiguous-link finding", mode)
		}
	}
}

// A collision nobody has linked to yet is a warning, and an error only under
// strict. It has to be fixable before it breaks a link.
func TestLatentCollisionIsSoft(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "Go", "type: concept", ""))
	g.Add("pages/b.md", page(t, "Golang", "type: concept\naliases:\n  - go", ""))

	lenient := withCode(g.Findings(Lenient), CodeNameCollision)
	if len(lenient) != 1 {
		t.Fatalf("lenient findings = %+v", g.Findings(Lenient))
	}
	if lenient[0].Severity != Warning {
		t.Errorf("lenient severity = %q, want %q", lenient[0].Severity, Warning)
	}
	if got := withCode(g.Findings(Strict), CodeNameCollision); got[0].Severity != Error {
		t.Errorf("strict severity = %q, want %q", got[0].Severity, Error)
	}
}

func TestUnresolvedLinkPointsAtItsLine(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", "\nsee [[Missing]].\n"))

	fs := withCode(g.Findings(Lenient), CodeUnresolvedLink)
	if len(fs) != 1 {
		t.Fatalf("findings = %+v", fs)
	}
	if fs[0].Path != "pages/a.md" || fs[0].Line != 6 {
		t.Errorf("path = %q, line = %d", fs[0].Path, fs[0].Line)
	}
}

func TestEmptyLinkIsReportedClearly(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", "[[]]\n"))

	fs := withCode(g.Findings(Lenient), CodeUnresolvedLink)
	if len(fs) != 1 {
		t.Fatalf("findings = %+v", fs)
	}
	if !strings.Contains(fs[0].Message, "empty") {
		t.Errorf("message = %q", fs[0].Message)
	}
}

func TestBacklinks(t *testing.T) {
	g := NewGraph()
	g.Add("pages/b.md", page(t, "B", "type: concept", ""))
	g.Add("pages/a.md", page(t, "A", "type: concept", "links to [[B]]\n"))
	g.Add("pages/c.md", page(t, "C", "type: concept", "also [[b]]\n"))

	got := g.Backlinks("pages/b.md")
	want := []string{"pages/a.md", "pages/c.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("backlinks = %q, want %q", got, want)
	}
}

// A page must not be its own backlink, or nothing would ever be an orphan.
func TestSelfLinkIsNotABacklink(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", "see [[A]]\n"))

	if got := g.Backlinks("pages/a.md"); len(got) != 0 {
		t.Errorf("backlinks = %q, want none", got)
	}
	if got := g.Orphans(); !reflect.DeepEqual(got, []string{"pages/a.md"}) {
		t.Errorf("orphans = %q", got)
	}
}

// A page linked by an alias is still linked, so it is not an orphan.
func TestAliasLinkCountsAsABacklink(t *testing.T) {
	g := NewGraph()
	g.Add("pages/b.md", page(t, "B", "type: concept\naliases:\n  - bee", ""))
	g.Add("pages/a.md", page(t, "A", "type: concept", "see [[bee]]\n"))

	if got := g.Orphans(); len(got) != 1 || got[0] != "pages/a.md" {
		t.Errorf("orphans = %q, want only pages/a.md", got)
	}
}

// A page added before the page it links to still yields the backlink, because
// the map is keyed by name rather than by the path that was known at the time.
func TestBacklinksDoNotDependOnAddOrder(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", "links to [[B]]\n"))
	if got := g.Backlinks("pages/b.md"); len(got) != 0 {
		t.Errorf("backlinks before B exists = %q", got)
	}
	g.Add("pages/b.md", page(t, "B", "type: concept", ""))

	if got := g.Backlinks("pages/b.md"); !reflect.DeepEqual(got, []string{"pages/a.md"}) {
		t.Errorf("backlinks = %q, want pages/a.md", got)
	}
}

func TestIndexPagesAreNotOrphans(t *testing.T) {
	g := NewGraph()
	g.Add("pages/index.md", page(t, "Index", "type: index", ""))
	g.Add("pages/a.md", page(t, "A", "type: concept", ""))

	if got := g.Orphans(); !reflect.DeepEqual(got, []string{"pages/a.md"}) {
		t.Errorf("orphans = %q, want only pages/a.md", got)
	}
}

// The graph is meant to be kept current one page at a time, so replacing a page
// must leave nothing of the old one behind.
func TestReplacingAPageUpdatesEverything(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept\naliases:\n  - old-name", "[[B]]\n"))
	g.Add("pages/b.md", page(t, "B", "type: concept", ""))

	if got := g.Resolve("old-name"); got.Kind != Resolved {
		t.Errorf("the alias is gone: %+v", got)
	}
	if got := g.Backlinks("pages/b.md"); len(got) != 1 {
		t.Errorf("backlinks = %q", got)
	}

	g.Add("pages/a.md", page(t, "A", "type: concept", "no links now\n"))

	if got := g.Resolve("old-name"); got.Kind != Unresolved {
		t.Errorf("the old alias survived: %+v", got)
	}
	if got := g.Backlinks("pages/b.md"); len(got) != 0 {
		t.Errorf("the old link survived: %q", got)
	}
	if got := g.Len(); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}

func TestRemoveTakesEverythingWithIt(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept\naliases:\n  - aye", "[[B]]\n"))
	g.Add("pages/b.md", page(t, "B", "type: concept", ""))

	g.Remove("pages/a.md")
	g.Remove("pages/a.md") // twice is not an error

	if got := g.Len(); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}
	if got := g.Resolve("A"); got.Kind != Unresolved {
		t.Errorf("the title survived: %+v", got)
	}
	if got := g.Resolve("aye"); got.Kind != Unresolved {
		t.Errorf("the alias survived: %+v", got)
	}
	if got := g.Backlinks("pages/b.md"); len(got) != 0 {
		t.Errorf("the link survived: %q", got)
	}
}

func TestResolvingAnUnknownPathIsEmpty(t *testing.T) {
	g := NewGraph()
	if got := g.Backlinks("pages/nope.md"); got != nil {
		t.Errorf("backlinks = %q", got)
	}
	if _, ok := g.Page("pages/nope.md"); ok {
		t.Error("Page found a page that is not there")
	}
	if got := g.Links("pages/nope.md"); got != nil {
		t.Errorf("links = %v", got)
	}
}

func TestFindingsAreSorted(t *testing.T) {
	g := NewGraph()
	g.Add("pages/b.md", page(t, "B", "type: concept", "[[Missing]]\n"))
	g.Add("pages/a.md", page(t, "A", "type: concept", "[[AlsoMissing]]\n"))

	fs := g.Findings(Lenient)
	if len(fs) != 4 {
		t.Fatalf("findings = %d, want 4", len(fs))
	}
	for i := 1; i < len(fs); i++ {
		if fs[i-1].Path > fs[i].Path {
			t.Fatalf("findings are not sorted by path: %+v", fs)
		}
	}
}

// Tier 0 has to hold up on a KB far larger than the hundreds-of-pages case the
// format is aimed at, without an index and without a database.
func TestResolvesAtScale(t *testing.T) {
	const n = 5000
	g := NewGraph()
	for i := 0; i < n; i++ {
		body := ""
		if i > 0 {
			body = fmt.Sprintf("see [[Page %d]]\n", i-1)
		}
		g.Add(fmt.Sprintf("pages/p%04d.md", i), page(t, fmt.Sprintf("Page %d", i), "type: concept", body))
	}

	if got := g.Len(); got != n {
		t.Fatalf("Len = %d, want %d", got, n)
	}
	if got := g.Resolve("Page 2500"); got.Kind != Resolved || got.Path != "pages/p2500.md" {
		t.Errorf("Resolve = %+v", got)
	}
	// Every page but the first is linked to by the one after it.
	if got := len(g.Orphans()); got != 1 {
		t.Errorf("orphans = %d, want 1", got)
	}
}

func BenchmarkGraphBuild(b *testing.B) {
	raw := make([]string, 1000)
	for i := range raw {
		raw[i] = fmt.Sprintf("---\ntitle: Page %d\ntype: concept\n---\nsee [[Page %d]]\n", i, (i+1)%1000)
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		g := NewGraph()
		for i, s := range raw {
			p, err := ParsePage([]byte(s))
			if err != nil {
				b.Fatal(err)
			}
			g.Add(fmt.Sprintf("pages/p%04d.md", i), p)
		}
	}
}

// Retiring a page is exactly when the links to it are removed, so an archived
// page with no inbound links is the expected end state and not a problem.
// Without this, archiving a page and unlinking it would leave a KB that could
// never lint clean again.
func TestArchivedPagesAreNotOrphans(t *testing.T) {
	g := NewGraph()
	g.Add("pages/index.md", page(t, "Index", "type: index", ""))
	g.Add("pages/retired.md", page(t, "Retired", "type: note\nstatus: archived", ""))
	g.Add("pages/live.md", page(t, "Live", "type: concept", ""))

	got := g.Orphans()
	if len(got) != 1 || got[0] != "pages/live.md" {
		t.Errorf("orphans = %q, want only pages/live.md", got)
	}
}

// An archived page is still a page: it just is not complained about for being
// unlinked.
func TestArchivedPagesAreStillInTheGraph(t *testing.T) {
	g := NewGraph()
	g.Add("pages/retired.md", page(t, "Retired", "type: note\nstatus: archived", ""))
	if got := g.Len(); got != 1 {
		t.Errorf("Len = %d", got)
	}
	if r := g.Resolve("Retired"); r.Kind != Resolved {
		t.Errorf("an archived page does not resolve: %+v", r)
	}
}

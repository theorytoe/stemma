package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// scopedKB is a KB built to give an extract every case it has to decide on:
// a link that stays, a link out of the slice, a link that names nothing, a link
// two pages answer to, a citation the bibliography defines, a citation it does
// not, a custom type, and the source's own entry document inside the slice.
func scopedKB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeKBFile(t, dir, "stemma.toml", "title = \"Scoped Test\"\ntypes = [\"principle\"]\n")
	writeKBFile(t, dir, "pages/index.md", "---\ntitle: Home\ntype: index\n---\nHome links [[Root]].\n")
	writeKBFile(t, dir, "pages/root.md", "---\ntitle: Root\ntype: principle\n---\nRoot links [[Inside]], [[Home]], [[Nowhere]] and [[Beta]], and cites [@bush1945] and [@missing2020].\n")
	writeKBFile(t, dir, "pages/inside.md", "---\ntitle: Inside\ntype: concept\n---\nInside links [[Deeper]].\n")
	writeKBFile(t, dir, "pages/deeper.md", "---\ntitle: Deeper\ntype: concept\n---\nDeeper is one hop too far for a depth-one extract.\n")
	writeKBFile(t, dir, "pages/alpha.md", "---\ntitle: Alpha\ntype: concept\naliases: [Beta]\n---\nAlpha answers to Beta as well.\n")
	writeKBFile(t, dir, "pages/beta.md", "---\ntitle: Beta\ntype: concept\n---\nBeta is a leaf.\n")
	writeKBFile(t, dir, "bibliography.bib", "@article{bush1945,\n  author = {Bush, Vannevar},\n  title = {As We May Think},\n  year = {1945},\n}\n")
	return dir
}

func scopedKBLoaded(t *testing.T) *kb.KB {
	t.Helper()
	return load(t, scopedKB(t))
}

// The slice is the root and what it links to, and nothing else.
func TestScopedTakesTheRootAndWhatItLinksTo(t *testing.T) {
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"pages/root.md", "pages/inside.md", "pages/index.md"}
	if strings.Join(x.Pages, ",") != strings.Join(want, ",") {
		t.Errorf("pages = %v, want %v", x.Pages, want)
	}
	if !wrote(x, "pages/index.md") {
		t.Error("the extract has no entry document")
	}
	if wrote(x, "pages/deeper.md") {
		t.Error("the extract holds a page more than one hop away")
	}
}

// A vendored capture is part of the source it belongs to, so an extract carries
// it: the extract's site then offers the same full text the source KB does.
func TestScopedCarriesVendoredCaptures(t *testing.T) {
	dir := scopedKB(t)
	text := "the captured text\n"
	full := kb.VendoredPath(dir, "bush1945")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	x, err := Scoped(load(t, dir), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	name := kb.VendoredName("bush1945")
	if !wrote(x, name) {
		t.Fatalf("the extract does not carry %s; it has %v", name, filePaths(x))
	}
	if got := string(fileOf(t, x, name)); got != text {
		t.Errorf("capture = %q, want %q", got, text)
	}
}

// Depth 0 is the whole reachable set, so the page that was out of reach comes
// along and nothing is pruned for being outside.
func TestScopedDepthZeroReachesEverything(t *testing.T) {
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"pages/root.md", "pages/inside.md", "pages/deeper.md"} {
		if !contains(x.Pages, p) {
			t.Errorf("depth 0 does not reach %s: %v", p, x.Pages)
		}
	}
	for _, p := range x.Pruned {
		if p.Reason == ReasonOutside {
			t.Errorf("%s was pruned as outside at depth 0", p.Target)
		}
	}
}

// The root takes the entry document's place, so the page the extract is of is
// the page a reader arrives at — and the source's entry document, which is in
// the slice, moves aside rather than being lost.
func TestScopedRootBecomesTheEntryDocument(t *testing.T) {
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	if x.Entry != "pages/index.md" {
		t.Errorf("entry = %q", x.Entry)
	}
	root := string(fileOf(t, x, "pages/index.md"))
	if !strings.Contains(root, "title: Root") {
		t.Errorf("the entry document is not the root page:\n%s", root)
	}
	elsewhere := string(fileOf(t, x, "pages/index-2.md"))
	if !strings.Contains(elsewhere, "title: Home") {
		t.Errorf("the source's entry document was not moved aside:\n%s", elsewhere)
	}
}

// A link that cannot come along becomes the anchor text a reader of the source
// would have seen, and a link that named nothing or named two pages becomes what
// the author wrote.
func TestScopedPrunesLinksThatCannotCome(t *testing.T) {
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	byTarget := map[string]Pruned{}
	for _, p := range x.Pruned {
		byTarget[p.Target] = p
	}

	out := byTarget["Inside"] // not pruned: Inside is in the slice
	if out.Kind != "" {
		t.Errorf("a link inside the slice was pruned: %+v", out)
	}
	// Inside's link to Deeper is one hop too far.
	if got := byTarget["Deeper"]; got.Reason != ReasonOutside || got.Became != "Deeper" {
		t.Errorf("a link out of the slice is %+v", got)
	}
	if got := byTarget["Nowhere"]; got.Reason != ReasonUnresolved || got.Became != "Nowhere" {
		t.Errorf("a link that names nothing is %+v", got)
	}
	if got := byTarget["Beta"]; got.Reason != ReasonAmbiguous || got.Became != "Beta" {
		t.Errorf("an ambiguous link is %+v", got)
	}

	// And the prose reads: the pruned links are gone, the kept one is not.
	inside := string(fileOf(t, x, "pages/inside.md"))
	if !strings.Contains(inside, "Inside links Deeper.") {
		t.Errorf("a pruned link did not become its text:\n%s", inside)
	}
	root := string(fileOf(t, x, "pages/index.md"))
	if !strings.Contains(root, "[[Inside]]") || !strings.Contains(root, "Nowhere") || strings.Contains(root, "[[Nowhere]]") {
		t.Errorf("the root page did not keep one link and prune another:\n%s", root)
	}
}

// References are closed over: only the keys the extract still cites are
// materialised, and a citation nothing defines becomes prose rather than a
// reference pointing at nothing.
func TestScopedClosesCitations(t *testing.T) {
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Sources) != 1 || x.Sources[0] != "bush1945" {
		t.Errorf("sources = %v, want [bush1945]", x.Sources)
	}
	if !wrote(x, "bibliography.bib") {
		t.Fatal("no bibliography was materialised")
	}
	bib := string(fileOf(t, x, "bibliography.bib"))
	if !strings.Contains(bib, "@article{bush1945") {
		t.Errorf("the cited entry is missing:\n%s", bib)
	}
	var citation *Pruned
	for i, p := range x.Pruned {
		if p.Kind == "citation" {
			citation = &x.Pruned[i]
		}
	}
	if citation == nil {
		t.Fatalf("the missing citation was not reported: %+v", x.Pruned)
	}
	if citation.Reason != ReasonMissingKey || citation.Became != "[missing2020]" {
		t.Errorf("the pruned citation is %+v", *citation)
	}
	root := string(fileOf(t, x, "pages/index.md"))
	if strings.Contains(root, "@missing2020") {
		t.Errorf("a citation pointing at nothing survived:\n%s", root)
	}
}

// The source KB comes back out of an extraction unchanged. The extract rewrites
// the body, so it has to be rewriting a copy.
func TestScopedLeavesTheSourceAlone(t *testing.T) {
	k := scopedKBLoaded(t)
	page, _ := k.Graph.Page("pages/root.md")
	before := string(page.Bytes())
	links, cites := len(k.Graph.Links("pages/root.md")), len(k.Graph.Citations("pages/root.md"))

	if _, err := Scoped(k, "pages/root.md", 1); err != nil {
		t.Fatal(err)
	}
	if after := string(page.Bytes()); after != before {
		t.Errorf("the extraction rewrote the KB it read:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	if len(k.Graph.Links("pages/root.md")) != links || len(k.Graph.Citations("pages/root.md")) != cites {
		t.Error("the source graph changed")
	}
}

// The point of the extract is that it is a KB in its own right: it loads, and
// every link and citation in it resolves.
func TestScopedWritesAKBThatStandsAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "extract")
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := x.Write(dir); err != nil {
		t.Fatal(err)
	}
	if x.Out != dir {
		t.Errorf("Out = %q, want %q", x.Out, dir)
	}

	got := load(t, dir)
	if findings := got.Lint(kb.Strict); len(findings) != 0 {
		t.Errorf("the extract does not stand alone:\n%v", findings)
	}
}

// A second extraction into the same directory refreshes it: what it wrote
// before and not now is removed, and anything else is left alone.
func TestScopedRefreshesAnExtract(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "extract")
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := x.Write(dir); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(dir, "CNAME")
	if err := os.WriteFile(mine, []byte("example.org\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deep, err := Scoped(scopedKBLoaded(t), "pages/root.md", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := deep.Write(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mine); err != nil {
		t.Error("a refresh removed a file the extract did not write")
	}
	if !wrote(deep, "pages/deeper.md") {
		t.Fatal("the deeper extraction did not write the page it should")
	}
	for _, f := range deep.Files {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.Path))); err != nil {
			t.Errorf("%s is missing after the refresh", f.Path)
		}
	}
}

// A directory that is neither empty nor an extract is refused, rather than
// getting a KB root scattered through it.
func TestScopedRefusesADirectoryItDidNotMake(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "something.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	x, err := Scoped(scopedKBLoaded(t), "pages/root.md", 1)
	if err != nil {
		t.Fatal(err)
	}
	err = x.Write(dir)
	if err == nil {
		t.Fatal("an extract was written over a directory that was not empty")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "stemma.toml")); !os.IsNotExist(statErr) {
		t.Error("a refused write left something behind")
	}
}

func TestScopedUnknownPage(t *testing.T) {
	if _, err := Scoped(scopedKBLoaded(t), "pages/absent.md", 1); err == nil {
		t.Error("an extract of a page that is not there was accepted")
	}
	if _, err := Scoped(scopedKBLoaded(t), "pages/root.md", -1); err == nil {
		t.Error("a negative depth was accepted")
	}
}

// wrote reports whether the extract carries a file.
func wrote(x *Extract, path string) bool {
	for _, f := range x.Files {
		if f.Path == path {
			return true
		}
	}
	return false
}

// fileOf is one file of an extract.
func fileOf(t *testing.T, x *Extract, path string) []byte {
	t.Helper()
	for _, f := range x.Files {
		if f.Path == path {
			return f.Body
		}
	}
	t.Fatalf("the extract has no %s; it has %v", path, filePaths(x))
	return nil
}

func filePaths(x *Extract) []string {
	out := make([]string, 0, len(x.Files))
	for _, f := range x.Files {
		out = append(out, f.Path)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

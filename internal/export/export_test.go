package export

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files from the current behaviour")

// testKB writes a small KB that exercises every outcome the dump can report: a
// link that resolves, one that is ambiguous because two pages answer to the same
// name, one that resolves to nothing, a citation the bibliography defines, a
// citation it does not, and a page with only a body.
func testKB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeKBFile(t, dir, "stemma.toml", "title = \"Export Test\"\ndescription = \"A KB for the dump tests.\"\n")
	writeKBFile(t, dir, "pages/index.md", "---\ntitle: Home\ntype: index\ntags: [site]\n---\nHome links [[Alpha]], [[Beta]] and [[Nowhere]].\n")
	writeKBFile(t, dir, "pages/alpha.md", "---\ntitle: Alpha\ntype: concept\naliases: [First]\n---\nAlpha cites [@bush1945] and [@missing2020]. See [[Beta]].\n")
	writeKBFile(t, dir, "pages/beta.md", "---\ntitle: Beta\ntype: concept\naliases: [Alpha]\n---\nBeta also answers to [[First]].\n")
	writeKBFile(t, dir, "bibliography.bib", "@article{bush1945,\n  author = {Bush, Vannevar},\n  title = {As We May Think},\n  year = {1945},\n}\n")
	return dir
}

func writeKBFile(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func load(t *testing.T, root string) *kb.KB {
	t.Helper()
	k, err := kb.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// TestDumpGolden fixes the shape. The dump is documented in FORMAT.md as the
// contract other systems read, so a change to it is a change to a published
// interface and should have to be looked at rather than noticed later.
func TestDumpGolden(t *testing.T) {
	got, err := Assemble(load(t, testKB(t))).Bytes()
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "dump.golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s", golden)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading the golden file (run `go test ./internal/export -update` to write it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the dump shape changed.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// The three outcomes a link can have are all reported, and the page a link
// reached is named only when exactly one page answered.
func TestLinksCarryTheirResolution(t *testing.T) {
	byTarget := map[string]Link{}
	for _, l := range Assemble(load(t, testKB(t))).Links {
		byTarget[l.From+"->"+l.Target] = l
	}

	res := byTarget["pages/index.md->Beta"]
	if res.Resolution != Resolved || res.To != "pages/beta.md" {
		t.Errorf("a resolved link is %+v", res)
	}
	if len(res.Matches) != 0 {
		t.Errorf("a resolved link names matches: %v", res.Matches)
	}

	amb := byTarget["pages/index.md->Alpha"]
	if amb.Resolution != Ambiguous || amb.To != "" {
		t.Errorf("an ambiguous link is %+v", amb)
	}
	if len(amb.Matches) != 2 {
		t.Errorf("the ambiguous link lists %v, want both claimants", amb.Matches)
	}

	un := byTarget["pages/index.md->Nowhere"]
	if un.Resolution != Unresolved {
		t.Errorf("an unresolved link is %+v", un)
	}

	if res.Line == 0 || res.Line != un.Line {
		t.Errorf("links carry their line: %d and %d", res.Line, un.Line)
	}
}

// A citation says whether the bibliography defines its key, so a consumer can
// see that a reference list is short without also holding the bibliography.
func TestCitationsSayWhetherTheyResolve(t *testing.T) {
	got := map[string]bool{}
	for _, c := range Assemble(load(t, testKB(t))).Citations {
		got[c.Key] = c.Resolved
	}
	if !got["bush1945"] {
		t.Error("a key the bibliography defines is reported as unresolved")
	}
	if len(got) != 2 {
		t.Fatalf("citations = %v, want two", got)
	}
	if got["missing2020"] {
		t.Error("a key nothing defines is reported as resolved")
	}
}

// A consumer should never have to tell "none" from "not in this version", so an
// empty list is an empty list and not a missing key.
func TestEmptyListsArePresent(t *testing.T) {
	dir := t.TempDir()
	writeKBFile(t, dir, "stemma.toml", "title = \"Empty\"\n")
	got, err := Assemble(load(t, dir)).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"pages": []`, `"links": []`, `"citations": []`, `"bibliography": []`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("the dump does not carry %s:\n%s", want, got)
		}
	}
	if strings.Contains(string(got), "null") {
		t.Errorf("the dump carries a null where it should carry an empty list:\n%s", got)
	}
}

// Two runs over one KB produce the same bytes, which is what makes the dump
// diffable and the golden test meaningful.
func TestBytesAreDeterministic(t *testing.T) {
	k := load(t, testKB(t))
	first, err := Assemble(k).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Assemble(k).Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two runs over one KB produced different bytes")
	}
}

// A body is the markdown as it is on disk and not the frontmatter, because the
// frontmatter is already the fields beside it.
func TestPageCarriesItsBody(t *testing.T) {
	d := Assemble(load(t, testKB(t)))
	for _, p := range d.Pages {
		if p.Path != "pages/alpha.md" {
			continue
		}
		if !strings.Contains(p.Body, "[@bush1945]") {
			t.Errorf("the body is missing its prose:\n%s", p.Body)
		}
		if strings.Contains(p.Body, "---") || strings.Contains(p.Body, "aliases:") {
			t.Errorf("the body carries its frontmatter:\n%s", p.Body)
		}
		if p.Title != "Alpha" || p.Type != "concept" || len(p.Aliases) != 1 {
			t.Errorf("page = %+v", p)
		}
		return
	}
	t.Fatal("pages/alpha.md is not in the dump")
}

// The dump is where the KB's own reading of an entry is, not only what BibTeX
// says: the fields are kept as written and the title, year, authors and URL are
// the readings the citation formatter uses.
func TestBibliographyCarriesFieldsAndReadings(t *testing.T) {
	d := Assemble(load(t, testKB(t)))
	if len(d.Bibliography) != 1 {
		t.Fatalf("bibliography = %d entries, want 1", len(d.Bibliography))
	}
	e := d.Bibliography[0]
	if e.Key != "bush1945" || e.Type != "article" {
		t.Errorf("entry = %+v", e)
	}
	if e.Title != "As We May Think" || e.Year != "1945" {
		t.Errorf("readings = %q %q", e.Title, e.Year)
	}
	if len(e.Authors) != 1 || e.Authors[0] != "Bush, Vannevar" {
		t.Errorf("authors = %v", e.Authors)
	}
	if e.Fields["year"] != "1945" {
		t.Errorf("fields = %v", e.Fields)
	}
}

// Write creates the directory it was pointed at, because the default lives in
// .stemma/, which a KB need never have had.
func TestWriteCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "dump.json")
	res, err := Write(load(t, testKB(t)), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Out != dir || res.Bytes == 0 {
		t.Errorf("result = %+v", res)
	}
	if res.Pages != 3 || res.Links != 5 || res.Citations != 2 || res.Bibliography != 1 {
		t.Errorf("counts = %+v", res.Counts)
	}

	b, err := os.ReadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	var d Dump
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("what Write wrote is not the dump: %v", err)
	}
	if d.Version != Version {
		t.Errorf("version = %d, want %d", d.Version, Version)
	}
}

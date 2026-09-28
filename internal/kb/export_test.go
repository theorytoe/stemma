package kb

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// exportKB is a bibliography with one @string macro in use, one record whose
// type and field CSL knows, one that only BibTeX knows, and the tool's own
// provenance fields.
func exportKB(t *testing.T) *Bibliography {
	t.Helper()
	raw := []byte(`@string{acm = {Association for Computing Machinery}}

@article{bush1945,
  title = {As We May Think},
  author = {Bush, Vannevar},
  journal = acm,
  year = {1945},
  volume = {176},
  pages = {101--108},
  doi = {10.0000/example},
  stemma-retrieved = {2026-09-28},
  stemma-content-hash = {sha256:abc},
}

@book{cormen2009,
  title = {Introduction to Algorithms},
  author = {Cormen, Thomas H. and Leiserson, Charles E.},
  publisher = {MIT Press},
  year = {2009},
  edition = {3},
}

@misc{weird,
  title = {Something Odd},
  note = {kept},
}
`)
	b, err := ParseBibliography("bibliography.bib", raw)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExportBibTeXResolvesMacros(t *testing.T) {
	out := ExportBibTeX(exportKB(t).Entries())
	if strings.Contains(string(out), "@string") {
		t.Errorf("the export still carries a macro:\n%s", out)
	}

	f, err := ParseBibFile("out.bib", out)
	if err != nil {
		t.Fatalf("the export does not parse: %v\n%s", err, out)
	}
	e, ok := f.Entry("bush1945")
	if !ok {
		t.Fatal("bush1945 did not survive the export")
	}
	if v, _ := e.Value("journal"); v != "Association for Computing Machinery" {
		t.Errorf("journal = %q, want the macro resolved", v)
	}
	if v, _ := e.Value("title"); v != "As We May Think" {
		t.Errorf("title = %q", v)
	}
}

func TestExportBibTeXKeepsUnknownFieldsAndTypes(t *testing.T) {
	f, err := ParseBibFile("out.bib", ExportBibTeX(exportKB(t).Entries()))
	if err != nil {
		t.Fatal(err)
	}
	e, ok := f.Entry("weird")
	if !ok {
		t.Fatal("the entry CSL has no type for did not survive")
	}
	if e.Type() != "misc" {
		t.Errorf("type = %q, want misc", e.Type())
	}
	if v, _ := e.Value("note"); v != "kept" {
		t.Errorf("note = %q", v)
	}

	b, ok := f.Entry("bush1945")
	if !ok {
		t.Fatal("bush1945 did not survive")
	}
	if v, _ := b.Value(FieldContentHash); v != "sha256:abc" {
		t.Errorf("%s = %q, want the provenance to survive", FieldContentHash, v)
	}
	if v, _ := b.Value(FieldRetrieved); v != "2026-09-28" {
		t.Errorf("%s = %q", FieldRetrieved, v)
	}
}

func TestExportBibTeXIsSortedAndStable(t *testing.T) {
	entries := exportKB(t).Entries()
	first := ExportBibTeX(entries)
	if second := ExportBibTeX(entries); string(first) != string(second) {
		t.Errorf("two exports differ:\n%s\n%s", first, second)
	}
	if i, j := strings.Index(string(first), "bush1945"), strings.Index(string(first), "cormen2009"); i > j {
		t.Errorf("entries are not in key order:\n%s", first)
	}
}

// cslProbe is a CSL-JSON schema written here rather than derived from the
// writer, so that the export is read back by an independent definition.
type cslProbe struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Title          string `json:"title"`
	ContainerTitle string `json:"container-title"`
	Volume         string `json:"volume"`
	Page           string `json:"page"`
	DOI            string `json:"DOI"`
	Publisher      string `json:"publisher"`
	Edition        string `json:"edition"`
	Note           string `json:"note"`
	Author         []struct {
		Family string `json:"family"`
		Given  string `json:"given"`
	} `json:"author"`
	Issued *struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"issued"`
	Accessed *struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"accessed"`
}

func TestExportCSLJSONReadsBackAsCSL(t *testing.T) {
	raw, err := ExportCSLJSON(exportKB(t).Entries())
	if err != nil {
		t.Fatal(err)
	}

	var items []cslProbe
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("the export is not JSON: %v\n%s", err, raw)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}

	article := items[0]
	if article.ID != "bush1945" || article.Type != "article-journal" {
		t.Errorf("first item = %q/%q", article.ID, article.Type)
	}
	if article.Title != "As We May Think" {
		t.Errorf("title = %q", article.Title)
	}
	if article.ContainerTitle != "Association for Computing Machinery" {
		t.Errorf("container-title = %q, want the macro resolved", article.ContainerTitle)
	}
	if article.Volume != "176" || article.Page != "101--108" || article.DOI != "10.0000/example" {
		t.Errorf("volume/page/DOI = %q/%q/%q", article.Volume, article.Page, article.DOI)
	}
	if len(article.Author) != 1 || article.Author[0].Family != "Bush" || article.Author[0].Given != "Vannevar" {
		t.Errorf("author = %+v", article.Author)
	}
	if article.Issued == nil || !reflect.DeepEqual(article.Issued.DateParts, [][]int{{1945}}) {
		t.Errorf("issued = %+v", article.Issued)
	}
	if article.Accessed == nil || !reflect.DeepEqual(article.Accessed.DateParts, [][]int{{2026, 9, 28}}) {
		t.Errorf("accessed = %+v", article.Accessed)
	}

	book := items[1]
	if book.ID != "cormen2009" || book.Type != "book" {
		t.Errorf("second item = %q/%q", book.ID, book.Type)
	}
	if book.Publisher != "MIT Press" || book.Edition != "3" {
		t.Errorf("publisher/edition = %q/%q", book.Publisher, book.Edition)
	}
	if len(book.Author) != 2 || book.Author[1].Family != "Leiserson" {
		t.Errorf("author = %+v", book.Author)
	}

	odd := items[2]
	if odd.ID != "weird" || odd.Type != "document" {
		t.Errorf("third item = %q/%q, want an unnamed type to become a document", odd.ID, odd.Type)
	}
	if odd.Note != "kept" {
		t.Errorf("note = %q", odd.Note)
	}
}

func TestExportCSLJSONCarriesProvenanceThrough(t *testing.T) {
	raw, err := ExportCSLJSON(exportKB(t).Entries())
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	first := items[0]
	if first["stemma-content-hash"] != "sha256:abc" {
		t.Errorf("the content hash did not survive: %v", first)
	}
	if _, ok := first["stemma-retrieved"]; ok {
		t.Errorf("the retrieval date is still under its BibTeX name: %v", first)
	}
	if _, ok := first["accessed"]; !ok {
		t.Errorf("the retrieval date did not become accessed: %v", first)
	}
}

func TestExportCSLJSONIsStable(t *testing.T) {
	entries := exportKB(t).Entries()
	first, err := ExportCSLJSON(entries)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExportCSLJSON(entries)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("two exports differ")
	}
}

// The reader that matters is someone else's. These run the real TeX tools when
// they are installed and skip when they are not, so the check is a bonus on a
// machine that has them rather than a dependency of the suite.
func TestExportBibTeXParsesWithBibtex(t *testing.T) {
	bibtex, err := exec.LookPath("bibtex")
	if err != nil {
		t.Skip("bibtex is not installed")
	}
	dir := writeBibProbe(t, ExportBibTeX(exportKB(t).Entries()))

	cmd := exec.Command(bibtex, "probe")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bibtex rejected the export: %v\n%s", err, out)
	}
	bbl, err := os.ReadFile(filepath.Join(dir, "probe.bbl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"bush1945", "cormen2009", "weird"} {
		if !strings.Contains(string(bbl), key) {
			t.Errorf("bibtex did not keep %q:\n%s", key, bbl)
		}
	}
}

func TestExportBibTeXParsesWithBiber(t *testing.T) {
	biber, err := exec.LookPath("biber")
	if err != nil {
		t.Skip("biber is not installed")
	}
	dir := writeBibProbe(t, ExportBibTeX(exportKB(t).Entries()))

	cmd := exec.Command(biber, "--tool", "--output-format=bibtex", "out.bib")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("biber rejected the export: %v\n%s", err, out)
	}
}

// writeBibProbe lays out a minimal BibTeX run: the export, and an aux file that
// pulls in every entry.
func writeBibProbe(t *testing.T, bib []byte) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "out.bib"), bib, 0o644); err != nil {
		t.Fatal(err)
	}
	aux := "\\citation{*}\n\\bibdata{out}\n\\bibstyle{plain}\n"
	if err := os.WriteFile(filepath.Join(dir, "probe.aux"), []byte(aux), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

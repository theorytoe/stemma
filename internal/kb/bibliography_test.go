package kb

import (
	"os"
	"path/filepath"
	"testing"
)

func parseOne(t *testing.T, raw string) *BibEntry {
	t.Helper()
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	entries := f.Entries()
	if len(entries) != 1 {
		t.Fatalf("want one entry, got %d", len(entries))
	}
	return entries[0]
}

func TestSameWorkByKey(t *testing.T) {
	a := parseOne(t, "@article{key, title = {One}}\n")
	b := parseOne(t, "@book{key, title = {A different work}}\n")
	if !SameWork(a, b) {
		t.Error("the same key was not the same work")
	}
}

func TestSameWorkByIdentifier(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b string
	}{
		{
			"doi written two ways",
			"@article{a, doi = {10.1145/X}}\n",
			"@article{b, doi = {https://doi.org/10.1145/x}}\n",
		},
		{
			"arxiv with a version",
			"@misc{a, eprint = {1706.03762}}\n",
			"@misc{b, eprint = {arXiv:1706.03762v2}}\n",
		},
		{
			"isbn with separators",
			"@book{a, isbn = {978-0-262-03384-8}}\n",
			"@book{b, isbn = {9780262033848}}\n",
		},
		{
			"url with a trailing slash",
			"@misc{a, url = {https://example.com/x}}\n",
			"@misc{b, url = {https://example.com/x/}}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !SameWork(parseOne(t, tc.a), parseOne(t, tc.b)) {
				t.Error("the same work was not recognised")
			}
		})
	}
}

func TestSameWorkByTitleYearAndAuthor(t *testing.T) {
	a := parseOne(t, "@article{a, title = {Attention Is All You Need}, year = {2017}, author = {Vaswani, Ashish and others}}\n")
	b := parseOne(t, "@article{b, title = {attention is all you need}, year = {2017}, author = {Vaswani, Noam}}\n")
	if !SameWork(a, b) {
		t.Error("the same title, year and first author did not match")
	}
}

func TestDifferentWorksStayDifferent(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b string
	}{
		{
			"a different year",
			"@article{a, title = {Attention}, year = {2017}, author = {Vaswani, Ashish}}\n",
			"@article{b, title = {Attention}, year = {2018}, author = {Vaswani, Ashish}}\n",
		},
		{
			"a different first author",
			"@article{a, title = {Attention}, year = {2017}, author = {Vaswani, Ashish}}\n",
			"@article{b, title = {Attention}, year = {2017}, author = {Shazeer, Noam}}\n",
		},
		{
			"a different title, no other signal",
			"@article{a, title = {Attention}, year = {2017}, author = {Vaswani, Ashish}}\n",
			"@article{b, title = {Something Else}, year = {2017}, author = {Vaswani, Ashish}}\n",
		},
		{
			"no title to compare",
			"@article{a, year = {2017}, author = {Vaswani, Ashish}}\n",
			"@article{b, year = {2017}, author = {Vaswani, Ashish}}\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if SameWork(parseOne(t, tc.a), parseOne(t, tc.b)) {
				t.Error("two different works were called the same")
			}
		})
	}
}

func TestMergeFromKeepsTheKeyAndUnknownFields(t *testing.T) {
	dst := parseOne(t, "@article{old, custom = {keep me}, title = {Old}}\n")
	src := parseOne(t, "@article{new, title = {New}, year = {2020}}\n")

	dst.MergeFrom(src)

	if got := dst.Key(); got != "old" {
		t.Errorf("key = %q, want the existing key", got)
	}
	if got, _ := dst.Value("custom"); got != "keep me" {
		t.Errorf("custom = %q, want it untouched", got)
	}
	if got, _ := dst.Value("title"); got != "New" {
		t.Errorf("title = %q, want the new value", got)
	}
	if got, _ := dst.Value("year"); got != "2020" {
		t.Errorf("year = %q", got)
	}
}

func TestBibSourcesFindsBothForms(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, BibliographyName), []byte("@article{a,}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, BibliographyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b.bib", "c.bib", "ignored.txt"} {
		if err := os.WriteFile(filepath.Join(root, BibliographyDir, name), []byte("@article{x,}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	k := &KB{Root: root}
	got, err := k.BibSources()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{BibliographyName, "bibliography/b.bib", "bibliography/c.bib"}
	if len(got) != len(want) {
		t.Fatalf("sources = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sources = %q, want %q", got, want)
		}
	}
}

func TestBibliographyForCreatesTheSingleFile(t *testing.T) {
	root := t.TempDir()
	k := &KB{Root: root}

	f, name, err := k.BibliographyFor("smith2020widgets")
	if err != nil {
		t.Fatal(err)
	}
	if name != BibliographyName {
		t.Errorf("name = %q, want %q", name, BibliographyName)
	}
	if f.Len() != 0 {
		t.Errorf("the new file has %d entries", f.Len())
	}
}

func TestBibliographyForUsesTheDirectoryForm(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, BibliographyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	k := &KB{Root: root}

	_, name, err := k.BibliographyFor("smith:2020/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if want := "bibliography/smith-2020-widgets.bib"; name != want {
		t.Errorf("name = %q, want %q", name, want)
	}
}

func TestWriteBibliographyCreatesTheDirectoryAndRoundTrips(t *testing.T) {
	root := t.TempDir()
	k := &KB{Root: root}

	e := NewBibEntry("book", "key")
	e.Set("title", "{A Title}")
	e.Set("stemma-retrieved", "{2026-09-28}")

	name := "bibliography/key.bib"
	f, err := ParseBibFile(name, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.AddEntry(e)
	if err := k.WriteBibliography(name, f); err != nil {
		t.Fatal(err)
	}

	back, err := k.ReadBibliography(name)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := back.Entry("key")
	if !ok {
		t.Fatal("the entry did not come back")
	}
	if v, _ := got.Value("title"); v != "A Title" {
		t.Errorf("title = %q", v)
	}
}

func TestBibliographyForReusesTheFileThatDefinesTheKey(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, BibliographyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, BibliographyDir, "key.bib"), []byte("@article{key,}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, name, err := (&KB{Root: root}).BibliographyFor("key")
	if err != nil {
		t.Fatal(err)
	}
	if want := "bibliography/key.bib"; name != want {
		t.Errorf("name = %q, want %q", name, want)
	}
}

func TestBibliographyForAvoidsANameAnotherKeyOwns(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, BibliographyDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// "smith:2020" normalises to "smith-2020", which this file already owns.
	if err := os.WriteFile(filepath.Join(dir, "smith-2020.bib"), []byte("@article{other,}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, name, err := (&KB{Root: root}).BibliographyFor("smith:2020")
	if err != nil {
		t.Fatal(err)
	}
	if want := "bibliography/smith-2020-2.bib"; name != want {
		t.Errorf("name = %q, want %q", name, want)
	}
	if f.Len() != 0 {
		t.Errorf("the new file should be empty, it has %d entries", f.Len())
	}
}

func TestBibliographyForPrefersTheSingleFileWhenBothFormsExist(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, BibliographyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, BibliographyName), []byte("@article{one,}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, name, err := (&KB{Root: root}).BibliographyFor("two")
	if err != nil {
		t.Fatal(err)
	}
	if name != BibliographyName {
		t.Errorf("name = %q, want %q", name, BibliographyName)
	}
	if !f.Has("one") {
		t.Error("the single file was not read")
	}
}

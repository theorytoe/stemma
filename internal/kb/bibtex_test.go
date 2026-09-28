package kb

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestGoldenBibRoundTrip is the preservation guarantee applied to a
// bibliography: a file the tool has no reason to change comes back byte for
// byte, comments, macros, indentation and all.
//
// The corpus is export styles from the tools a person actually uses, plus the
// project's own bibliography, for the same reason the page corpus includes the
// example wiki: if the format is bad, the project's own documentation degrades
// first.
func TestGoldenBibRoundTrip(t *testing.T) {
	files, err := filepath.Glob("testdata/bibtex/roundtrip/*.bib")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("the bibliography round-trip corpus is empty")
	}
	files = append(files, filepath.Join(repoRoot(t), "wiki", "bibliography.bib"))

	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := ParseBibFile(path, raw)
			if err != nil {
				t.Fatalf("the corpus does not parse: %v", err)
			}
			if got := f.Bytes(); !bytes.Equal(got, raw) {
				t.Errorf("round trip changed the bibliography.\n--- got ---\n%s\n--- want ---\n%s", got, raw)
			}
		})
	}
}

func TestBibEntryStructure(t *testing.T) {
	raw := `@dataset{lee2021,
  title = {An Annotated Corpus},
  custom-field = {kept verbatim},
  year = {2021},
}
`
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if f.Len() != 1 {
		t.Fatalf("entries = %d, want 1", f.Len())
	}
	e, ok := f.Entry("lee2021")
	if !ok {
		t.Fatal("the entry is missing")
	}
	if got := e.Type(); got != "dataset" {
		t.Errorf("type = %q, want dataset", got)
	}
	if got := e.Key(); got != "lee2021" {
		t.Errorf("key = %q, want lee2021", got)
	}
	want := []string{"title", "custom-field", "year"}
	if got := e.FieldNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("field names = %q, want %q", got, want)
	}
	if got, _ := e.Raw("custom-field"); got != "{kept verbatim}" {
		t.Errorf("raw custom-field = %q", got)
	}
	// A field name is matched without case, which is how BibTeX reads one.
	if got, _ := e.Value("CUSTOM-FIELD"); got != "kept verbatim" {
		t.Errorf("value custom-field = %q", got)
	}
}

func TestBibValueDecoding(t *testing.T) {
	macros := map[string]string{
		"acm":    `"Association for Computing Machinery"`,
		"acmpub": `acm # ", New York"`,
	}
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"braced", "{As We May Think}", "As We May Think"},
		{"quoted", `"As We May Think"`, "As We May Think"},
		{"bare number", "2020", "2020"},
		{"brace protection kept", "{{T}ransformer}", "{T}ransformer"},
		{"whitespace collapsed", "{Attention\n      Is All You Need}", "Attention Is All You Need"},
		{"concatenation", `"part one" # " and part two"`, "part one and part two"},
		{"macro", "acm", "Association for Computing Machinery"},
		{"macro in a concatenation", "acmpub", "Association for Computing Machinery, New York"},
		{"undefined macro is kept", "feb", "feb"},
		{"hash inside braces is not a join", "{C#}", "C#"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := decodeBibValue(tc.raw, macros); got != tc.want {
				t.Errorf("decode(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestBibMacros(t *testing.T) {
	raw := `@string{jan = "January"}
@string{acm = "Association for Computing Machinery"}
@string{acmpub = acm # ", New York"}
@string{jan = "Jan"}
@article{k, month = jan, publisher = acmpub, note = feb}
`
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	// A later definition wins, which is what BibTeX does.
	if got, ok := f.Macro("jan"); !ok || got != "Jan" {
		t.Errorf("macro jan = %q, %v; want Jan", got, ok)
	}
	if got, ok := f.Macro("ACMPUB"); !ok || got != "Association for Computing Machinery, New York" {
		t.Errorf("macro acmpub = %q, %v", got, ok)
	}
	e, _ := f.Entry("k")
	if got, _ := e.Value("publisher"); got != "Association for Computing Machinery, New York" {
		t.Errorf("publisher = %q", got)
	}
	if got, _ := e.Value("month"); got != "Jan" {
		t.Errorf("month = %q", got)
	}
	// A name that is not a defined macro is returned as written.
	if got, _ := e.Value("note"); got != "feb" {
		t.Errorf("note = %q", got)
	}
}

// A macro that refers to itself must not send the decoder into a loop.
func TestBibMacroCycleTerminates(t *testing.T) {
	f, err := ParseBibFile("b.bib", []byte("@string{a = b}\n@string{b = a}\n@article{k, x = a}\n"))
	if err != nil {
		t.Fatal(err)
	}
	e, _ := f.Entry("k")
	if got, ok := e.Value("x"); !ok || got == "" {
		t.Errorf("x = %q, %v; want a non-empty value", got, ok)
	}
}

// @string, @preamble and @comment are not entries, and an entry-shaped thing
// inside a @comment is part of the comment.
func TestBibNonEntriesCarryNoKey(t *testing.T) {
	raw := `@string{jan = "January"}
@preamble{ "\newcommand{\noopsort}[1]{}" }
@comment{jabref-meta: databaseType:bibtex; and @article{fake,} inside}
@article{real,}
`
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Len(); got != 1 {
		t.Fatalf("entries = %d, want 1", got)
	}
	if !f.Has("real") || f.Has("fake") {
		t.Errorf("keys = %v", f.Entries())
	}
}

func TestBibParenthesisedEntry(t *testing.T) {
	raw := "@article(k, title = {A value with a (parenthesis) and a ) inside}, year = {2019})\n"
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	e, ok := f.Entry("k")
	if !ok {
		t.Fatal("the entry is missing")
	}
	if got, _ := e.Value("title"); got != "A value with a (parenthesis) and a ) inside" {
		t.Errorf("title = %q", got)
	}
	if got := string(f.Bytes()); got != raw {
		t.Errorf("round trip changed the entry: %q", got)
	}
}

func TestBibSetRendersInCanonicalShape(t *testing.T) {
	raw := "@article{k,\n  title = {Old},\n  year  = {2019},\n}\n"
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	e, _ := f.Entry("k")
	e.Set("title", "{New}")
	e.Set("author", "{Doe, John}")

	want := "@article{k,\n  title  = {New},\n  year   = {2019},\n  author = {Doe, John},\n}\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("bytes =\n%s\nwant\n%s", got, want)
	}
}

func TestBibDeleteRendersWithoutTheField(t *testing.T) {
	raw := "@article{k,\n  title = {Old},\n  year  = {2019},\n}\n"
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	e, _ := f.Entry("k")
	if !e.Delete("year") {
		t.Fatal("year was not there")
	}
	if e.Delete("year") {
		t.Error("deleting an absent field reported success")
	}
	want := "@article{k,\n  title = {Old},\n}\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("bytes =\n%s\nwant\n%s", got, want)
	}
}

func TestBibAddEntry(t *testing.T) {
	f, err := ParseBibFile("b.bib", []byte("@article{a,}\n"))
	if err != nil {
		t.Fatal(err)
	}
	e := NewBibEntry("book", "b")
	e.Set("title", "{B}")
	e.Set("author", "{Doe, John}")
	f.AddEntry(e)

	want := "@article{a,}\n@book{b,\n  title  = {B},\n  author = {Doe, John},\n}\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("bytes =\n%s\nwant\n%s", got, want)
	}
}

func TestBibAddEntryToAFileWithoutAFinalNewline(t *testing.T) {
	f, err := ParseBibFile("b.bib", []byte("@article{a,}"))
	if err != nil {
		t.Fatal(err)
	}
	f.AddEntry(NewBibEntry("misc", "b"))

	want := "@article{a,}\n@misc{b}\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("bytes = %q, want %q", got, want)
	}
}

func TestBibRemoveEntry(t *testing.T) {
	f, err := ParseBibFile("b.bib", []byte("@article{a,}\n@book{b,}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !f.RemoveEntry("a") {
		t.Fatal("a was not there")
	}
	if f.RemoveEntry("a") {
		t.Error("removing an absent key reported success")
	}
	if f.Has("a") || !f.Has("b") {
		t.Errorf("keys after removal: %v", f.Entries())
	}
	want := "@book{b,}\n"
	if got := string(f.Bytes()); got != want {
		t.Errorf("bytes = %q, want %q", got, want)
	}
}

func TestBibRefusesWhatItCannotLocate(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"not utf-8", "@article{k\xff,}\n"},
		{"entry never closed", "@article{k, title = {x}\n"},
		{"a quoted value never closed", "@article{k, title = \"x}\n"},
		{"a field that is not a pair", "@article{k, title {x}}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseBibFile("b.bib", []byte(tc.raw)); err == nil {
				t.Errorf("ParseBibFile accepted %q", tc.raw)
			}
		})
	}
}

// A file the tool cannot parse as entries is not a file it has to refuse: text
// that is not an entry is kept as text.
func TestBibKeepsTextThatIsNotAnEntry(t *testing.T) {
	raw := "Written by a@b.com about nothing in particular.\n"
	f, err := ParseBibFile("b.bib", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if f.Len() != 0 {
		t.Errorf("entries = %d, want none", f.Len())
	}
	if got := string(f.Bytes()); got != raw {
		t.Errorf("bytes = %q, want %q", got, raw)
	}
}

// The serializer must not read bytes it does not own when it extends a gap.
func TestBibAddEntryDoesNotDisturbTheInput(t *testing.T) {
	raw := []byte("leading text with no newline")
	original := append([]byte(nil), raw...)
	f, err := ParseBibFile("b.bib", raw)
	if err != nil {
		t.Fatal(err)
	}
	f.AddEntry(NewBibEntry("misc", "k"))
	if !bytes.Equal(raw, original) {
		t.Errorf("the input buffer changed: %q", raw)
	}
	if !strings.HasPrefix(string(f.Bytes()), "leading text with no newline\n@misc{k}") {
		t.Errorf("bytes = %q", f.Bytes())
	}
}

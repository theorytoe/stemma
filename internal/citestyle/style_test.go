package citestyle

import (
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

func entry(t *testing.T, bib string) *kb.BibEntry {
	t.Helper()
	f, err := kb.ParseBibFile("b.bib", []byte(bib))
	if err != nil {
		t.Fatal(err)
	}
	entries := f.Entries()
	if len(entries) != 1 {
		t.Fatalf("want one entry, got %d", len(entries))
	}
	return entries[0]
}

func refs(entries ...*kb.BibEntry) []kb.Reference {
	out := make([]kb.Reference, len(entries))
	for i, e := range entries {
		out[i] = kb.Reference{Entry: e}
	}
	return out
}

const bush = `@article{bush1945,
  author = {Bush, Vannevar},
  title = {As We May Think},
  journal = {The Atlantic Monthly},
  year = {1945},
  volume = {176},
  number = {1},
  pages = {101--108},
  doi = {10.1000/xyz},
}
`

func TestParse(t *testing.T) {
	for _, name := range []string{"", "author-date", "numeric"} {
		if _, err := Parse(name); err != nil {
			t.Errorf("Parse(%q): %v", name, err)
		}
	}
	if f, err := Parse(""); err != nil {
		t.Fatal(err)
	} else if _, ok := f.(authorDate); !ok {
		t.Errorf("the empty name should be the default style, got %T", f)
	}
	if _, err := Parse("apa"); err == nil {
		t.Error("an unknown style was accepted")
	} else if !strings.Contains(err.Error(), "unknown citation style") {
		t.Errorf("error = %v", err)
	}
}

func TestGroups(t *testing.T) {
	citations := []kb.Citation{
		{Key: "a", Group: 0, Position: 0},
		{Key: "b", Group: 0, Position: 1},
		{Key: "c", Group: 1, Narrative: true},
		{Key: "d", Group: 2},
	}
	got := Groups(citations)
	if len(got) != 3 {
		t.Fatalf("groups = %d, want 3", len(got))
	}
	if len(got[0]) != 2 || got[0][0].Key != "a" || got[0][1].Key != "b" {
		t.Errorf("group 0 = %+v", got[0])
	}
	if len(got[1]) != 1 || !got[1][0].Narrative {
		t.Errorf("group 1 = %+v", got[1])
	}
	if len(got[2]) != 1 || got[2][0].Key != "d" {
		t.Errorf("group 2 = %+v", got[2])
	}
}

func TestAuthorDateCite(t *testing.T) {
	e := entry(t, bush)
	other := entry(t, "@article{doe2020, author = {Doe, Jane}, year = {2020}}\n")
	two := entry(t, "@article{two2020, author = {Bush, Vannevar and Doe, Jane}, year = {2020}}\n")
	three := entry(t, "@article{three2020, author = {Bush, Vannevar and Doe, Jane and Roe, John}, year = {2020}}\n")
	list := refs(e, other, two, three)

	f, err := Parse("author-date")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		group []kb.Citation
		want  string
	}{
		{"parenthetical", []kb.Citation{{Key: "bush1945"}}, "(Bush 1945)"},
		{"locator", []kb.Citation{{Key: "bush1945", Locator: "p. 33"}}, "(Bush 1945, p. 33)"},
		{"suppressed author", []kb.Citation{{Key: "bush1945", SuppressAuthor: true}}, "(1945)"},
		{"prefix", []kb.Citation{{Key: "bush1945", Prefix: "see"}}, "(see Bush 1945)"},
		{"narrative", []kb.Citation{{Key: "bush1945", Narrative: true}}, "Bush (1945)"},
		{"two authors", []kb.Citation{{Key: "two2020"}}, "(Bush and Doe 2020)"},
		{"three authors", []kb.Citation{{Key: "three2020"}}, "(Bush et al. 2020)"},
		{"a group", []kb.Citation{{Key: "bush1945"}, {Key: "doe2020"}}, "(Bush 1945; Doe 2020)"},
		{"an unresolved key", []kb.Citation{{Key: "missing"}}, "(missing)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Cite(tc.group, list); got != tc.want {
				t.Errorf("Cite = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAuthorDateEntry(t *testing.T) {
	f, err := Parse("author-date")
	if err != nil {
		t.Fatal(err)
	}
	e := entry(t, bush)
	want := "Bush, Vannevar. 1945. As We May Think. The Atlantic Monthly, 176(1): 101--108. https://doi.org/10.1000/xyz."
	if got := f.Entry(kb.Reference{Entry: e}, 1); got != want {
		t.Errorf("Entry =\n%q\nwant\n%q", got, want)
	}
}

func TestAuthorDateEntryWithOnlyATitle(t *testing.T) {
	f, _ := Parse("author-date")
	e := entry(t, "@misc{k, title = {A Note}, year = {2020}}\n")
	if got, want := f.Entry(kb.Reference{Entry: e}, 1), "2020. A Note."; got != want {
		t.Errorf("Entry = %q, want %q", got, want)
	}
}

func TestNumericCite(t *testing.T) {
	f, err := Parse("numeric")
	if err != nil {
		t.Fatal(err)
	}
	a := entry(t, bush)
	b := entry(t, "@article{doe2020, author = {Doe, Jane}, year = {2020}}\n")
	list := refs(a, b)

	for _, tc := range []struct {
		name  string
		group []kb.Citation
		want  string
	}{
		{"single", []kb.Citation{{Key: "bush1945"}}, "[1]"},
		{"second", []kb.Citation{{Key: "doe2020"}}, "[2]"},
		{"a group", []kb.Citation{{Key: "bush1945"}, {Key: "doe2020"}}, "[1; 2]"},
		{"locator", []kb.Citation{{Key: "bush1945", Locator: "p. 33"}}, "[1, p. 33]"},
		{"an unresolved key", []kb.Citation{{Key: "missing"}}, "[missing]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.Cite(tc.group, list); got != tc.want {
				t.Errorf("Cite = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNumericEntry(t *testing.T) {
	f, _ := Parse("numeric")
	e := entry(t, bush)
	want := "[1] Bush, Vannevar. As We May Think. The Atlantic Monthly, 176(1): 101--108. 1945. https://doi.org/10.1000/xyz."
	if got := f.Entry(kb.Reference{Entry: e}, 1); got != want {
		t.Errorf("Entry =\n%q\nwant\n%q", got, want)
	}
}

func TestNameHandling(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Bush, Vannevar", "Bush, Vannevar"},
		{"Vannevar Bush", "Bush, Vannevar"},
		{"{von Neumann}, John", "von Neumann, John"},
		{"Plato", "Plato"},
	} {
		if got := formatName(tc.in); got != tc.want {
			t.Errorf("formatName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := surname("Bush, Vannevar"); got != "Bush" {
		t.Errorf("surname = %q", got)
	}
	if got := surname("Vannevar Bush"); got != "Bush" {
		t.Errorf("surname = %q", got)
	}
}

func TestReferenceListOfNames(t *testing.T) {
	e := entry(t, "@article{k, author = {Bush, Vannevar and Doe, Jane and Roe, John}, year = {2020}}\n")
	if got, want := authorList(e), "Bush, Vannevar, Doe, Jane, and Roe, John"; got != want {
		t.Errorf("authorList = %q, want %q", got, want)
	}
}

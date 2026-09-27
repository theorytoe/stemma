package kb

import (
	"reflect"
	"strings"
	"testing"
)

func bibKeys(t *testing.T, raw string) []string {
	t.Helper()
	b, err := ParseBibliography("bibliography.bib", []byte(raw))
	if err != nil {
		t.Fatalf("ParseBibliography: %v", err)
	}
	return b.Keys()
}

func TestParseBibliographyFindsKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "one entry",
			raw:  "@article{vaswani2017,\n  title = {Attention},\n}\n",
			want: []string{"vaswani2017"},
		},
		{
			name: "several entries keep their order",
			raw:  "@article{b,\n}\n@book{a,\n}\n@misc{c,\n}\n",
			want: []string{"b", "a", "c"},
		},
		{
			name: "parentheses instead of braces",
			raw:  "@article(key1, title = {x})\n",
			want: []string{"key1"},
		},
		{
			name: "braces nested inside a value",
			raw:  "@article{k, title = {a {b} c}, note = {d}}\n",
			want: []string{"k"},
		},
		{
			name: "a brace inside a quoted value is not structural",
			raw:  "@article{k, title = \"a } brace\", year = 2017}\n",
			want: []string{"k"},
		},
		{
			name: "an escaped quote does not end a value",
			raw:  "@article{k, title = \"a \\\" b }\", year = 1}\n",
			want: []string{"k"},
		},
		{
			name: "an address inside a value is not an entry",
			raw:  "@article{k, note = {mail me@example.com}}\n",
			want: []string{"k"},
		},
		{
			name: "an address between entries is not an entry",
			raw:  "written by a@b.com\n@article{k,}\n",
			want: []string{"k"},
		},
		{
			name: "string preamble and comment carry no key",
			raw:  "@string{jan = \"January\"}\n@preamble{ \"\\x\" }\n@comment{anything at all}\n@article{k,}\n",
			want: []string{"k"},
		},
		{
			name: "entry types are case insensitive",
			raw:  "@String{x = \"y\"}\n@ARTICLE{k,}\n",
			want: []string{"k"},
		},
		{
			name: "no comma after the key",
			raw:  "@article{k}\n",
			want: []string{"k"},
		},
		{
			name: "whitespace before the key",
			raw:  "@article{  k ,\n}\n",
			want: []string{"k"},
		},
		{
			name: "no entries at all",
			raw:  "This file is empty of entries.\n",
			want: nil,
		},
		{
			name: "an empty file",
			raw:  "",
			want: nil,
		},
		{
			name: "keys with punctuation",
			raw:  "@misc{smith:2017/a+b,}\n",
			want: []string{"smith:2017/a+b"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := bibKeys(t, tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("keys = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("keys = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestParseBibliographyReportsDuplicates(t *testing.T) {
	b, err := ParseBibliography("bibliography.bib", []byte("@article{k,}\n@book{k,}\n@misc{other,}\n"))
	if err != nil {
		t.Fatalf("ParseBibliography: %v", err)
	}
	if got := b.Duplicates(); !reflect.DeepEqual(got, []string{"k"}) {
		t.Errorf("Duplicates = %q, want [k]", got)
	}
	if got := b.Len(); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}

func TestBibliographyHasIsCaseSensitive(t *testing.T) {
	b, err := ParseBibliography("bibliography.bib", []byte("@article{Vaswani2017,}\n"))
	if err != nil {
		t.Fatalf("ParseBibliography: %v", err)
	}
	if !b.Has("Vaswani2017") {
		t.Error("the exact key was not found")
	}
	if b.Has("vaswani2017") {
		t.Error("keys are identifiers, so a different case is a different key")
	}
}

func TestParseBibliographyRefusesUnreadableFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"not utf-8", "@article{k\xff,}\n"},
		{"entry never closed", "@article{k, title = {x}\n"},
		{"entry never closed with a nested brace", "@article{k, title = {x}\n@book{j,}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseBibliography("bibliography.bib", []byte(tc.raw)); err == nil {
				t.Errorf("ParseBibliography accepted %q", tc.raw)
			}
		})
	}
}

func TestCitationForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"parenthetical", "text [@key].\n", []string{"key"}},
		{"grouped", "text [@a; @b].\n", []string{"a", "b"}},
		{"with a locator", "text [@key, p. 33].\n", []string{"key"}},
		{"narrative", "text @key says.\n", []string{"key"}},
		{"author suppressed", "text [-@key].\n", []string{"key"}},
		{"with a prefix", "text [see @key].\n", []string{"key"}},
		{"at the start of a line", "@key\n", []string{"key"}},
		{"at the start of the file", "@key opens.\n", []string{"key"}},
		{"a sentence ending after a key", "as @key.\n", []string{"key"}},
		{"punctuation inside a key", "[@smith:2017/a+b].\n", []string{"smith:2017/a+b"}},
		{"underscore and digits", "[@a_1].\n", []string{"a_1"}},
		{"no citation", "just prose.\n", nil},
		{"an address is not a citation", "mail user@example.com.\n", nil},
		{"an address in brackets is not a citation", "[user@example.com]\n", nil},
		{"a bare at sign", "at @ sign\n", nil},
		{"inside a code span", "`[@key]`\n", nil},
		{"inside a fenced block", "```\n[@key]\n```\n", nil},
		{"two on one line", "[@a] and [@b]\n", []string{"a", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := pageWithBody(t, tc.body).Citations()
			keys := make([]string, 0, len(got))
			for _, c := range got {
				keys = append(keys, c.Key)
			}
			if len(keys) != len(tc.want) {
				t.Fatalf("citations = %q, want %q", keys, tc.want)
			}
			for i := range keys {
				if keys[i] != tc.want[i] {
					t.Fatalf("citations = %q, want %q", keys, tc.want)
				}
			}
		})
	}
}

func TestCitationsCarryTheirLine(t *testing.T) {
	p := pageWithBody(t, "first\n\nthird [@key]\n")
	got := p.Citations()
	if len(got) != 1 {
		t.Fatalf("citations = %+v", got)
	}
	// Four lines of frontmatter, so the body starts at line 5.
	if got[0].Line != 7 {
		t.Errorf("line = %d, want 7", got[0].Line)
	}
}

func TestCitationFindings(t *testing.T) {
	raw := "@article{present,}\n@article{uncited,}\n@article{present,}\n"
	b, err := ParseBibliography("bibliography.bib", []byte(raw))
	if err != nil {
		t.Fatalf("ParseBibliography: %v", err)
	}

	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", "cites [@present] and [@absent]\n"))

	got := g.CitationFindings(b, Lenient)
	want := []string{CodeCitationMissing, CodeCitationUncited, CodeCitationDuplicate}

	have := map[string]bool{}
	for _, f := range got {
		have[f.Code] = true
	}
	for _, code := range want {
		if !have[code] {
			t.Errorf("no %s finding in %+v", code, got)
		}
	}

	for _, f := range got {
		if f.Code == CodeCitationMissing {
			if f.Path != "pages/a.md" || f.Line != 5 {
				t.Errorf("missing-citation finding = %+v", f)
			}
			if !strings.Contains(f.Message, "absent") {
				t.Errorf("message = %q", f.Message)
			}
		} else if f.Path != "bibliography.bib" {
			t.Errorf("%s finding is not attributed to the bibliography: %+v", f.Code, f)
		}
		if f.Severity != Warning {
			t.Errorf("%s severity = %q, want %q", f.Code, f.Severity, Warning)
		}
	}
}

func TestCitationFindingsUnderStrict(t *testing.T) {
	b, err := ParseBibliography("bibliography.bib", []byte("@article{uncited,}\n"))
	if err != nil {
		t.Fatalf("ParseBibliography: %v", err)
	}
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", ""))

	got := withCode(g.CitationFindings(b, Strict), CodeCitationUncited)
	if len(got) != 1 {
		t.Fatalf("findings = %+v", got)
	}
	if got[0].Severity != Error {
		t.Errorf("severity = %q, want %q", got[0].Severity, Error)
	}
}

func TestCleanCitationsProduceNoFindings(t *testing.T) {
	b, err := ParseBibliography("bibliography.bib", []byte("@article{key,}\n"))
	if err != nil {
		t.Fatalf("ParseBibliography: %v", err)
	}
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", "cites [@key]\n"))

	if got := g.CitationFindings(b, Strict); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

// The citation index is maintained the same way the link index is, so replacing
// a page must not leave the old citations behind.
func TestReplacingAPageUpdatesItsCitations(t *testing.T) {
	g := NewGraph()
	g.Add("pages/a.md", page(t, "A", "type: concept", "[@old]\n"))
	if got := g.CitedKeys(); !reflect.DeepEqual(got, []string{"old"}) {
		t.Fatalf("cited keys = %q", got)
	}

	g.Add("pages/a.md", page(t, "A", "type: concept", "[@new]\n"))
	if got := g.CitedKeys(); !reflect.DeepEqual(got, []string{"new"}) {
		t.Errorf("cited keys = %q, want [new]", got)
	}
	if got := g.CitedBy("old"); len(got) != 0 {
		t.Errorf("the old citation survived: %q", got)
	}

	g.Remove("pages/a.md")
	if got := g.CitedKeys(); len(got) != 0 {
		t.Errorf("cited keys = %q, want none", got)
	}
}

// Inside a braced value a double quote is an ordinary character. Reading it as
// the start of a quoted string sends the scan hunting for a closing quote that
// is not there, and a valid bibliography is then refused as an entry that is
// never closed. Inch marks and quoted words in titles are ordinary, so this
// refuses real files.
func TestAQuoteInsideABracedValueIsNotAQuote(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry string
		want  []string
	}{
		{"an inch mark",
			"@article{key1,\n  title = {A 12\" telescope},\n  year = {1957},\n}\n",
			[]string{"key1"}},
		{"an unbalanced quote",
			"@article{key1,\n  note = {He said \"yes and left},\n}\n",
			[]string{"key1"}},
		{"a quote in a later entry",
			"@article{key1,\n  title = {5\" of rain},\n}\n\n@article{key2,\n  title = {Ordinary},\n}\n",
			[]string{"key1", "key2"}},
	} {
		b, err := ParseBibliography("bibliography.bib", []byte(tc.entry))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got := b.Keys(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: keys = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A quoted value is still a quoted value: braces inside it are literal, and the
// quote that ends it is the one that ends the entry's value, not the entry.
func TestAQuotedValueIsStillQuoted(t *testing.T) {
	b, err := ParseBibliography("b.bib", []byte("@article{key1,\n  title = \"A {brace} inside\",\n  year = \"1957\",\n}\n\n@article{key2,\n  title = {Second},\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !b.Has("key1") || !b.Has("key2") {
		t.Errorf("keys = %q, want key1 and key2", b.Keys())
	}
}

// An entry whose quoted value really is never closed is still refused.
func TestAnUnclosedQuoteIsStillRefused(t *testing.T) {
	if _, err := ParseBibliography("b.bib", []byte("@article{key1,\n  title = \"never closed,\n}\n")); err == nil {
		t.Error("an unterminated quoted value was accepted")
	}
}

package kb

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckFindingsFindsEverythingItShould(t *testing.T) {
	b, err := ParseBibliography("b.bib", []byte(`@article{dup,}
@book{dup,}
@article{one, doi = {10.1145/x}}
@article{two, doi = {10.1145/x}}
@article{bad, stemma-retrieved = {yesterday}, stemma-content-hash = {sha256:zz}}
`))
	if err != nil {
		t.Fatal(err)
	}
	g := NewGraph()
	g.Add("pages/p.md", pageWithBody(t, "see [@absent]\n"))
	k := &KB{Graph: g, Bibliography: b}

	have := map[string]int{}
	for _, f := range k.CheckFindings() {
		have[f.Code]++
	}
	for _, code := range []string{
		CodeCitationDuplicate,
		CodeDuplicateWork,
		CodeCitationMissing,
		CodeMissingRetrieved,
		CodeMissingHash,
		CodeBadRetrieved,
		CodeBadHash,
	} {
		if have[code] == 0 {
			t.Errorf("no %s finding in %v", code, have)
		}
	}
}

func TestCheckFindingsNamesTheDuplicateWork(t *testing.T) {
	b, err := ParseBibliography("b.bib", []byte(
		"@article{one, doi = {10.1145/x}, stemma-retrieved = {2026-09-28}, stemma-content-hash = {sha256:ab}}\n"+
			"@article{two, doi = {https://doi.org/10.1145/X}, stemma-retrieved = {2026-09-28}, stemma-content-hash = {sha256:cd}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	k := &KB{Graph: NewGraph(), Bibliography: b}

	found := false
	for _, f := range k.CheckFindings() {
		if f.Code != CodeDuplicateWork {
			continue
		}
		found = true
		if !strings.Contains(f.Message, "one") || !strings.Contains(f.Message, "two") {
			t.Errorf("message = %q, want both keys", f.Message)
		}
	}
	if !found {
		t.Fatal("the same work under two keys was not reported")
	}
}

func TestCheckFindingsIsCleanOnAGoodRecord(t *testing.T) {
	b, err := ParseBibliography("b.bib", []byte(
		"@article{ok, stemma-retrieved = {2026-09-28}, stemma-content-hash = {sha256:ab12}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	g := NewGraph()
	g.Add("pages/p.md", pageWithBody(t, "cite [@ok]\n"))
	k := &KB{Graph: g, Bibliography: b}

	if got := k.CheckFindings(); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestValidProvenance(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"2026-09-28", true},
		{"yesterday", false},
		{"2026-9-28", false},
		{"", false},
	} {
		if got := validRetrieved(tc.value); got != tc.valid {
			t.Errorf("validRetrieved(%q) = %v, want %v", tc.value, got, tc.valid)
		}
	}

	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"sha256:ab12", true},
		{"sha256:ABCDEF", false},
		{"sha256:abc", false},
		{"sha256:", false},
		{"sha256", false},
		{"sha-256:ab", true},
		{"", false},
	} {
		if got := validHash(tc.value); got != tc.valid {
			t.Errorf("validHash(%q) = %v, want %v", tc.value, got, tc.valid)
		}
	}
}

func TestBibEntryAccessors(t *testing.T) {
	e := parseOne(t, "@article{k, title = {A Study}, author = {Doe, Jane and Roe, John}, year = {2019-04-01}}\n")
	if got := e.Title(); got != "A Study" {
		t.Errorf("Title = %q", got)
	}
	if got := e.Year(); got != "2019" {
		t.Errorf("Year = %q", got)
	}
	if got := e.Authors(); len(got) != 2 || got[0] != "Doe, Jane" || got[1] != "Roe, John" {
		t.Errorf("Authors = %q", got)
	}
}

// The duplicate check indexes records by what makes them one work, so its cost
// is what the duplicates cost rather than the square of the bibliography. This
// plants two duplicates in a large bibliography and asks for exactly those two,
// which is the case the pairwise form spent seconds on while finding nothing.
func TestCheckFindingsFindsDuplicatesInALargeBibliography(t *testing.T) {
	var b strings.Builder
	const n = 2000
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "@article{key%04d,\n  title = {Distinct Work %d},\n  author = {Author%04d, First},\n  year = {%d},\n  doi = {10.1000/%d},\n}\n\n",
			i, i, i, 1900+i%100, i)
	}
	// One duplicate by identifier, in another spelling and another case.
	b.WriteString("@article{dupdois,\n  title = {Some Other Title},\n  author = {Else, Someone},\n  year = {2021},\n  doi = {https://doi.org/10.1000/7},\n}\n\n")
	// One with no identifier at all, which only title, year and first author
	// can catch.
	b.WriteString("@article{duptitle,\n  title = {Distinct Work 42},\n  author = {Author0042, First},\n  year = {1942},\n}\n")

	bb, err := ParseBibliography("b.bib", []byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	k := &KB{Graph: NewGraph(), Bibliography: bb}

	var got []string
	for _, f := range k.CheckFindings() {
		if f.Code == CodeDuplicateWork {
			got = append(got, f.Message)
		}
	}
	if len(got) != 2 {
		t.Fatalf("duplicate-work findings = %d, want 2: %v", len(got), got)
	}
	if !strings.Contains(got[0], "dupdois") || !strings.Contains(got[1], "duptitle") {
		t.Errorf("findings = %v", got)
	}
}

func BenchmarkDuplicateWorkFindings(b *testing.B) {
	var src strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&src, "@article{key%04d,\n  title = {Distinct Work %d},\n  author = {Author%04d, First},\n  year = {%d},\n  doi = {10.1000/%d},\n}\n\n",
			i, i, i, 1900+i%100, i)
	}
	bb, err := ParseBibliography("b.bib", []byte(src.String()))
	if err != nil {
		b.Fatal(err)
	}
	k := &KB{Graph: NewGraph(), Bibliography: bb}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if n := len(k.duplicateWorkFindings()); n != 0 {
			b.Fatalf("findings = %d", n)
		}
	}
}

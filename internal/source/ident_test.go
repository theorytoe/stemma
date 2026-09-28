package source

import (
	"errors"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

func TestDetect(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want Kind
	}{
		{"10.1145/3375637", KindDOI},
		{"https://doi.org/10.1145/3375637", KindDOI},
		{"doi:10.1145/3375637", KindDOI},
		{"DOI:10.1145/3375637", KindDOI},
		{"arXiv:1706.03762", KindarXiv},
		{"1706.03762", KindarXiv},
		{"1706.03762v2", KindarXiv},
		{"hep-th/9901001", KindarXiv},
		{"math.GT/0309136", KindarXiv},
		{"978-0-262-03384-8", KindISBN},
		{"0262033844", KindISBN},
		{"https://example.com/a", KindURL},
		{"http://example.com", KindURL},
		{"not an identifier", KindUnknown},
		{"@article{k,}", KindUnknown},
		{"", KindUnknown},
	} {
		if got := Detect(tc.id); got != tc.want {
			t.Errorf("Detect(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestValidISBN(t *testing.T) {
	for _, tc := range []struct {
		isbn string
		want bool
	}{
		{"978-0-262-03384-8", true},
		{"9780262033848", true},
		{"0262033844", true},
		{"026203384X", false},
		{"0-306-40615-2", true},
		{"9780262033849", false},
		{"12345", false},
	} {
		if got := validISBN(tc.isbn); got != tc.want {
			t.Errorf("validISBN(%q) = %v, want %v", tc.isbn, got, tc.want)
		}
	}
}

func TestCiteKey(t *testing.T) {
	for _, tc := range []struct {
		name    string
		authors []string
		date    string
		title   string
		want    string
	}{
		{
			name:    "family name first",
			authors: []string{"Cormen, Thomas H."},
			date:    "2009",
			title:   "Introduction to Algorithms",
			want:    "cormen2009introduction",
		},
		{
			name:    "given name first",
			authors: []string{"Ashish Vaswani"},
			date:    "2017-06-12",
			title:   "Attention Is All You Need",
			want:    "vaswani2017attention",
		},
		{
			name:    "stopword skipped",
			authors: []string{"Doe, Jane"},
			date:    "2019",
			title:   "A Study of Things",
			want:    "doe2019study",
		},
		{
			name:  "no author falls back to the year and title",
			date:  "2020",
			title: "On Widgets",
			want:  "2020widgets",
		},
		{
			name:    "accent folded",
			authors: []string{"Müller, Jörg"},
			date:    "2021",
			title:   "Über Quanten",
			want:    "muller2021uber",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := CiteKey(tc.authors, tc.date, tc.title, ""); got != tc.want {
				t.Errorf("CiteKey = %q, want %q", got, tc.want)
			}
		})
	}

	if got := CiteKey(nil, "", "", "10.1/x"); got != "101x" {
		t.Errorf("CiteKey fallback = %q, want 101x", got)
	}
	if got := CiteKey(nil, "", "", ""); got != "source" {
		t.Errorf("CiteKey empty = %q, want source", got)
	}
}

func TestKindOfAndOperational(t *testing.T) {
	for _, tc := range []struct {
		err         error
		wantKind    ErrorKind
		operational bool
	}{
		{notFound("x", "gone"), ErrNotFound, false},
		{invalid("x", "bad"), ErrInvalid, false},
		{offline("x"), ErrOffline, true},
		{network("x", errBoom), ErrNetwork, true},
	} {
		kind, ok := KindOf(tc.err)
		if !ok || kind != tc.wantKind {
			t.Errorf("KindOf(%v) = %v, %v; want %v", tc.err, kind, ok, tc.wantKind)
		}
		if got := Operational(tc.err); got != tc.operational {
			t.Errorf("Operational(%v) = %v, want %v", tc.err, got, tc.operational)
		}
	}

	if _, ok := KindOf(errBoom); ok {
		t.Error("KindOf accepted an error that is not a resolution failure")
	}
	if Operational(errBoom) {
		t.Error("Operational accepted an error that is not a resolution failure")
	}
}

func TestManualBuildsFromFields(t *testing.T) {
	e, err := Manual(Fields{
		Title:     "A Study of Things",
		Authors:   []string{"Doe, Jane", "Roe, John"},
		Year:      "2019-04-01",
		Container: "Journal of Things",
		Volume:    "12",
		Issue:     "3",
		Pages:     "1-10",
		DOI:       "https://doi.org/10.1/x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Type(); got != "article" {
		t.Errorf("type = %q, want article", got)
	}
	if got := e.Key(); got != "doe2019study" {
		t.Errorf("key = %q, want doe2019study", got)
	}
	if got, _ := e.Value("journal"); got != "Journal of Things" {
		t.Errorf("journal = %q", got)
	}
	if got, _ := e.Value("doi"); got != "10.1/x" {
		t.Errorf("doi = %q", got)
	}
	if got, _ := e.Value("author"); got != "Doe, Jane and Roe, John" {
		t.Errorf("author = %q", got)
	}
	// A hand-entered record has no retrieval date, because nothing was fetched.
	if _, ok := e.Raw(kb.FieldRetrieved); ok {
		t.Error("a manual record carries a retrieval date")
	}
}

func TestManualInfersBookAndMisc(t *testing.T) {
	book, err := Manual(Fields{Title: "Introduction to Algorithms", ISBN: "978-0-262-03384-8"})
	if err != nil {
		t.Fatal(err)
	}
	if book.Type() != "book" {
		t.Errorf("isbn type = %q, want book", book.Type())
	}
	misc, err := Manual(Fields{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if misc.Type() != "misc" {
		t.Errorf("url type = %q, want misc", misc.Type())
	}
}

func TestManualRefusesAnEmptyRecord(t *testing.T) {
	if _, err := Manual(Fields{}); err == nil {
		t.Error("Manual accepted a record with nothing in it")
	}
}

func TestManualarXivFields(t *testing.T) {
	e, err := Manual(Fields{Title: "Attention", ArXiv: "arXiv:1706.03762"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := e.Value("eprint"); got != "1706.03762" {
		t.Errorf("eprint = %q", got)
	}
	if got, _ := e.Value("archivePrefix"); got != "arXiv" {
		t.Errorf("archivePrefix = %q", got)
	}
}

var errBoom = errors.New("boom")

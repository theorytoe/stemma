package kb

import "testing"

// The identifier rules are shared by duplicate detection and by resolution, so
// these pin the rule rather than the caller: one DOI must normalise one way
// whichever side asks.
func TestNormalizeDOI(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"10.1145/3375637", "10.1145/3375637"},
		{"doi:10.1145/3375637", "10.1145/3375637"},
		{"DOI:10.1145/3375637", "10.1145/3375637"},
		{"https://doi.org/10.1145/3375637", "10.1145/3375637"},
		{"http://dx.doi.org/10.1145/3375637", "10.1145/3375637"},
		{"https://doi.org/10.1145/3375637?x=1", "10.1145/3375637"},
		{"https://doi.org/10.1145/3375637#section", "10.1145/3375637"},
		// Case and query were the two differences that once made one DOI look
		// like two records.
		{"10.1145/AbC", "10.1145/abc"},
	} {
		if got := NormalizeDOI(tc.in); got != tc.want {
			t.Errorf("NormalizeDOI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeArXiv(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"1706.03762", "1706.03762"},
		{"arXiv:1706.03762", "1706.03762"},
		{"https://arxiv.org/abs/1706.03762", "1706.03762"},
		{"https://arxiv.org/pdf/1706.03762.pdf", "1706.03762"},
		{"hep-th/9901001", "hep-th/9901001"},
		// A version is kept here, because it names the version to fetch.
		{"1706.03762v2", "1706.03762v2"},
	} {
		if got := NormalizeArXiv(tc.in); got != tc.want {
			t.Errorf("NormalizeArXiv(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestArXivIdentityDropsTheVersion(t *testing.T) {
	for _, in := range []string{
		"1706.03762v2",
		"arXiv:1706.03762v2",
		"https://arxiv.org/abs/1706.03762v2",
	} {
		if got := ArXivIdentity(in); got != "1706.03762" {
			t.Errorf("ArXivIdentity(%q) = %q, want 1706.03762", in, got)
		}
	}
	// An old-style identifier whose text contains a "v" is not a version.
	for _, in := range []string{"hep-th/9901001", "solv-int/9901001"} {
		if got := ArXivIdentity(in); got != in {
			t.Errorf("ArXivIdentity(%q) = %q, want it unchanged", in, got)
		}
	}
}

func TestNormalizeISBN(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"978-0-262-03384-8", "9780262033848"},
		{"978 0 262 03384 8", "9780262033848"},
		{"080442957X", "080442957x"},
	} {
		if got := NormalizeISBN(tc.in); got != tc.want {
			t.Errorf("NormalizeISBN(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	if got := NormalizeURL("  https://example.com/x/  "); got != "https://example.com/x" {
		t.Errorf("NormalizeURL = %q", got)
	}
}

func TestYearOf(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"2009", "2009"},
		{"2009-07-31", "2009"},
		{"July 2009", "2009"},
		{"n.d.", ""},
		{"12009", ""},
		{"120090", ""},
	} {
		if got := YearOf(tc.in); got != tc.want {
			t.Errorf("YearOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

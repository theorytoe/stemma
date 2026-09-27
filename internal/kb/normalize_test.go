package kb

import "testing"

func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Attention Is All You Need", "attention-is-all-you-need"},
		{"attention is all you need", "attention-is-all-you-need"},
		{"attention-is-all-you-need", "attention-is-all-you-need"},
		{"Attention  Is   All", "attention-is-all"},
		{"  padded  ", "padded"},
		{"C++", "c++"},
		{"C#", "c#"},
		{"C", "c"},
		{"Go (language)", "go-language"},
		{"a/b", "a-b"},
		{"a.b.c", "a-b-c"},
		{"---", ""},
		{"", ""},
		{"   ", ""},
		{"...", ""},
		{"don't", "don-t"},
		{"e.g.", "e-g"},
		{"A9", "a9"},
		{"Über", "über"},
		{"日本語", "日本語"},
		{"[[bracketed]]", "bracketed"},
	} {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The three spellings the format promises are one name have to actually be one
// name.
func TestNormalizeFoldsTheThreeSpellings(t *testing.T) {
	a := Normalize("Attention Is All You Need")
	b := Normalize("attention is all you need")
	c := Normalize("attention-is-all-you-need")
	if a != b || b != c {
		t.Errorf("the three spellings normalise to %q, %q, %q", a, b, c)
	}
}

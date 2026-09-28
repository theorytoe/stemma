package kb

import "testing"

// SplitName is the one reader duplicate detection, the CSL export and the
// built-in formatters all use, so its answers are pinned here rather than three
// times over at the call sites.
func TestSplitName(t *testing.T) {
	for _, tc := range []struct {
		in     string
		family string
		given  string
		suffix string
		lit    string
	}{
		{in: "Bush, Vannevar", family: "Bush", given: "Vannevar"},
		{in: "Vannevar Bush", family: "Bush", given: "Vannevar"},
		{in: "Plato", family: "Plato"},
		{in: "von Neumann, John", family: "von Neumann", given: "John"},
		{in: "John von Neumann", family: "von Neumann", given: "John"},
		{in: "{von Neumann}, John", family: "von Neumann", given: "John"},
		// BibTeX orders a suffixed name "von Last, Jr, First".
		{in: "Knuth, Jr, Donald E.", family: "Knuth", given: "Donald E.", suffix: "Jr"},
		{in: "{The MITRE Corporation}", lit: "The MITRE Corporation"},
		{in: "others", lit: "others"},
		{in: "  ", family: "", given: ""},
	} {
		n, ok := SplitName(tc.in)
		if tc.in == "  " {
			if ok {
				t.Errorf("SplitName(%q) reported a name", tc.in)
			}
			continue
		}
		if !ok {
			t.Fatalf("SplitName(%q) found no name", tc.in)
		}
		if n.Family != tc.family || n.Given != tc.given || n.Suffix != tc.suffix || n.Literal != tc.lit {
			t.Errorf("SplitName(%q) = %+v, want family=%q given=%q suffix=%q literal=%q",
				tc.in, n, tc.family, tc.given, tc.suffix, tc.lit)
		}
	}
}

func TestSplitNameListKeepsABracedNameTogether(t *testing.T) {
	names := SplitNameList("{Smith and Sons} and Doe, Jane and others")
	if len(names) != 3 {
		t.Fatalf("names = %+v, want 3", names)
	}
	if names[0].Literal != "Smith and Sons" {
		t.Errorf("names[0] = %+v, want the braced name kept whole", names[0])
	}
	if names[1].Family != "Doe" || names[1].Given != "Jane" {
		t.Errorf("names[1] = %+v", names[1])
	}
	if names[2].Literal != "others" {
		t.Errorf("names[2] = %+v", names[2])
	}
}

func TestSurname(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Bush, Vannevar", "Bush"},
		{"Vannevar Bush", "Bush"},
		{"John von Neumann", "von Neumann"},
		{"{The MITRE Corporation}", "The MITRE Corporation"},
		{"", ""},
	} {
		if got := Surname(tc.in); got != tc.want {
			t.Errorf("Surname(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPlainName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Bush, Vannevar", "Bush, Vannevar"},
		{"Vannevar Bush", "Bush, Vannevar"},
		{"Plato", "Plato"},
		{"{The MITRE Corporation}", "The MITRE Corporation"},
	} {
		n, ok := SplitName(tc.in)
		if !ok {
			t.Fatalf("SplitName(%q) found no name", tc.in)
		}
		if got := n.Plain(); got != tc.want {
			t.Errorf("SplitName(%q).Plain() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStripBraces(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"As We May Think", "As We May Think"},
		{"{As We May Think}", "As We May Think"},
		{"{As {We} May Think}", "As We May Think"},
	} {
		if got := StripBraces(tc.in); got != tc.want {
			t.Errorf("StripBraces(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

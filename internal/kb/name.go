package kb

import "strings"

// nameSeparator is what BibTeX puts between the names of a name list.
const nameSeparator = " and "

// Name is one name from a BibTeX name list, split into the parts a formatter
// needs.
//
// BibTeX writes a personal name as "von Last, Jr, First" or as "First von Last",
// and a name it must not split — a corporation, say — in braces. A braced name
// comes back as a Literal, because splitting it would be wrong.
//
// There is one reader because there was nearly one per caller: duplicate
// detection, the CSL export and the built-in formatters all have to read the
// same name, and three readers meant three answers.
type Name struct {
	Family string
	Given  string
	Suffix string

	// Literal is a name that was not split: a braced name, or BibTeX's own
	// "others".
	Literal string
}

// Surname is the family name, which is what an in-text label shows. A literal
// name is its own surname.
func (n Name) Surname() string {
	if n.Family != "" {
		return n.Family
	}
	return n.Literal
}

// Plain is the name as a formatter writes it: "Family, Given" for a personal
// name, and the literal itself for one that was not split.
func (n Name) Plain() string {
	switch {
	case n.Literal != "":
		return n.Literal
	case n.Family == "":
		return n.Given
	case n.Given == "":
		return n.Family
	default:
		return n.Family + ", " + n.Given
	}
}

// Surname reads the family name from one name written in any BibTeX form. A
// name it cannot split comes back as written.
func Surname(s string) string {
	if n, ok := SplitName(s); ok {
		return n.Surname()
	}
	return strings.TrimSpace(s)
}

// SplitNameList splits a BibTeX name-list field into names.
//
// BibTeX separates names with " and ", which is why a name that contains it is
// written in braces. The split reads the braces, so "{Smith and Sons}" stays
// one name rather than becoming two.
func SplitNameList(value string) []Name {
	var out []Name
	for _, raw := range splitNameList(value) {
		if n, ok := SplitName(raw); ok {
			out = append(out, n)
		}
	}
	return out
}

// splitNameList splits a name list into the raw names it holds, at the
// separators that are not inside braces.
func splitNameList(value string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
		if depth != 0 || !strings.HasPrefix(value[i:], nameSeparator) {
			continue
		}
		if s := strings.TrimSpace(value[start:i]); s != "" {
			out = append(out, s)
		}
		i += len(nameSeparator) - 1
		start = i + 1
	}
	if s := strings.TrimSpace(value[start:]); s != "" {
		out = append(out, s)
	}
	return out
}

// SplitName reads one BibTeX name.
func SplitName(s string) (Name, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Name{}, false
	}
	if strings.EqualFold(s, "others") {
		return Name{Literal: "others"}, true
	}
	if inner, whole := braced(s); whole {
		return Name{Literal: strings.TrimSpace(StripBraces(inner))}, true
	}

	parts := strings.Split(s, ",")
	if len(parts) == 1 {
		family, given := splitFirstLast(parts[0])
		return Name{Family: StripBraces(family), Given: StripBraces(given)}, true
	}
	// "von Last, First" and "von Last, Jr, First". Anything longer keeps the
	// middle as the suffix rather than dropping it.
	return Name{
		Family: StripBraces(strings.TrimSpace(parts[0])),
		Suffix: StripBraces(strings.TrimSpace(strings.Join(parts[1:len(parts)-1], ", "))),
		Given:  StripBraces(strings.TrimSpace(parts[len(parts)-1])),
	}, true
}

// splitFirstLast splits "First von Last" into a family name and the rest.
//
// The family name is the last word plus the lowercase words before it, which is
// BibTeX's von part: "John von Neumann" is Neumann by family name and "von
// Neumann" by name.
func splitFirstLast(s string) (family, given string) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ""
	}
	split := len(fields) - 1
	for split > 0 && startsLower(fields[split-1]) {
		split--
	}
	return strings.Join(fields[split:], " "), strings.Join(fields[:split], " ")
}

func startsLower(s string) bool {
	return s != "" && s[0] >= 'a' && s[0] <= 'z'
}

// braced reports whether a name is wrapped in one pair of braces, and returns
// what is inside. It is how BibTeX protects a name that must not be split.
func braced(s string) (inner string, whole bool) {
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return "", false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[1:i], i == len(s)-1
			}
		}
	}
	return "", false
}

// braceRemover is built once: a replacer is a trie, and building one per field
// per generated page was pure waste.
var braceRemover = strings.NewReplacer("{", "", "}", "")

// StripBraces removes the braces a BibTeX value uses to protect capitalisation,
// because they are markup and not part of what a reader sees.
func StripBraces(s string) string {
	if !strings.ContainsAny(s, "{}") {
		return s
	}
	return braceRemover.Replace(s)
}

package kb

import (
	"strings"
	"unicode"
)

// Normalize reduces a name to the form links are compared in.
//
// A link matches a page when the two normalise to the same thing, so
// "Attention Is All You Need", "attention is all you need" and
// "attention-is-all-you-need" are one name rather than three.
//
// Letters and digits are kept, and "+" and "#" along with them, because
// dropping those two would collapse genuinely different titles such as "C" and
// "C++" into a name the format has no way to disambiguate. Every other run of
// characters becomes a single "-", and leading and trailing "-" are dropped.
func Normalize(name string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '#' {
			if gap && b.Len() > 0 {
				b.WriteByte('-')
			}
			gap = false
			b.WriteRune(r)
			continue
		}
		gap = true
	}
	return b.String()
}

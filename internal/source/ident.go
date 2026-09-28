package source

import (
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// Kind is the sort of identifier a string is.
type Kind int

const (
	KindUnknown Kind = iota
	KindDOI
	KindarXiv
	KindISBN
	KindURL
)

func (k Kind) String() string {
	switch k {
	case KindDOI:
		return "doi"
	case KindarXiv:
		return "arxiv"
	case KindISBN:
		return "isbn"
	case KindURL:
		return "url"
	}
	return "unknown"
}

// Detect classifies an identifier by its shape.
//
// Detection is by shape and not by a lookup, so a well-formed identifier that
// names nothing is still classified and fails later with the right message. A
// bare arxiv-style number is read as an arXiv identifier because nothing else
// in this format has that shape.
func Detect(id string) Kind {
	s := strings.TrimSpace(id)
	if s == "" {
		return KindUnknown
	}
	l := strings.ToLower(s)
	switch {
	case strings.HasPrefix(l, "doi:"), looksLikeDOI(s), isDOIURL(l):
		return KindDOI
	case strings.HasPrefix(l, "arxiv:"), looksLikearXiv(s), isArXivURL(l):
		return KindarXiv
	case strings.HasPrefix(l, "http://"), strings.HasPrefix(l, "https://"):
		return KindURL
	case looksLikeISBN(s):
		return KindISBN
	}
	return KindUnknown
}

// isDOIURL recognises the DOI resolver's own URL as a DOI, so that a DOI pasted
// from a browser address bar is read as what it is rather than fetched as a
// page.
func isDOIURL(l string) bool {
	for _, prefix := range []string{
		"https://doi.org/", "http://doi.org/",
		"https://dx.doi.org/", "http://dx.doi.org/",
	} {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// isArXivURL recognises an arXiv abstract or PDF URL as an identifier.
func isArXivURL(l string) bool {
	for _, prefix := range []string{
		"https://arxiv.org/abs/", "http://arxiv.org/abs/",
		"https://arxiv.org/pdf/", "http://arxiv.org/pdf/",
	} {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

func looksLikeDOI(s string) bool {
	if !strings.HasPrefix(s, "10.") {
		return false
	}
	i, start := 3, 3
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if n := i - start; n < 4 || n > 9 {
		return false
	}
	return i < len(s) && s[i] == '/' && i+1 < len(s)
}

func looksLikearXiv(s string) bool {
	t := s
	if len(t) >= 6 && strings.EqualFold(t[:6], "arxiv:") {
		t = t[6:]
	}
	return newStylearXiv(t) || oldStylearXiv(t)
}

func newStylearXiv(s string) bool {
	if len(s) < 9 {
		return false
	}
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if i != 4 || i >= len(s) || s[i] != '.' {
		return false
	}
	i++
	start := i
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	if n := i - start; n < 4 || n > 5 {
		return false
	}
	return versionSuffixOrEnd(s, i)
}

// oldStylearXiv matches "archive/YYMMNNN" and "archive.CATEGORY/YYMMNNN", the
// form arXiv used before April 2007 and still accepts.
func oldStylearXiv(s string) bool {
	slash := strings.IndexByte(s, '/')
	if slash <= 0 || slash == len(s)-1 {
		return false
	}
	if !validArchive(s[:slash]) {
		return false
	}
	id := s[slash+1:]
	i := 0
	for i < len(id) && isDigit(id[i]) {
		i++
	}
	if i != 7 {
		return false
	}
	return versionSuffixOrEnd(id, i)
}

func validArchive(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', isDigit(byte(r)), r == '-', r == '.', r == '_':
		default:
			return false
		}
	}
	return true
}

// versionSuffixOrEnd accepts "vN" at position i or the end of the string.
func versionSuffixOrEnd(s string, i int) bool {
	if i == len(s) {
		return true
	}
	if s[i] != 'v' && s[i] != 'V' {
		return false
	}
	i++
	if i == len(s) {
		return false
	}
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i == len(s)
}

func looksLikeISBN(s string) bool {
	t := kb.NormalizeISBN(s)
	if len(t) != 10 && len(t) != 13 {
		return false
	}
	for i := 0; i < len(t); i++ {
		if isDigit(t[i]) {
			continue
		}
		if i == 9 && len(t) == 10 && (t[i] == 'X' || t[i] == 'x') {
			continue
		}
		return false
	}
	return true
}

// validISBN checks the check digit, so a mistyped ISBN is refused as malformed
// rather than looked up and reported as missing.
func validISBN(s string) bool {
	t := kb.NormalizeISBN(s)
	switch len(t) {
	case 10:
		sum := 0
		for i := 0; i < 10; i++ {
			d := 0
			switch {
			case isDigit(t[i]):
				d = int(t[i] - '0')
			case i == 9 && (t[i] == 'X' || t[i] == 'x'):
				d = 10
			default:
				return false
			}
			sum += (10 - i) * d
		}
		return sum%11 == 0
	case 13:
		sum := 0
		for i := 0; i < 13; i++ {
			if !isDigit(t[i]) {
				return false
			}
			d := int(t[i] - '0')
			if i%2 == 1 {
				d *= 3
			}
			sum += d
		}
		return sum%10 == 0
	}
	return false
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// CiteKey builds a BibTeX citation key from the parts every entry has, so that
// a record resolved from a source without one still gets a stable, readable
// key. The parts are the first author's surname, the year, and the first
// significant title word: "cormen2009introduction".
func CiteKey(authors []string, date, title, fallback string) string {
	var b strings.Builder
	if len(authors) > 0 {
		b.WriteString(keyPart(kb.Surname(authors[0])))
	}
	b.WriteString(kb.YearOf(date))
	b.WriteString(firstWord(title))
	key := b.String()
	if key == "" {
		key = keyPart(fallback)
	}
	if key == "" {
		key = "source"
	}
	return key
}

// stopwords are the words a citation key skips when it reaches for a title's
// first significant word.
var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "on": true, "of": true,
	"in": true, "for": true, "and": true, "to": true, "at": true,
	"is": true, "as": true, "by": true,
}

func firstWord(title string) string {
	for _, word := range strings.Fields(title) {
		if w := keyPart(word); w != "" && !stopwords[w] {
			return w
		}
	}
	return ""
}

// keyPart keeps only the characters a citation key may carry, lowercased, so
// that a word with punctuation in it still yields a usable fragment.
func keyPart(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(foldASCII(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// foldASCII maps the accented letters that appear in names to the plain letters
// a key is written with. Anything else outside ASCII is dropped.
func foldASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
			continue
		}
		if repl, ok := latinFold[r]; ok {
			b.WriteString(repl)
		}
	}
	return b.String()
}

var latinFold = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ī': "i",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u",
	'ç': "c", 'ñ': "n", 'ý': "y", 'ÿ': "y",
	'æ': "ae", 'œ': "oe", 'ß': "ss",
	'À': "A", 'Á': "A", 'Â': "A", 'Ã': "A", 'Ä': "A", 'Å': "A",
	'È': "E", 'É': "E", 'Ê': "E", 'Ë': "E",
	'Ì': "I", 'Í': "I", 'Î': "I", 'Ï': "I",
	'Ò': "O", 'Ó': "O", 'Ô': "O", 'Õ': "O", 'Ö': "O", 'Ø': "O",
	'Ù': "U", 'Ú': "U", 'Û': "U", 'Ü': "U",
	'Ç': "C", 'Ñ': "N",
}

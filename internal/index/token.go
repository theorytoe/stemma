package index

import (
	"strings"
	"unicode"
)

// Tokenize splits text into the tokens both tiers compare.
//
// It lowercases, folds the common Latin accents to their base letters, and
// breaks on everything that is not a letter or a digit. Its output is the
// definition of a match: Tier 0 tokenizes a page with this function, and Tier 1
// stores the same output in the FTS table, so the two cannot disagree about
// what a page contains. That is why folding lives here and not in the FTS
// tokenizer, which would fold on one tier and not the other.
//
// A camel-case identifier stays one token, because nothing inside it is a
// separator. That is deliberate: `getUserName` is a word the author wrote, and
// splitting it would make a search for it fail.
func Tokenize(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(fold(unicode.ToLower(r)))
			continue
		}
		flush()
	}
	flush()
	return out
}

// Join renders tokens as the single space-separated string the FTS table
// stores. The separators are spaces because that is what the FTS tokenizer
// splits on, so it reproduces these tokens exactly rather than re-deciding
// where the words are.
func Join(tokens []string) string { return strings.Join(tokens, " ") }

// foldPairs is the accent-folding table, as consecutive (accented, base) pairs.
// It covers the precomposed Latin letters that occur in practice.
//
// It is deliberately a table and not Unicode decomposition: Go's standard
// library has no normaliser and the pinned dependency allowlist has no room for
// one, so the tool folds what it can and leaves every other script as written.
// Two pages that spell the same word with different accents still match when
// the accent is one of these; the rest match only when spelled the same way.
const foldPairs = "àaáaâaãaäaåaāaăaąa" +
	"çcćcĉcċcčc" +
	"ďd" +
	"èeéeêeëeēeĕeėeęeěe" +
	"ĝgğgģg" +
	"ĥh" +
	"ìiíiîiïiĩiīiĭiįi" +
	"ĵj" +
	"ķk" +
	"ĺlļlľl" +
	"ñnńnņnňn" +
	"òoóoôoõoöoōoŏoőo" +
	"ŕrřr" +
	"śsŝsşsšs" +
	"ţtťt" +
	"ùuúuûuüuũuūuŭuůuűuųu" +
	"ŵw" +
	"ýyÿyŷy" +
	"źzżzžz"

// folds maps a folded letter to its base. It is built from foldPairs, whose
// shape a test asserts.
var folds = func() map[rune]rune {
	rs := []rune(foldPairs)
	m := make(map[rune]rune, len(rs)/2)
	for i := 0; i+1 < len(rs); i += 2 {
		m[rs[i]] = rs[i+1]
	}
	return m
}()

// fold returns a rune's base letter, or the rune itself when it has none.
func fold(r rune) rune {
	if base, ok := folds[r]; ok {
		return base
	}
	return r
}

package index

import (
	"strings"
	"unicode"
)

// DefaultSnippetWidth is how much of a page a snippet shows when a caller does
// not choose. It is a target in characters, not a hard limit.
const DefaultSnippetWidth = 120

// Snippet returns one line of body around the first query term it finds, with
// every matched token wrapped in square brackets.
//
// The result is plain text, never HTML, because the commands that print it
// print to a terminal or a pipe. An empty result means none of the terms is in
// the body, which is a different answer from "the body is empty" and is left
// for the caller to say.
//
// The search is over tokens, not raw bytes, so a snippet marks the same words a
// query would match — including a word reached through the accent fold, whose
// original spelling is what gets shown.
func Snippet(body string, terms []string, width int) string {
	if width <= 0 {
		width = DefaultSnippetWidth
	}
	rs := []rune(body)
	spans := tokenSpans(rs)
	if len(spans) == 0 {
		return ""
	}

	want := map[string]bool{}
	for _, t := range terms {
		if t != "" {
			want[t] = true
		}
	}
	at := -1
	for i, sp := range spans {
		if want[sp.text] {
			at = i
			break
		}
	}
	if at < 0 {
		return ""
	}

	// Centre the window on the first match, then pull its edges in to token
	// boundaries so a word is never cut in half.
	center := (spans[at].start + spans[at].end) / 2
	start := center - width/3
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > len(rs) {
		end = len(rs)
	}
	start = snapRight(spans, start)
	end = snapLeft(spans, end)
	if end <= start {
		start, end = spans[at].start, spans[at].end
	}

	var out strings.Builder
	if start > 0 {
		out.WriteString("...")
	}
	pos := start
	for _, sp := range spans {
		if sp.end <= start {
			continue
		}
		if sp.start >= end {
			break
		}
		if sp.start > pos {
			out.WriteString(string(rs[pos:sp.start]))
		}
		if want[sp.text] {
			out.WriteString("[")
			out.WriteString(string(rs[sp.start:sp.end]))
			out.WriteString("]")
		} else {
			out.WriteString(string(rs[sp.start:sp.end]))
		}
		pos = sp.end
	}
	if pos < end {
		out.WriteString(string(rs[pos:end]))
	}
	if end < len(rs) {
		out.WriteString("...")
	}
	return strings.TrimSpace(out.String())
}

// tokenSpan is one token's place in a rune slice.
type tokenSpan struct {
	text  string
	start int
	end   int
}

// tokenSpans returns every token in rs with its rune offsets. It uses the same
// rule Tokenize does, so a match found here is a match a query would find.
func tokenSpans(rs []rune) []tokenSpan {
	var out []tokenSpan
	start := -1
	var b strings.Builder
	flush := func(end int) {
		if start >= 0 {
			out = append(out, tokenSpan{text: b.String(), start: start, end: end})
			b.Reset()
			start = -1
		}
	}
	for i, r := range rs {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
			b.WriteRune(fold(unicode.ToLower(r)))
			continue
		}
		flush(i)
	}
	flush(len(rs))
	return out
}

// snapRight moves an index forward off the start of a token.
func snapRight(spans []tokenSpan, at int) int {
	for _, sp := range spans {
		if sp.start < at && at < sp.end {
			return sp.end
		}
	}
	return at
}

// snapLeft moves an index back off the end of a token.
func snapLeft(spans []tokenSpan, at int) int {
	for _, sp := range spans {
		if sp.start < at && at < sp.end {
			return sp.start
		}
	}
	return at
}

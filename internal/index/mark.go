package index

import "strings"

// SnippetPart is one run of a snippet: literal text, or a query term the
// snippet marked.
//
// The snippet itself carries a mark as square brackets, which is a plain-text
// convention every consumer can read. A surface that can do better — the CLI
// with ANSI, the site with <mark> — needs the two kinds separated, and this is
// that separation, so neither surface has to decide for itself which brackets
// are marks.
type SnippetPart struct {
	// Text is the run as the page wrote it, without the marking brackets.
	Text string

	// Match is true when Text is one of the query's terms.
	Match bool
}

// SplitSnippet separates a snippet's marked terms from its literal text.
//
// A pair of brackets is a mark only when it holds exactly one token and that
// token is one of terms. Every other bracket is literal, which is what keeps a
// body's own "[[wikilink]]" from being read as a mark. The terms are the
// tokenizer's output, not the query as typed, so the same folding applies here
// as everywhere else.
func SplitSnippet(s string, terms []string) []SnippetPart {
	want := make(map[string]bool, len(terms))
	for _, t := range terms {
		if t != "" {
			want[t] = true
		}
	}

	var parts []SnippetPart
	add := func(text string, match bool) {
		if text == "" {
			return
		}
		if n := len(parts); n > 0 && parts[n-1].Match == match {
			parts[n-1].Text += text
			return
		}
		parts = append(parts, SnippetPart{Text: text, Match: match})
	}

	for len(s) > 0 {
		i := strings.IndexByte(s, '[')
		if i < 0 {
			add(s, false)
			break
		}
		j := strings.IndexByte(s[i+1:], ']')
		if j < 0 {
			add(s, false)
			break
		}
		inner := s[i+1 : i+1+j]
		toks := Tokenize(inner)
		if strings.ContainsAny(inner, "[]") || len(toks) != 1 || !want[toks[0]] {
			add(s[:i+1], false)
			s = s[i+1:]
			continue
		}
		add(s[:i], false)
		add(inner, true)
		s = s[i+1+j+1:]
	}
	return parts
}

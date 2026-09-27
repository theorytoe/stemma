package kb

import "strings"

// Link is one wikilink found in a page body.
type Link struct {
	// Target is the text between the brackets, as the author wrote it with
	// surrounding whitespace removed.
	Target string

	// Name is what Target normalises to. Resolution compares this.
	Name string

	// Line is the line in the file the link appears on.
	Line int
}

// Links returns every wikilink in the page body, in the order it appears.
//
// Link syntax applies to prose and not to code. Fenced blocks and inline code
// spans are skipped, which is what lets a page write `[[name]]` to describe the
// syntax without linking to a page called "name". The specification does
// exactly that, so a tool that read code as prose would make the project's own
// documentation unlinkable.
//
// A "[[", with no matching "]]" on the same line, is left as ordinary text
// rather than reported. A link cannot span lines, so the only thing it could
// mean is that something was typed by accident.
func (p *Page) Links() []Link {
	var out []Link
	fence := ""
	for i, l := range p.doc.body() {
		if fence == "" {
			if f, ok := fenceMarker(l.text); ok {
				fence = f
				continue
			}
		} else {
			if closesFence(l.text, fence) {
				fence = ""
			}
			continue
		}
		for _, target := range scanLine(l.text) {
			target = strings.TrimSpace(target)
			out = append(out, Link{
				Target: target,
				Name:   Normalize(target),
				Line:   p.doc.bodyStart + i + 1,
			})
		}
	}
	return out
}

// scanLine returns the targets of the wikilinks on one line of markdown,
// ignoring anything inside an inline code span.
func scanLine(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		if s[i] == '`' {
			n := 0
			for i+n < len(s) && s[i+n] == '`' {
				n++
			}
			if end, ok := closeBackticks(s, i+n, n); ok {
				i = end
				continue
			}
			i += n
			continue
		}
		if strings.HasPrefix(s[i:], "[[") {
			end := strings.Index(s[i+2:], "]]")
			if end < 0 {
				return out
			}
			out = append(out, s[i+2:i+2+end])
			i += 2 + end + 2
			continue
		}
		i++
	}
	return out
}

// closeBackticks finds the end of an inline code span opened by a run of n
// backticks: the next run of exactly n. A longer run does not close a shorter
// one, which is what lets a code span contain backticks.
func closeBackticks(s string, from, n int) (end int, ok bool) {
	for j := from; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		m := 0
		for j+m < len(s) && s[j+m] == '`' {
			m++
		}
		if m == n {
			return j + m, true
		}
		j += m
	}
	return 0, false
}

// fenceMarker returns the delimiter of a fenced code block when a line opens
// one. Markdown allows up to three leading spaces and a trailing info string.
func fenceMarker(text string) (string, bool) {
	trimmed := text
	for n := 0; n < 3 && strings.HasPrefix(trimmed, " "); n++ {
		trimmed = trimmed[1:]
	}
	if len(trimmed) < 3 {
		return "", false
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return "", false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return "", false
	}
	return trimmed[:n], true
}

// closesFence reports whether a line closes a block opened with fence. The
// closing run must be at least as long as the opening one and carry nothing but
// whitespace after it, so an info string inside the block does not close it.
func closesFence(text, fence string) bool {
	trimmed := strings.TrimLeft(text, " ")
	if len(text)-len(trimmed) > 3 {
		return false
	}
	if !strings.HasPrefix(trimmed, fence) {
		return false
	}
	return strings.TrimSpace(trimmed[len(fence):]) == ""
}

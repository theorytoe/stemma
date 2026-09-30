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
	for _, in := range p.Inlines() {
		if in.Link != nil {
			out = append(out, *in.Link)
		}
	}
	return out
}

// proseLine is one body line that is outside every fenced block, with the line
// number it has in the file and its index among the body's lines.
type proseLine struct {
	text  string
	line  int
	index int
}

// proseLines returns the body lines that are outside fenced code blocks.
//
// Fences are the one form of code decided a line at a time rather than a span
// at a time, so the decision is made once here and shared by everything that
// reads prose: links and citations both need it, and so will the renderer.
// Deciding it twice would be how lint and rendering come to disagree.
func (p *Page) proseLines() []proseLine {
	var out []proseLine
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
		out = append(out, proseLine{text: l.text, line: p.doc.bodyStart + i + 1, index: p.doc.bodyStart + i})
	}
	return out
}

// textSpan is a half-open byte range within one line.
type textSpan struct {
	start, end int
}

// wikilinkSpans returns the byte ranges of the "[[...]]" in one run of prose,
// each range covering both pairs of brackets.
func wikilinkSpans(s string) []textSpan {
	var out []textSpan
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], "[[") {
			end := strings.Index(s[i+2:], "]]")
			if end < 0 {
				return out
			}
			out = append(out, textSpan{i, i + 2 + end + 2})
			i += 2 + end + 2
			continue
		}
		i++
	}
	return out
}

// proseSpans returns the ranges of a line that are outside inline code spans.
func proseSpans(s string) []textSpan {
	var out []textSpan
	start := 0
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		n := 0
		for i+n < len(s) && s[i+n] == '`' {
			n++
		}
		end, ok := closeBackticks(s, i+n, n)
		if !ok {
			// An unmatched run is literal text, so what follows it is prose.
			i += n
			continue
		}
		out = append(out, textSpan{start, i})
		start, i = end, end
	}
	return append(out, textSpan{start, len(s)})
}

// RewriteLinks replaces the text inside the brackets of wikilinks in the body.
//
// rewrite is offered every link in prose, with the page's aliases and the
// page's links available through the ordinary accessors, and returns the text
// to put in place of the target, or false to leave that link alone. Links
// inside code are not links and are not offered.
//
// Everything else in the page comes back unchanged: no line is added, removed
// or reordered, so the frontmatter and the rest of the body are untouched byte
// for byte. The return value reports whether anything changed, so that a caller
// can leave a file it has no reason to write alone.
func (p *Page) RewriteLinks(rewrite func(Link) (string, bool)) bool {
	changed := false
	for _, pl := range p.proseLines() {
		text, rewritten := rewriteLinksInLine(pl.text, func(target string) (string, bool) {
			target = strings.TrimSpace(target)
			return rewrite(Link{Target: target, Name: Normalize(target), Line: pl.line})
		})
		if !rewritten {
			continue
		}
		p.doc.lines[pl.index].text = text
		changed = true
	}
	return changed
}

// rewriteLinksInLine replaces the targets of the wikilinks in one line,
// leaving the code spans and every other byte of the line as they were.
func rewriteLinksInLine(text string, rewrite func(string) (string, bool)) (string, bool) {
	if !strings.Contains(text, "[[") {
		return text, false
	}
	var b strings.Builder
	changed := false
	last := 0
	for _, run := range proseSpans(text) {
		b.WriteString(text[last:run.start])
		segment := text[run.start:run.end]
		at := 0
		for _, link := range wikilinkSpans(segment) {
			b.WriteString(segment[at:link.start])
			if replacement, ok := rewrite(segment[link.start+2 : link.end-2]); ok {
				b.WriteString("[[" + replacement + "]]")
				changed = true
			} else {
				b.WriteString(segment[link.start:link.end])
			}
			at = link.end
		}
		b.WriteString(segment[at:])
		last = run.end
	}
	b.WriteString(text[last:])
	if !changed {
		return text, false
	}
	return b.String(), true
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
//
// It may be longer. Markdown allows a closing fence to be any run of the same
// character that is at least as long as the opening one, and reading a longer
// run as still-inside-the-block would silently swallow every line after it.
func closesFence(text, fence string) bool {
	trimmed := strings.TrimLeft(text, " ")
	if len(text)-len(trimmed) > 3 {
		return false
	}
	if !strings.HasPrefix(trimmed, fence) {
		return false
	}
	rest := strings.TrimLeft(trimmed[len(fence):], string(fence[0]))
	return strings.TrimSpace(rest) == ""
}

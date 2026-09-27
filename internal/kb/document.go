package kb

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// delimiter is the line that opens and closes a frontmatter block.
const delimiter = "---"

// bom is the UTF-8 byte-order mark. A page that begins with one is refused
// rather than silently read as having no frontmatter.
const bom = "\ufeff"

// line is one line of a page, kept apart from its terminator so that the
// terminators come back unchanged.
type line struct {
	text string // never contains \r or \n
	eol  string // "\n", "\r\n", or "" on a final line with no terminator
}

func splitLines(b []byte) []line {
	var out []line
	for len(b) > 0 {
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			out = append(out, line{text: string(b)})
			break
		}
		text, eol := b[:i], "\n"
		if n := len(text); n > 0 && text[n-1] == '\r' {
			text, eol = text[:n-1], "\r\n"
		}
		out = append(out, line{text: string(text), eol: eol})
		b = b[i+1:]
	}
	return out
}

func joinLines(ls []line) []byte {
	var buf bytes.Buffer
	for _, l := range ls {
		buf.WriteString(l.text)
		buf.WriteString(l.eol)
	}
	return buf.Bytes()
}

// document is a page's bytes together with the line span that separates
// frontmatter from body.
//
// The bytes are the source of truth. Fields are read through yaml.v3, but they
// are written by splicing lines into the original block, so a field the tool
// never writes comes back exactly as it went in: same key order, same comments,
// same quoting style. Re-emitting the block through the YAML encoder could not
// promise that.
type document struct {
	lines []line

	// Frontmatter content occupies lines[fmStart:fmEnd]. fmStart is -1 when the
	// file has no frontmatter block at all.
	fmStart, fmEnd int

	// Body occupies lines[bodyStart:].
	bodyStart int
}

func newDocument(raw []byte) (document, error) {
	if !utf8.Valid(raw) {
		return document{}, fmt.Errorf("page is not valid UTF-8")
	}
	if bytes.HasPrefix(raw, []byte(bom)) {
		return document{}, fmt.Errorf("page begins with a byte-order mark")
	}
	d := document{lines: splitLines(raw)}
	if err := d.locate(); err != nil {
		return document{}, err
	}
	return d, nil
}

// locate recomputes the frontmatter and body spans from the lines. It runs
// after every edit, so the spans cannot drift from the bytes they describe.
func (d *document) locate() error {
	d.fmStart, d.fmEnd, d.bodyStart = -1, -1, 0
	if len(d.lines) == 0 || d.lines[0].text != delimiter {
		return nil
	}
	for i := 1; i < len(d.lines); i++ {
		if d.lines[i].text == delimiter {
			d.fmStart, d.fmEnd, d.bodyStart = 1, i, i+1
			return nil
		}
	}
	return fmt.Errorf("frontmatter opens on line 1 and is never closed")
}

func (d *document) hasFrontmatter() bool { return d.fmStart >= 0 }

func (d *document) frontmatter() []line {
	if !d.hasFrontmatter() {
		return nil
	}
	return d.lines[d.fmStart:d.fmEnd]
}

func (d *document) body() []line { return d.lines[d.bodyStart:] }

func (d *document) bytes() []byte { return joinLines(d.lines) }

// setField writes a top-level frontmatter key in place, creating a frontmatter
// block if the page has none.
//
// The value is encoded by yaml.v3, so quoting and indentation are always
// correct for the value being written. Everything outside the key's own line
// span is left alone.
func (d *document) setField(key string, value any) error {
	enc, err := encodeField(key, value)
	if err != nil {
		return err
	}
	block := append([]line(nil), d.frontmatter()...)
	if start, end, ok := fieldSpan(block, key); ok {
		block = splice(block, start, end, enc)
	} else {
		block = append(block, enc...)
	}
	return d.replaceFrontmatter(block)
}

// deleteField removes a top-level frontmatter key and everything that belongs
// to its value. Deleting a key that is absent does nothing.
func (d *document) deleteField(key string) error {
	block := d.frontmatter()
	start, end, ok := fieldSpan(block, key)
	if !ok {
		return nil
	}
	return d.replaceFrontmatter(splice(block, start, end, nil))
}

// replaceFrontmatter puts an edited block back, wrapping it in delimiters if
// the page had none.
func (d *document) replaceFrontmatter(block []line) error {
	block = terminate(block, d.preferredEOL())
	var next []line
	if d.hasFrontmatter() {
		next = make([]line, 0, len(d.lines)-(d.fmEnd-d.fmStart)+len(block))
		next = append(next, d.lines[:d.fmStart]...)
		next = append(next, block...)
		next = append(next, d.lines[d.fmEnd:]...)
	} else {
		eol := d.preferredEOL()
		next = make([]line, 0, len(d.lines)+len(block)+2)
		next = append(next, line{text: delimiter, eol: eol})
		next = append(next, block...)
		next = append(next, line{text: delimiter, eol: eol})
		next = append(next, d.lines...)
	}
	d.lines = next
	return d.locate()
}

// preferredEOL is the terminator the page already uses, so that lines the tool
// adds match the rest of the file instead of mixing conventions into it.
func (d *document) preferredEOL() string {
	if len(d.lines) > 0 && d.lines[0].eol != "" {
		return d.lines[0].eol
	}
	return "\n"
}

// splice replaces lines[start:end] with repl.
func splice(ls []line, start, end int, repl []line) []line {
	out := make([]line, 0, len(ls)-(end-start)+len(repl))
	out = append(out, ls[:start]...)
	out = append(out, repl...)
	out = append(out, ls[end:]...)
	return out
}

// terminate gives every line an end-of-line marker, keeping any marker already
// present. Spliced lines therefore never fuse with the line that follows them.
func terminate(ls []line, prefer string) []line {
	out := append([]line(nil), ls...)
	for i := range out {
		if out[i].eol == "" {
			out[i].eol = prefer
		}
	}
	return out
}

// encodeField renders one key and its value as YAML lines.
func encodeField(key string, value any) ([]line, error) {
	out, err := yaml.Marshal(map[string]any{key: value})
	if err != nil {
		return nil, fmt.Errorf("encoding %s: %w", key, err)
	}
	out = bytes.TrimSuffix(out, []byte("\n"))
	return splitLines(out), nil
}

// fieldSpan returns the line range a top-level key owns inside a frontmatter
// block.
//
// A key owns its own line and every following line that is indented or empty,
// which keeps a nested mapping, a block sequence and a block scalar together as
// one unit. A comment line belongs to the key that follows it, not to the key
// before it, so editing one field never disturbs another field's explanation of
// itself.
func fieldSpan(block []line, key string) (start, end int, ok bool) {
	for i := range block {
		k, isKey := topLevelKey(block[i].text)
		if !isKey || k != key {
			continue
		}
		end = i + 1
		for end < len(block) && continuesField(block[end].text) {
			end++
		}
		// Blank lines before the next key separate the two, so they belong to
		// the key that follows.
		for end > i+1 && strings.TrimSpace(block[end-1].text) == "" {
			end--
		}
		return i, end, true
	}
	return 0, 0, false
}

// continuesField reports whether a line can only be part of a value that began
// on an earlier line.
func continuesField(text string) bool {
	return text == "" || text[0] == ' ' || text[0] == '\t'
}

// topLevelKey reports whether a line opens an unindented mapping key, and
// returns that key.
//
// This works on bytes, not runes: every character that decides the answer is
// ASCII, and a lead byte of anything else is not one of them.
func topLevelKey(text string) (key string, ok bool) {
	if text == "" || strings.IndexByte(" \t#-", text[0]) >= 0 {
		return "", false
	}
	i := strings.IndexByte(text, ':')
	if i <= 0 {
		return "", false
	}
	k := strings.TrimSpace(text[:i])
	if k == "" {
		return "", false
	}
	if q := k[0]; q == '"' || q == '\'' {
		if len(k) < 2 || k[len(k)-1] != q {
			return "", false
		}
		return k[1 : len(k)-1], true
	}
	if strings.ContainsAny(k, " \t") {
		return "", false
	}
	return k, true
}

package kb

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Fields the tool owns on a bibliography entry.
//
// They carry provenance: when a source was retrieved and the hash of what was
// retrieved. They live on the entry because the bibliography is committed and
// is the only place a source's record can live (D38).
const (
	FieldRetrieved   = "stemma-retrieved"
	FieldContentHash = "stemma-content-hash"
	// FieldVendored is the hash of the text captured in sources/, and its presence
	// on an entry is what claims that a capture exists. It is a separate field from
	// FieldContentHash because a capture and the record it came from are different
	// artifacts, fetched at different times and able to drift apart.
	FieldVendored = "stemma-vendored-hash"
)

// BibFile is a parsed BibTeX file.
//
// As with a page, the bytes are the source of truth. Parsing finds each entry
// and keeps everything around it — the comments, the @string and @preamble
// blocks, the blank lines between entries — as untouched text, so a file nobody
// has edited comes back byte for byte. An entry nobody has edited comes back as
// it was written, too.
//
// An entry that is edited is re-rendered in one canonical shape rather than
// spliced field by field. Field order, unknown fields, unknown entry types and
// every value survive exactly; what a render may tidy is the alignment and
// whitespace inside that one entry. That is the trade the format makes
// everywhere: a file the tool has no reason to change is never rewritten, and a
// file it does rewrite keeps every fact it held.
type BibFile struct {
	name  string
	eol   string
	parts []bibPart

	entries []*BibEntry
	macros  map[string]string
}

// bibPart is one run of the file: an entry, or the literal text between
// entries.
type bibPart struct {
	raw   []byte
	entry *BibEntry
}

// BibEntry is one entry of a BibTeX file.
//
// The type and key are read-only and the fields are changed through Set and
// Delete, so that changing what an entry says always makes the entry re-render
// rather than leaving its bytes describing the entry it used to be.
type BibEntry struct {
	typ    string
	key    string
	fields []BibField
	raw    []byte
	dirty  bool
	eol    string
	owner  *BibFile
}

// BibField is one field of an entry. Name and Raw are both exactly as written;
// Raw includes whatever delimiters the value was written with, so {x}, "x" and
// x are three different raws for the same value.
type BibField struct {
	Name string
	Raw  string
}

// ParseBibFile reads a BibTeX file into entries, keeping the bytes it read so
// that a file nobody has edited can be written back unchanged.
//
// It refuses only what it cannot locate: bytes that are not UTF-8 and an entry
// whose closing delimiter is missing. Everything else — an unknown entry type,
// a field it has never heard of, a comment between entries, a macro defined
// after the entry that uses it — is kept, because a bibliography is committed
// and the tool must not destroy a record it does not understand.
func ParseBibFile(name string, raw []byte) (*BibFile, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("bibliography is not valid UTF-8")
	}
	f := &BibFile{name: name, eol: detectEOL(raw), macros: map[string]string{}}
	for i := 0; i < len(raw); {
		rel := bytes.IndexByte(raw[i:], '@')
		if rel < 0 {
			f.appendGap(raw[i:])
			break
		}
		at := i + rel

		typ, open, closing, body, ok := scanEntryHeader(raw, at)
		if !ok {
			// An "@" that opens no entry, such as one in a comment or in an
			// email address.
			f.appendGap(raw[i : at+1])
			i = at + 1
			continue
		}
		closeAt := scanEntryEnd(raw, body, open, closing)
		if closeAt < 0 {
			return nil, fmt.Errorf("the @%s entry beginning on line %d is never closed",
				strings.ToLower(typ), lineOf(raw, at))
		}
		end := closeAt + 1 // one past the closing delimiter
		if isNotAnEntry(strings.ToLower(typ)) {
			if strings.EqualFold(typ, "string") {
				f.readMacros(raw, body, closeAt)
			}
			f.appendGap(raw[i:end])
			i = end
			continue
		}

		key, fieldsFrom := scanKey(raw, body, closeAt)
		fields, err := parseFields(raw, fieldsFrom, closeAt, typ, false)
		if err != nil {
			return nil, err
		}
		if at > i {
			f.appendGap(raw[i:at])
		}
		e := &BibEntry{typ: typ, key: key, fields: fields, raw: raw[at:end], owner: f, eol: f.eol}
		f.parts = append(f.parts, bibPart{entry: e})
		f.entries = append(f.entries, e)
		i = end
	}
	return f, nil
}

// Name is the path the file was read from.
func (f *BibFile) Name() string { return f.name }

// Entries returns every entry in the file, in the order it appears.
func (f *BibFile) Entries() []*BibEntry { return append([]*BibEntry(nil), f.entries...) }

// Len is the number of entries.
func (f *BibFile) Len() int { return len(f.entries) }

// Entry returns the first entry with a key.
//
// A key defined twice has two entries and this returns the first; the duplicate
// is a finding rather than a reason to refuse the file.
func (f *BibFile) Entry(key string) (*BibEntry, bool) {
	for _, e := range f.entries {
		if e.key == key {
			return e, true
		}
	}
	return nil, false
}

// Has reports whether the file defines a key, matched exactly as written.
func (f *BibFile) Has(key string) bool {
	_, ok := f.Entry(key)
	return ok
}

// Bytes is the file as it would be written now. A file nobody has edited comes
// back exactly as it was read.
func (f *BibFile) Bytes() []byte {
	var buf bytes.Buffer
	for _, p := range f.parts {
		if p.entry != nil {
			buf.Write(p.entry.Bytes())
			continue
		}
		buf.Write(p.raw)
	}
	return buf.Bytes()
}

// AddEntry appends an entry to the file, on a line of its own.
func (f *BibFile) AddEntry(e *BibEntry) {
	e.owner = f
	if e.eol == "" {
		e.eol = f.eol
	}
	e.dirty = true

	if n := len(f.parts); n > 0 {
		switch last := f.parts[n-1]; {
		case last.entry != nil:
			f.parts = append(f.parts, bibPart{raw: []byte(f.eol)})
		case !bytes.HasSuffix(last.raw, []byte("\n")):
			f.parts[n-1].raw = withSuffix(last.raw, f.eol)
		}
	}
	f.parts = append(f.parts, bibPart{entry: e}, bibPart{raw: []byte(f.eol)})
	f.entries = append(f.entries, e)
}

// RemoveEntry removes the first entry with a key and reports whether it found
// one.
func (f *BibFile) RemoveEntry(key string) bool {
	for i, e := range f.entries {
		if e.key != key {
			continue
		}
		f.entries = append(f.entries[:i], f.entries[i+1:]...)
		for j, p := range f.parts {
			if p.entry != e {
				continue
			}
			f.parts = append(f.parts[:j], f.parts[j+1:]...)
			// The newline the entry was written on belongs to the entry, not to
			// the neighbour that follows it, so drop it when it is only
			// whitespace. A gap that carries a comment is left alone.
			if j < len(f.parts) {
				if g := f.parts[j]; g.entry == nil && isBlank(g.raw) {
					f.parts = append(f.parts[:j], f.parts[j+1:]...)
				}
			}
			break
		}
		return true
	}
	return false
}

// Macro returns a @string definition, decoded, and whether it is defined. A
// later definition of a name wins, which is what BibTeX does.
func (f *BibFile) Macro(name string) (string, bool) {
	raw, ok := f.macros[strings.ToLower(name)]
	if !ok {
		return "", false
	}
	return decodeBibValue(raw, f.macros), true
}

func (f *BibFile) appendGap(b []byte) {
	if len(b) == 0 {
		return
	}
	f.parts = append(f.parts, bibPart{raw: b})
}

// readMacros records the definitions in an @string block.
//
// Macros are best effort: they exist so that a value can be decoded, and a
// malformed @string must not make the whole bibliography unreadable.
func (f *BibFile) readMacros(src []byte, from, end int) {
	defs, _ := parseFields(src, from, end, "string", true)
	for _, d := range defs {
		if d.Name == "" {
			continue
		}
		f.macros[strings.ToLower(d.Name)] = d.Raw
	}
}

// Type is the entry type as written, without the "@", preserving its case.
func (e *BibEntry) Type() string { return e.typ }

// Key is the citation key as written.
func (e *BibEntry) Key() string { return e.key }

// SetType changes the entry type.
func (e *BibEntry) SetType(typ string) {
	e.typ = typ
	e.dirty = true
}

// SetKey changes the citation key.
func (e *BibEntry) SetKey(key string) {
	e.key = key
	e.dirty = true
}

// Fields returns every field in the order it appears, including fields the tool
// does not know.
func (e *BibEntry) Fields() []BibField { return append([]BibField(nil), e.fields...) }

// FieldNames returns the field names in order, as written.
func (e *BibEntry) FieldNames() []string {
	out := make([]string, 0, len(e.fields))
	for _, f := range e.fields {
		out = append(out, f.Name)
	}
	return out
}

// Raw returns a field's value exactly as written, delimiters included. Field
// names are matched without case, which is how BibTeX reads them.
func (e *BibEntry) Raw(name string) (string, bool) {
	for _, f := range e.fields {
		if strings.EqualFold(f.Name, name) {
			return f.Raw, true
		}
	}
	return "", false
}

// Value returns a field's value as BibTeX reads it: the outer delimiters gone,
// a concatenation joined, a macro expanded, runs of whitespace closed to one
// space. Braces inside a value are kept, because they are how a value protects
// its capitalisation.
func (e *BibEntry) Value(name string) (string, bool) {
	raw, ok := e.Raw(name)
	if !ok {
		return "", false
	}
	var macros map[string]string
	if e.owner != nil {
		macros = e.owner.macros
	}
	return decodeBibValue(raw, macros), true
}

// Set writes a field's value. The value is taken verbatim, delimiters and all,
// so the caller decides whether it is braced, quoted or a bare macro. An
// existing field keeps its name as written and only its value moves; a new
// field is appended.
func (e *BibEntry) Set(name, raw string) {
	for i := range e.fields {
		if strings.EqualFold(e.fields[i].Name, name) {
			e.fields[i].Raw = raw
			e.dirty = true
			return
		}
	}
	e.fields = append(e.fields, BibField{Name: name, Raw: raw})
	e.dirty = true
}

// Delete removes a field and reports whether it was there.
func (e *BibEntry) Delete(name string) bool {
	for i, f := range e.fields {
		if strings.EqualFold(f.Name, name) {
			e.fields = append(e.fields[:i], e.fields[i+1:]...)
			e.dirty = true
			return true
		}
	}
	return false
}

// Bytes is the entry as it would be written now.
func (e *BibEntry) Bytes() []byte {
	if !e.dirty && e.raw != nil {
		return append([]byte(nil), e.raw...)
	}
	return e.render()
}

// NewBibEntry returns an entry that is not yet in any file. It renders in the
// canonical shape until it is added to one, at which point it picks up that
// file's line endings.
func NewBibEntry(typ, key string) *BibEntry {
	return &BibEntry{typ: typ, key: key, dirty: true}
}

// render writes the entry in the canonical shape: one field per line, each
// "=" aligned to the longest name, so that a generated entry reads like a
// hand-written one.
func (e *BibEntry) render() []byte {
	eol := e.eol
	if eol == "" {
		eol = "\n"
	}
	var buf bytes.Buffer
	buf.WriteByte('@')
	buf.WriteString(e.typ)
	buf.WriteByte('{')
	buf.WriteString(e.key)
	if len(e.fields) == 0 {
		buf.WriteByte('}')
		return buf.Bytes()
	}
	buf.WriteByte(',')
	buf.WriteString(eol)

	width := 0
	for _, f := range e.fields {
		if len(f.Name) > width {
			width = len(f.Name)
		}
	}
	for _, f := range e.fields {
		buf.WriteString("  ")
		buf.WriteString(f.Name)
		for n := len(f.Name); n < width; n++ {
			buf.WriteByte(' ')
		}
		buf.WriteString(" = ")
		buf.WriteString(f.Raw)
		buf.WriteString(",")
		buf.WriteString(eol)
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// scanEntryHeader reads the "@type(" that opens an entry.
//
// It returns the type as written, the delimiter pair that encloses the entry,
// and the offset just past the opening delimiter. ok is false when the "@"
// opens no entry at all.
func scanEntryHeader(raw []byte, at int) (typ string, open, closing byte, body int, ok bool) {
	i := at + 1
	start := i
	for i < len(raw) && isASCIILetter(raw[i]) {
		i++
	}
	if i == start {
		return "", 0, 0, 0, false
	}
	typ = string(raw[start:i])
	for i < len(raw) && isBibSpace(raw[i]) {
		i++
	}
	if i >= len(raw) {
		return "", 0, 0, 0, false
	}
	switch raw[i] {
	case '{':
		return typ, '{', '}', i + 1, true
	case '(':
		return typ, '(', ')', i + 1, true
	}
	return "", 0, 0, 0, false
}

// scanEntryEnd returns the offset of the delimiter that closes an entry, or -1
// when it is never closed.
//
// Braces are counted wherever they appear, including inside a parenthesised
// entry, because a ")" inside a braced value belongs to the value and not to
// the structure around it. A double quote only opens a quoted value at the
// entry's own level: inside a braced value it is an ordinary character, which
// is what keeps an inch mark in a title from swallowing the rest of the file.
func scanEntryEnd(raw []byte, from int, open, closing byte) int {
	brace, quoted := 0, false
	for i := from; i < len(raw); i++ {
		c := raw[i]
		if c == '\\' {
			i++ // an escaped character is never structural
			continue
		}
		if brace == 0 && c == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if c == '{' {
			brace++
			continue
		}
		if c == '}' {
			if brace > 0 {
				brace--
				continue
			}
			if open == '{' {
				return i
			}
		}
		if brace == 0 && c == closing {
			return i
		}
	}
	return -1
}

// scanKey reads the citation key at the head of an entry body and returns the
// offset where the fields begin.
func scanKey(raw []byte, from, end int) (key string, fields int) {
	i := from
	for i < end && isBibSpace(raw[i]) {
		i++
	}
	start := i
	for i < end && !isBibSpace(raw[i]) && raw[i] != ',' {
		i++
	}
	key = string(raw[start:i])
	for i < end && isBibSpace(raw[i]) {
		i++
	}
	if i < end && raw[i] == ',' {
		i++
	}
	return key, i
}

// parseFields reads the "name = value" pairs in src[from:end].
//
// A value ends at the next top-level comma, so braces and quotes shield the
// commas they contain. In lenient mode a field that is not a pair simply ends
// the list, which is how an @string block is read without being able to make
// the file unreadable over a macro nobody can decode anyway.
func parseFields(src []byte, from, end int, typ string, lenient bool) ([]BibField, error) {
	var out []BibField
	for i := from; i < end; {
		for i < end && (isBibSpace(src[i]) || src[i] == ',') {
			i++
		}
		if i >= end {
			break
		}

		nameStart := i
		for i < end && !isBibSpace(src[i]) && src[i] != '=' && src[i] != ',' {
			i++
		}
		name := string(src[nameStart:i])
		for i < end && isBibSpace(src[i]) {
			i++
		}
		if name == "" || i >= end || src[i] != '=' {
			if lenient {
				break
			}
			return nil, fmt.Errorf(
				"the @%s entry beginning on line %d has a field that is not a name = value pair",
				strings.ToLower(typ), lineOf(src, nameStart))
		}
		i++ // the "="

		for i < end && isBibSpace(src[i]) {
			i++
		}
		valueStart := i
		brace, quoted := 0, false
		for i < end {
			c := src[i]
			if c == '\\' {
				i += 2
				continue
			}
			if brace == 0 && c == '"' {
				quoted = !quoted
				i++
				continue
			}
			if quoted {
				i++
				continue
			}
			if c == '{' {
				brace++
				i++
				continue
			}
			if c == '}' {
				if brace > 0 {
					brace--
				}
				i++
				continue
			}
			if c == ',' && brace == 0 {
				break
			}
			i++
		}
		out = append(out, BibField{Name: name, Raw: strings.TrimSpace(string(src[valueStart:i]))})
	}
	return out, nil
}

// decodeBibValue reads a value the way BibTeX does: it joins a "#"
// concatenation, strips the outer delimiters of each piece and expands a macro.
//
// A name that is not a defined macro is returned as written rather than
// dropped. A value is evidence, and a macro this file does not define is still
// something the author wrote.
func decodeBibValue(raw string, macros map[string]string) string {
	return decodeBibValueSeen(raw, macros, map[string]bool{})
}

func decodeBibValueSeen(raw string, macros map[string]string, seen map[string]bool) string {
	var buf strings.Builder
	for _, part := range splitConcat(raw) {
		part = strings.TrimSpace(part)
		switch {
		case len(part) >= 2 && part[0] == '{' && part[len(part)-1] == '}':
			buf.WriteString(collapseSpaces(part[1 : len(part)-1]))
		case len(part) >= 2 && part[0] == '"' && part[len(part)-1] == '"':
			buf.WriteString(collapseSpaces(part[1 : len(part)-1]))
		default:
			name := strings.ToLower(part)
			if v, ok := macros[name]; ok && !seen[name] {
				seen[name] = true
				buf.WriteString(decodeBibValueSeen(v, macros, seen))
				delete(seen, name)
				continue
			}
			buf.WriteString(part)
		}
	}
	return buf.String()
}

// splitConcat splits a value on the "#" that joins pieces, ignoring the ones
// inside a braced or quoted piece.
func splitConcat(raw string) []string {
	var out []string
	brace, quoted, start := 0, false, 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\\' {
			i++
			continue
		}
		if brace == 0 && c == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if c == '{' {
			brace++
			continue
		}
		if c == '}' {
			if brace > 0 {
				brace--
			}
			continue
		}
		if c == '#' && brace == 0 {
			out = append(out, raw[start:i])
			start = i + 1
		}
	}
	return append(out, raw[start:])
}

// collapseSpaces closes every run of whitespace to one space, keeping a leading
// or trailing space so that the space a "#" concatenation relies on survives.
func collapseSpaces(s string) string {
	var buf strings.Builder
	space := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			space = true
			continue
		}
		if space {
			buf.WriteByte(' ')
			space = false
		}
		buf.WriteRune(r)
	}
	if space {
		buf.WriteByte(' ')
	}
	return buf.String()
}

// detectEOL is the terminator the file already uses, so that an entry the tool
// adds matches the rest of the file.
func detectEOL(raw []byte) string {
	if bytes.Contains(raw, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// withSuffix copies b before appending, because b may be a slice of a larger
// buffer that other parts of the file still point into.
func withSuffix(b []byte, s string) []byte {
	out := make([]byte, 0, len(b)+len(s))
	out = append(out, b...)
	out = append(out, s...)
	return out
}

// isNotAnEntry reports whether an entry type is one of the three that carry no
// citation key. @string defines a macro, @preamble injects LaTeX and @comment
// is ignored outright; the name after each is not a key anything can cite.
func isNotAnEntry(entryType string) bool {
	switch entryType {
	case "string", "preamble", "comment":
		return true
	}
	return false
}

func isASCIILetter(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z')
}

func isBibSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// isBlank reports whether a run of text is nothing but whitespace.
func isBlank(b []byte) bool {
	for _, c := range b {
		if !isBibSpace(c) {
			return false
		}
	}
	return true
}

func lineOf(raw []byte, offset int) int {
	return bytes.Count(raw[:offset], []byte("\n")) + 1
}

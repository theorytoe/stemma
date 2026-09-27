package kb

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Field names the format gives meaning to. Anything else in a page's
// frontmatter is an unknown field: preserved, never interpreted, never a
// reason to refuse a write.
const (
	FieldTitle         = "title"
	FieldType          = "type"
	FieldStatus        = "status"
	FieldAliases       = "aliases"
	FieldArchiveReason = "archive_reason"
	FieldKey           = "key"
)

// Status values.
const (
	StatusActive   = "active"
	StatusArchived = "archived"
)

// Page is one document: an optional frontmatter block followed by a markdown
// body.
//
// A Page keeps the bytes it was parsed from. Reading goes through the parsed
// YAML, but writing splices the original block, so the parts of a page the tool
// does not own survive every edit unchanged. A page that is parsed and written
// back without touching a field is byte-identical to what came in.
type Page struct {
	doc document

	// fields holds the parsed top-level frontmatter values, and keys records
	// their order. Both are empty when the page has no frontmatter.
	fields map[string]*yaml.Node
	keys   []string
}

// ParsePage reads a page from its bytes.
//
// It fails only on a file the tool cannot read as a page at all: bytes that are
// not valid UTF-8, a frontmatter block that is never closed, frontmatter that
// is not valid YAML, and frontmatter that is not a mapping. Those are the cases
// where the tool cannot tell what it would be rewriting, so it refuses. Every
// other problem is reported by Validate, so that a page with something wrong in
// it can still be read, shown and repaired.
func ParsePage(raw []byte) (*Page, error) {
	doc, err := newDocument(raw)
	if err != nil {
		return nil, err
	}
	p := &Page{doc: doc}
	if err := p.load(); err != nil {
		return nil, err
	}
	return p, nil
}

// load parses the frontmatter block into fields, replacing whatever was there.
func (p *Page) load() error {
	p.fields, p.keys = nil, nil

	src := joinLines(p.doc.frontmatter())
	if len(bytes.TrimSpace(src)) == 0 {
		return nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return fmt.Errorf("frontmatter: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	m := root.Content[0]
	if m.Kind != yaml.MappingNode {
		return fmt.Errorf("frontmatter: top level must be a mapping, found %s", kindName(m.Kind))
	}

	p.fields = make(map[string]*yaml.Node, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		key := m.Content[i].Value
		if _, seen := p.fields[key]; seen {
			return fmt.Errorf("frontmatter: %q is defined twice", key)
		}
		p.fields[key] = m.Content[i+1]
		p.keys = append(p.keys, key)
	}
	return nil
}

// Bytes returns the page exactly as it now stands, frontmatter and body.
func (p *Page) Bytes() []byte { return p.doc.bytes() }

// Body returns the markdown after the frontmatter block.
func (p *Page) Body() []byte { return joinLines(p.doc.body()) }

// Frontmatter returns the frontmatter between its delimiters, without them.
func (p *Page) Frontmatter() []byte { return joinLines(p.doc.frontmatter()) }

// HasFrontmatter reports whether the page has a frontmatter block at all.
func (p *Page) HasFrontmatter() bool { return p.doc.hasFrontmatter() }

// Keys returns the top-level frontmatter keys in the order they appear.
func (p *Page) Keys() []string { return append([]string(nil), p.keys...) }

// Field returns the raw parsed value of a top-level key. Callers that need to
// know whether a value is shaped as the format expects use this; the typed
// accessors below are conveniences that give up when it is not.
func (p *Page) Field(name string) (*yaml.Node, bool) {
	n, ok := p.fields[name]
	return n, ok
}

// String returns a field's value when the field is present and is a scalar, and
// "" otherwise.
func (p *Page) String(name string) string {
	n, ok := p.fields[name]
	if !ok || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// Title is the page's identity: the name wikilinks resolve against.
func (p *Page) Title() string { return p.String(FieldTitle) }

// Type is the page's kind, one of the vocabulary.
func (p *Page) Type() string { return p.String(FieldType) }

// Status is effective, not literal: a page with no status is active.
func (p *Page) Status() string {
	if s := p.String(FieldStatus); s != "" {
		return s
	}
	return StatusActive
}

// Aliases returns the names the page also answers to. It returns nothing unless
// the field is a sequence of scalars.
func (p *Page) Aliases() []string {
	n, ok := p.fields[FieldAliases]
	if !ok || n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(n.Content))
	for _, item := range n.Content {
		if item.Kind != yaml.ScalarNode {
			return nil
		}
		out = append(out, item.Value)
	}
	return out
}

// ArchiveReason returns the reason recorded when the page was archived.
func (p *Page) ArchiveReason() string { return p.String(FieldArchiveReason) }

// Key returns the citation key of a virtual source page.
func (p *Page) Key() string { return p.String(FieldKey) }

// Set writes a top-level frontmatter field. Every other byte of the page,
// including unknown fields, key order, comments and the whole body, is left
// untouched. Setting a field the page does not have appends it.
//
// A comment written inside the field's own lines goes with it, because those
// lines are what is being replaced.
func (p *Page) Set(name string, value any) error {
	if err := p.doc.setField(name, value); err != nil {
		return err
	}
	return p.load()
}

// Delete removes a top-level frontmatter field and its value.
func (p *Page) Delete(name string) error {
	if err := p.doc.deleteField(name); err != nil {
		return err
	}
	return p.load()
}

// line converts a node's line within the frontmatter block to a line in the
// file. It returns 0 when the line is not known, as for a field that is absent.
func (p *Page) line(n *yaml.Node) int {
	if n == nil || p.doc.fmStart < 0 {
		return 0
	}
	return p.doc.fmStart + n.Line
}

func kindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "a document"
	case yaml.SequenceNode:
		return "a sequence"
	case yaml.MappingNode:
		return "a mapping"
	case yaml.ScalarNode:
		return "a scalar"
	case yaml.AliasNode:
		return "an alias"
	}
	return "an unknown node"
}

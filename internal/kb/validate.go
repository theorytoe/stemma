package kb

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Severity is how much a finding matters.
type Severity string

const (
	Warning Severity = "warning"
	Error   Severity = "error"
)

// Mode decides whether the format's soft rules are complaints or failures.
type Mode int

const (
	// Lenient is the default: a soft rule produces a warning.
	Lenient Mode = iota
	// Strict escalates a soft rule to an error, for CI.
	Strict
)

// Finding codes. They are stable strings, because they travel in --json output.
const (
	CodeMissingField   = "missing-field"
	CodeMalformedField = "malformed-field"
	CodeUnknownType    = "unknown-type"
	CodeInvalidStatus  = "invalid-status"
	CodeReservedType   = "reserved-type"
	CodeReservedField  = "reserved-field"
)

// Finding is one thing a page gets wrong.
//
// Line is the line in the file, or 0 when the field is absent and so has no
// line. A finding that depends on other pages — an unresolved link, a name
// collision, an orphan — cannot be decided by looking at one page, and is
// raised by lint rather than here.
type Finding struct {
	Severity Severity
	Code     string
	Field    string
	Line     int
	Message  string
}

// The built-in types.
const (
	TypeTopic   = "topic"
	TypeConcept = "concept"
	TypeNote    = "note"
)

// DefaultType is what `new` applies when an author does not choose.
const DefaultType = TypeTopic

// The reserved types. They are owned by the tool: valid to find in a file,
// because the tool writes them, but never assigned on an author's behalf.
const (
	TypeSource = "source"
	TypeIndex  = "index"
)

// Vocabulary is the set of types a KB accepts: the built-in three, plus the
// reserved two, plus whatever a manifest adds.
type Vocabulary struct {
	named map[string]bool
}

// DefaultVocabulary is the vocabulary of a KB with no manifest.
func DefaultVocabulary() Vocabulary {
	return Vocabulary{named: map[string]bool{
		TypeTopic:   true,
		TypeConcept: true,
		TypeNote:    true,
		TypeSource:  true,
		TypeIndex:   true,
	}}
}

// Extend returns a vocabulary that also accepts the named types, which is how a
// manifest widens the set. Names that are already present change nothing, and
// widening never makes a reserved type assignable.
func (v Vocabulary) Extend(names ...string) Vocabulary {
	out := Vocabulary{named: make(map[string]bool, len(v.named)+len(names))}
	for name := range v.named {
		out.named[name] = true
	}
	for _, name := range names {
		if name != "" {
			out.named[name] = true
		}
	}
	return out
}

// Has reports whether a type is one the KB knows.
func (v Vocabulary) Has(name string) bool { return v.named[name] }

// Assignable reports whether an author may choose a type.
func (v Vocabulary) Assignable(name string) bool {
	return v.named[name] && !IsReservedType(name)
}

// Names returns every type the vocabulary accepts, sorted.
func (v Vocabulary) Names() []string {
	out := make([]string, 0, len(v.named))
	for name := range v.named {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// IsReservedType reports whether a type is owned by the tool.
func IsReservedType(name string) bool {
	return name == TypeSource || name == TypeIndex
}

// Validate reports everything wrong with this page on its own terms: the
// fields the format requires, the values it allows, and the fields reserved for
// the tool.
//
// Nothing here needs another page, so nothing here is a hard error. Under
// Strict every finding becomes an error; under Lenient they are warnings. The
// one failure the format never softens — a link that does not resolve to
// exactly one page — belongs to lint.
func (p *Page) Validate(v Vocabulary, mode Mode) []Finding {
	soft := Warning
	if mode == Strict {
		soft = Error
	}
	var out []Finding
	add := func(sev Severity, code, field, msg string) {
		out = append(out, Finding{
			Severity: sev,
			Code:     code,
			Field:    field,
			Line:     p.line(p.fields[field]),
			Message:  msg,
		})
	}

	// title is the page's identity, so it is required and it must be a string.
	title, ok := p.fields[FieldTitle]
	switch {
	case !ok:
		add(soft, CodeMissingField, FieldTitle, "no title: a page is identified by its title")
	case title.Kind != yaml.ScalarNode:
		add(soft, CodeMalformedField, FieldTitle,
			"title must be a string, found "+kindName(title.Kind))
	case strings.TrimSpace(title.Value) == "":
		add(soft, CodeMissingField, FieldTitle, "title is empty")
	}

	// type is required, and must be one the KB knows.
	typ, ok := p.fields[FieldType]
	switch {
	case !ok:
		add(soft, CodeMissingField, FieldType, "no type")
	case typ.Kind != yaml.ScalarNode:
		add(soft, CodeMalformedField, FieldType,
			"type must be a string, found "+kindName(typ.Kind))
	case strings.TrimSpace(typ.Value) == "":
		add(soft, CodeMissingField, FieldType, "type is empty")
	case !v.Has(typ.Value):
		add(soft, CodeUnknownType, FieldType,
			"unknown type "+quote(typ.Value)+"; a manifest can add it to the vocabulary")
	case typ.Value == TypeSource:
		add(soft, CodeReservedType, FieldType,
			"source pages are generated at export and build time and are never committed")
	}

	// status is optional and defaults to active.
	if node, ok := p.fields[FieldStatus]; ok {
		switch {
		case node.Kind != yaml.ScalarNode:
			add(soft, CodeMalformedField, FieldStatus,
				"status must be a string, found "+kindName(node.Kind))
		case node.Value != StatusActive && node.Value != StatusArchived:
			add(soft, CodeInvalidStatus, FieldStatus,
				"status is "+quote(node.Value)+"; it is "+StatusActive+" or "+StatusArchived)
		}
	}

	// aliases is optional and, when present, is a list of names.
	if node, ok := p.fields[FieldAliases]; ok {
		if node.Kind != yaml.SequenceNode {
			add(soft, CodeMalformedField, FieldAliases,
				"aliases must be a list, found "+kindName(node.Kind))
		} else {
			for _, item := range node.Content {
				if item.Kind != yaml.ScalarNode {
					add(soft, CodeMalformedField, FieldAliases,
						"aliases must be a list of strings, found "+kindName(item.Kind)+" in it")
					break
				}
			}
		}
	}

	// archive_reason is written by `archive` and is optional everywhere else.
	if node, ok := p.fields[FieldArchiveReason]; ok && node.Kind != yaml.ScalarNode {
		add(soft, CodeMalformedField, FieldArchiveReason,
			"archive_reason must be a string, found "+kindName(node.Kind))
	}

	// key belongs to virtual source pages and nowhere else.
	if node, ok := p.fields[FieldKey]; ok {
		if node.Kind != yaml.ScalarNode {
			add(soft, CodeMalformedField, FieldKey,
				"key must be a string, found "+kindName(node.Kind))
		} else if p.Type() != TypeSource {
			add(soft, CodeReservedField, FieldKey,
				"key belongs to a source page; this page has type "+quote(p.Type()))
		}
	}

	return out
}

func quote(s string) string {
	if s == "" {
		return "nothing"
	}
	return `"` + s + `"`
}

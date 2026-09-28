package kb

import (
	"testing"
)

func codes(fs []Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func TestCleanPageProducesNoFindings(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: concept\nstatus: archived\naliases:\n  - a\n---\n")
	if got := p.Validate(DefaultVocabulary(), Lenient); len(got) != 0 {
		t.Errorf("findings = %v, want none", codes(got))
	}
}

func TestValidatePerPageRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "no frontmatter at all",
			raw:  "body only\n",
			want: []string{CodeMissingField, CodeMissingField},
		},
		{
			name: "empty title is a missing title",
			raw:  "---\ntitle: \"\"\ntype: concept\n---\n",
			want: []string{CodeMissingField},
		},
		{
			name: "empty type is a missing type",
			raw:  "---\ntitle: A\ntype: \"\"\n---\n",
			want: []string{CodeMissingField},
		},
		{
			name: "a title that names nothing",
			raw:  "---\ntitle: \"!!!\"\ntype: concept\n---\n",
			want: []string{CodeMissingField},
		},
		{
			name: "a title of only punctuation",
			raw:  "---\ntitle: \"...\"\ntype: concept\n---\n",
			want: []string{CodeMissingField},
		},
		{
			name: "title is not a string",
			raw:  "---\ntitle: [a, b]\ntype: concept\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "type is not a string",
			raw:  "---\ntitle: A\ntype: [concept]\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "unknown type",
			raw:  "---\ntitle: A\ntype: person\n---\n",
			want: []string{CodeUnknownType},
		},
		{
			name: "invalid status",
			raw:  "---\ntitle: A\ntype: concept\nstatus: draft\n---\n",
			want: []string{CodeInvalidStatus},
		},
		{
			name: "status is not a string",
			raw:  "---\ntitle: A\ntype: concept\nstatus: [active]\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "aliases is not a list",
			raw:  "---\ntitle: A\ntype: concept\naliases: a\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "aliases holds something other than strings",
			raw:  "---\ntitle: A\ntype: concept\naliases:\n  - a\n  - {b: c}\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "tags is not a list",
			raw:  "---\ntitle: A\ntype: concept\ntags: a\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "tags holds something other than strings",
			raw:  "---\ntitle: A\ntype: concept\ntags:\n  - a\n  - {b: c}\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "a tag that normalises to nothing",
			raw:  "---\ntitle: A\ntype: concept\ntags: [\"!!!\"]\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "free-form tags are never unknown",
			raw:  "---\ntitle: A\ntype: concept\ntags: [anything, at, all]\n---\n",
			want: nil,
		},
		{
			name: "archive_reason is not a string",
			raw:  "---\ntitle: A\ntype: concept\narchive_reason: [x]\n---\n",
			want: []string{CodeMalformedField},
		},
		{
			name: "a committed source page",
			raw:  "---\ntitle: A\ntype: source\nkey: vaswani2017\n---\n",
			want: []string{CodeReservedType},
		},
		{
			name: "key on a page that is not a source",
			raw:  "---\ntitle: A\ntype: concept\nkey: vaswani2017\n---\n",
			want: []string{CodeReservedField},
		},
		{
			name: "an index page is legitimate on disk",
			raw:  "---\ntitle: A\ntype: index\n---\n",
			want: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := mustParse(t, tc.raw)
			got := codes(p.Validate(DefaultVocabulary(), Lenient))
			if len(got) != len(tc.want) {
				t.Fatalf("findings = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("findings = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// Lenient warns and strict fails, on the same page and the same finding.
func TestStrictEscalatesEveryFinding(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: person\nstatus: draft\n---\n")

	lenient := p.Validate(DefaultVocabulary(), Lenient)
	if len(lenient) != 2 {
		t.Fatalf("lenient findings = %v, want 2", codes(lenient))
	}
	for _, f := range lenient {
		if f.Severity != Warning {
			t.Errorf("lenient severity = %q, want %q", f.Severity, Warning)
		}
	}

	strict := p.Validate(DefaultVocabulary(), Strict)
	if len(strict) != 2 {
		t.Fatalf("strict findings = %v, want 2", codes(strict))
	}
	for _, f := range strict {
		if f.Severity != Error {
			t.Errorf("strict severity = %q, want %q", f.Severity, Error)
		}
	}
}

// A finding points at the line the reader would have to open.
func TestFindingsCarryTheirLine(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: person\n---\n")
	got := p.Validate(DefaultVocabulary(), Lenient)
	if len(got) != 1 {
		t.Fatalf("findings = %v", codes(got))
	}
	if got[0].Line != 3 {
		t.Errorf("line = %d, want 3", got[0].Line)
	}
	if got[0].Field != FieldType {
		t.Errorf("field = %q, want %q", got[0].Field, FieldType)
	}
}

func TestVocabularyExtension(t *testing.T) {
	v := DefaultVocabulary().Extend("person", "project")

	if !v.Has("person") || !v.Has("project") {
		t.Error("Extend did not add the named types")
	}
	if !v.Has(TypeTopic) {
		t.Error("Extend dropped a built-in type")
	}
	if !v.Assignable("person") {
		t.Error("an added type should be assignable")
	}
	for _, reserved := range []string{TypeSource, TypeIndex} {
		if !v.Has(reserved) {
			t.Errorf("%s should be a known type", reserved)
		}
		if v.Assignable(reserved) {
			t.Errorf("%s must never be assignable", reserved)
		}
	}

	p := mustParse(t, "---\ntitle: A\ntype: person\n---\n")
	if got := p.Validate(v, Lenient); len(got) != 0 {
		t.Errorf("a configured type was reported unknown: %v", codes(got))
	}
}

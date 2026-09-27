package kb

import (
	"strings"
	"testing"
)

// gnarly is a page doing everything it can to be broken by a careless writer:
// unknown fields, nested structures, block scalars, quoting that has to be
// preserved rather than normalised, and comments in the middle of the block.
const gnarly = `---
title: Attention Is All You Need
type: concept
# This comment belongs to the next field, not the one before it.
aliases:
  - transformer paper
  - vaswani2017
status: active
review:
  state: draft
  reviewers:
    - ashish
    - noam
  history:
    - when: 2017-06-12
      what: "first arXiv posting"
numbers: [1, 2.5, -3, 1_000]
unusual:
  empty: ""
  nil: ~
  truthy: yes
  spaced:    "much   space"
  colour: '#hashtag'
  clock: 12:30
  block: |
    line one
    line two
  folded: >
    folded
    text
"quoted key": keeps working
---

# Body heading

The body is not the tool's business. It has [[wikilinks]], a citation
[@vaswani2017], and a line that looks like a delimiter:

---
`

func mustParse(t *testing.T, raw string) *Page {
	t.Helper()
	p, err := ParsePage([]byte(raw))
	if err != nil {
		t.Fatalf("ParsePage: %v", err)
	}
	return p
}

// A page the tool has no reason to change comes back byte-identical, which is
// the whole point of keeping the bytes instead of re-encoding the block.
func TestParseThenWriteIsByteIdentical(t *testing.T) {
	p := mustParse(t, gnarly)
	if got := string(p.Bytes()); got != gnarly {
		t.Errorf("round trip changed the page:\n--- got ---\n%s\n--- want ---\n%s", got, gnarly)
	}
}

// An edit must move its own field and nothing else: not the key order, not the
// comments, not the unknown fields, not the quoting style, not the body.
func TestSetRewritesOnlyItsOwnField(t *testing.T) {
	p := mustParse(t, gnarly)
	if err := p.Set(FieldStatus, StatusArchived); err != nil {
		t.Fatalf("Set: %v", err)
	}

	want := strings.Replace(gnarly, "status: active", "status: archived", 1)
	if got := string(p.Bytes()); got != want {
		t.Errorf("edit disturbed more than its own field:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// A comment sits above the field it explains. Editing the field before it must
// not swallow the comment.
func TestCommentSurvivesEditingThePrecedingField(t *testing.T) {
	in := "---\ntitle: A\ntype: concept\nunknown: 1\n# why the aliases look like this\naliases:\n  - x\n---\nbody\n"
	p := mustParse(t, in)
	if err := p.Set(FieldType, TypeNote); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := string(p.Bytes()); !strings.Contains(got, "# why the aliases look like this") {
		t.Errorf("the comment was lost:\n%s", got)
	}
	if got := string(p.Bytes()); !strings.Contains(got, "type: note") {
		t.Errorf("type was not written:\n%s", got)
	}
}

func TestSetReplacesAMultiLineValue(t *testing.T) {
	p := mustParse(t, gnarly)
	if err := p.Set(FieldAliases, []string{"maybe"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if got := p.Aliases(); len(got) != 1 || got[0] != "maybe" {
		t.Errorf("Aliases = %q, want [maybe]", got)
	}

	got := string(p.Bytes())
	if strings.Contains(got, "transformer paper") {
		t.Errorf("the old value survived:\n%s", got)
	}
	if !strings.Contains(got, "status: active") || !strings.Contains(got, "review:") {
		t.Errorf("neighbouring fields were disturbed:\n%s", got)
	}
	if !strings.Contains(got, "[@vaswani2017]") || !strings.Contains(got, "# Body heading") {
		t.Errorf("the body was disturbed:\n%s", got)
	}
}

// Writing a field the page does not have appends it, and still leaves every
// other byte alone.
func TestSetAppendsAMissingField(t *testing.T) {
	in := "---\ntitle: A\ntype: concept\n---\nbody\n"
	p := mustParse(t, in)
	if err := p.Set(FieldStatus, StatusArchived); err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "---\ntitle: A\ntype: concept\nstatus: archived\n---\nbody\n"
	if got := string(p.Bytes()); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// A file with no frontmatter is still a page, so it can be given one.
func TestSetCreatesFrontmatterWhenThereIsNone(t *testing.T) {
	p := mustParse(t, "just a body\n")
	if p.HasFrontmatter() {
		t.Fatal("HasFrontmatter is true for a file with no block")
	}
	if err := p.Set(FieldTitle, "A new page"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := p.Set(FieldType, TypeConcept); err != nil {
		t.Fatalf("Set: %v", err)
	}

	want := "---\ntitle: A new page\ntype: concept\n---\njust a body\n"
	if got := string(p.Bytes()); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	if got := string(p.Body()); got != "just a body\n" {
		t.Errorf("body = %q", got)
	}
}

// Values that YAML would otherwise reinterpret have to survive the encoder.
func TestSetQuotesAwkwardValues(t *testing.T) {
	for _, value := range []string{
		"a: b",
		"#hashtag",
		"- leading dash",
		"yes",
		"009",
		"",
		"trailing space ",
		"multi\nline",
		"C++",
	} {
		p := mustParse(t, "---\ntitle: x\ntype: concept\n---\n")
		if err := p.Set(FieldTitle, value); err != nil {
			t.Fatalf("Set(%q): %v", value, err)
		}
		// Re-read, so the check is that the bytes are valid YAML holding the
		// same value, not that our own encoder agrees with itself.
		reparsed, err := ParsePage(p.Bytes())
		if err != nil {
			t.Fatalf("re-parsing after Set(%q): %v\n%s", value, err, p.Bytes())
		}
		if got := reparsed.Title(); got != value {
			t.Errorf("title round-tripped as %q, want %q\n%s", got, value, p.Bytes())
		}
	}
}

func TestDeleteRemovesOnlyItsOwnField(t *testing.T) {
	in := "---\ntitle: A\ntype: concept\nstatus: archived\n---\nbody\n"
	p := mustParse(t, in)
	if err := p.Delete(FieldStatus); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	want := "---\ntitle: A\ntype: concept\n---\nbody\n"
	if got := string(p.Bytes()); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestDeleteAbsentFieldChangesNothing(t *testing.T) {
	in := "---\ntitle: A\ntype: concept\n---\nbody\n"
	p := mustParse(t, in)
	if err := p.Delete(FieldStatus); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := string(p.Bytes()); got != in {
		t.Errorf("got:\n%q\nwant:\n%q", got, in)
	}
}

// A file using CRLF keeps using it, including on lines the tool adds.
func TestLineTerminatorsArePreserved(t *testing.T) {
	in := "---\r\ntitle: A\r\ntype: concept\r\n---\r\nbody\r\n"
	p := mustParse(t, in)
	if got := string(p.Bytes()); got != in {
		t.Errorf("round trip changed terminators: %q", got)
	}
	if err := p.Set(FieldStatus, StatusActive); err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "---\r\ntitle: A\r\ntype: concept\r\nstatus: active\r\n---\r\nbody\r\n"
	if got := string(p.Bytes()); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// The tool refuses files where it cannot tell what it would be rewriting.
func TestRefusedFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"unclosed frontmatter", "---\ntitle: A\n"},
		{"not utf-8", "---\ntitle: \xff\xfe\n---\n"},
		{"byte-order mark", "\ufeff---\ntitle: A\n---\n"},
		{"frontmatter is not yaml", "---\ntitle: [unclosed\n---\n"},
		{"frontmatter is a sequence", "---\n- a\n- b\n---\n"},
		{"frontmatter is a scalar", "---\njust a word\n---\n"},
		{"duplicate field", "---\ntitle: A\ntitle: B\n---\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParsePage([]byte(tc.raw)); err == nil {
				t.Errorf("ParsePage accepted %q", tc.raw)
			}
		})
	}
}

// None of the refusals may be triggered by a page that is merely unusual.
func TestUnusualButValidPages(t *testing.T) {
	for _, raw := range []string{
		"",
		"body only, no frontmatter",
		"---\n---\n",
		"---\n---\nbody\n",
		"---\ntitle: A\ntype: concept\n---",
		"---\n  \n---\nbody\n",
	} {
		if _, err := ParsePage([]byte(raw)); err != nil {
			t.Errorf("ParsePage(%q): %v", raw, err)
		}
	}
}

func TestAccessorsReadTheParsedValues(t *testing.T) {
	p := mustParse(t, gnarly)
	if got := p.Title(); got != "Attention Is All You Need" {
		t.Errorf("Title = %q", got)
	}
	if got := p.Type(); got != TypeConcept {
		t.Errorf("Type = %q", got)
	}
	if got := p.Status(); got != StatusActive {
		t.Errorf("Status = %q", got)
	}
	if got := p.Aliases(); len(got) != 2 || got[0] != "transformer paper" || got[1] != "vaswani2017" {
		t.Errorf("Aliases = %q", got)
	}
	if got := p.Keys(); !contains(got, "review") || !contains(got, "quoted key") {
		t.Errorf("Keys = %q", got)
	}
	if _, ok := p.Field("review"); !ok {
		t.Error("an unknown field is not reachable through Field")
	}
}

// Status has to be effective, because a page with no status is active and the
// rest of the tool should not have to know that.
func TestStatusDefaultsToActive(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: concept\n---\n")
	if got := p.Status(); got != StatusActive {
		t.Errorf("Status = %q, want %q", got, StatusActive)
	}
}

func TestAliasesGivesUpOnAMalformedValue(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: concept\naliases: not a list\n---\n")
	if got := p.Aliases(); got != nil {
		t.Errorf("Aliases = %q, want nil", got)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// A write that cannot be completed must leave the page as it was. The tool
// refuses rather than half-writes, because a page it has rewritten into
// something it can no longer read is worse than one it declined to touch: the
// next command cannot even open it to fix it.
func TestAFailedSetLeavesThePageAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		edit func(*Page) error
	}{
		{
			// "-foo" is a valid YAML key that the line-based key scanner does
			// not recognise, so writing it appends a second copy and the block
			// stops parsing as a mapping with a key defined twice.
			"Set over a key it did not recognise",
			"---\n-foo: 1\nstatus: active\n---\nbody\n",
			func(p *Page) error { return p.Set("-foo", 2) },
		},
		{
			// An alias needs the anchor it names, so removing the field that
			// carries the anchor leaves the rest of the block unresolvable.
			"Delete an anchor another field aliases",
			"---\ntitle: A\ntype: concept\nbase: &b\n  x: 1\nderived: *b\n---\nbody\n",
			func(p *Page) error { return p.Delete("base") },
		},
	} {
		p := mustParse(t, tc.in)
		before := string(p.Bytes())
		if err := tc.edit(p); err == nil {
			t.Fatalf("%s: expected the write to fail", tc.name)
		}
		if after := string(p.Bytes()); after != before {
			t.Errorf("%s: the page changed:\n--- was ---\n%s\n--- now ---\n%s", tc.name, before, after)
		}
		if _, err := ParsePage(p.Bytes()); err != nil {
			t.Errorf("%s: the tool can no longer read the page it just wrote: %v", tc.name, err)
		}
	}
}

// Deleting a field that is not there is not a failure, and changes nothing.
func TestDeletingAnAbsentFieldDoesNotFail(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: concept\n---\nbody\n")
	before := string(p.Bytes())
	if err := p.Delete("nothing_here"); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if after := string(p.Bytes()); after != before {
		t.Errorf("the page changed:\n%s", after)
	}
}

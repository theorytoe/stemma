package kb

import (
	"reflect"
	"testing"
)

func pageWithBody(t *testing.T, body string) *Page {
	t.Helper()
	return mustParse(t, "---\ntitle: A\ntype: concept\n---\n"+body)
}

func targets(links []Link) []string {
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, l.Target)
	}
	return out
}

func TestLinksAreFoundInOrder(t *testing.T) {
	p := pageWithBody(t, "See [[One]] and [[Two]], then [[One]] again.\n")
	got := targets(p.Links())
	want := []string{"One", "Two", "One"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("links = %q, want %q", got, want)
	}
}

func TestLinkLineNumbersAreLinesInTheFile(t *testing.T) {
	p := pageWithBody(t, "first\n\nthird [[X]]\n")
	links := p.Links()
	if len(links) != 1 {
		t.Fatalf("links = %q", targets(links))
	}
	// The file is frontmatter on lines 1-4, so the body starts at line 5 and
	// the link is on the third line of it.
	if links[0].Line != 7 {
		t.Errorf("line = %d, want 7", links[0].Line)
	}
}

func TestLinkTargetsAreTrimmed(t *testing.T) {
	p := pageWithBody(t, "[[  Some Page  ]]\n")
	links := p.Links()
	if len(links) != 1 {
		t.Fatalf("links = %q", targets(links))
	}
	if links[0].Target != "Some Page" {
		t.Errorf("target = %q", links[0].Target)
	}
	if links[0].Name != "some-page" {
		t.Errorf("name = %q", links[0].Name)
	}
}

// The specification writes `[[name]]` to describe the syntax. If code were read
// as prose, the project's own documentation would fail its own link check.
func TestLinksInsideCodeAreNotLinks(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"inline code span", "Write `[[X]]` to link.\n", nil},
		{"inline span beside a real link", "Write `[[X]]` then [[Y]].\n", []string{"Y"}},
		{"fenced block", "```\n[[X]]\n```\n", nil},
		{"fenced block with info string", "```markdown\n[[X]]\n```\n", nil},
		{"tilde fence", "~~~\n[[X]]\n~~~\n", nil},
		{"after the fence closes", "```\n[[X]]\n```\n[[Y]]\n", []string{"Y"}},
		{"indented fence", "  ```\n[[X]]\n  ```\n", nil},
		{"a longer run closes a shorter fence", "```\n[[X]]\n````\n", nil},
		{"an info string does not close a fence", "```\n[[X]]\n```go\n[[Y]]\n```\n", nil},
		{"double backticks hold a single backtick", "``a ` b`` [[Y]]\n", []string{"Y"}},
		// An unmatched backtick run is literal text, exactly as CommonMark says,
		// so the links after it are still links. A missing closer is a typo, not
		// an escape hatch.
		{"unmatched backtick is literal", "`[[X]] and [[Y]]\n", []string{"X", "Y"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := targets(pageWithBody(t, tc.body).Links())
			if len(got) != len(tc.want) {
				t.Fatalf("links = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("links = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

// A link is a line-level thing. Half of one is text.
func TestUnterminatedLinkIsText(t *testing.T) {
	p := pageWithBody(t, "[[open and never closed\n")
	if got := p.Links(); len(got) != 0 {
		t.Errorf("links = %q, want none", targets(got))
	}
}

func TestNoLinksInPlainProse(t *testing.T) {
	p := pageWithBody(t, "Brackets [one] and [[not closed and ] alone.\n")
	if got := p.Links(); len(got) != 0 {
		t.Errorf("links = %q, want none", targets(got))
	}
}

func TestEmptyLinkIsFound(t *testing.T) {
	p := pageWithBody(t, "[[]] and [[   ]]\n")
	links := p.Links()
	if len(links) != 2 {
		t.Fatalf("links = %d, want 2", len(links))
	}
	for _, l := range links {
		if l.Target != "" || l.Name != "" {
			t.Errorf("target = %q, name = %q", l.Target, l.Name)
		}
	}
}

func TestLinksInAFileWithNoFrontmatter(t *testing.T) {
	p := mustParse(t, "see [[X]]\n")
	links := p.Links()
	if len(links) != 1 {
		t.Fatalf("links = %q", targets(links))
	}
	if links[0].Line != 1 {
		t.Errorf("line = %d, want 1", links[0].Line)
	}
}

// The scanner reads the source rather than a markdown tree, which is what lets
// it report the line a link is on. The cost is three things markdown recognises
// as not-prose and this does not. They are pinned here so the behaviour is a
// decision on the record rather than an accident.
//
// The indented case is deliberate. A four-space indent is only a code block
// when it is not a list continuation, and guessing wrong the other way would
// hide real links, which is worse than reporting links that are not there.
func TestScannerTreatsSomeNonProseAsProse(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want []string
	}{
		{"indented code block", "para\n\n    [[X]]\n", []string{"X"}},
		{"raw html block", "<div>\n[[X]]\n</div>\n", []string{"X"}},
		{"fence inside a blockquote", "> ```\n> [[X]]\n> ```\n", []string{"X"}},
		{"prose inside a blockquote", "> see [[X]]\n", []string{"X"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := targets(pageWithBody(t, tc.body).Links())
			if len(got) != len(tc.want) {
				t.Fatalf("links = %q, want %q", got, tc.want)
			}
		})
	}
}

// A closing fence may be longer than the one that opened the block, and
// markdown renders the block as closed when it is. Reading it as still-open
// code would swallow everything after it: a link below would go unseen, and the
// page it names would be reported as an orphan instead — a finding about the
// wrong file.
func TestALongerFenceClosesTheBlock(t *testing.T) {
	for _, tc := range []struct{ open, close string }{
		{"```", "````"},
		{"````", "`````"},
		{"~~~", "~~~~"},
	} {
		body := tc.open + "\ncode\n" + tc.close + "\n\nsee [[Target]]\n"
		p := pageWithBody(t, body)
		if got := targets(p.Links()); !reflect.DeepEqual(got, []string{"Target"}) {
			t.Errorf("opened %q and closed %q: links = %q, want [Target]", tc.open, tc.close, got)
		}
	}
}

// The run may be longer, but it may not be shorter, and nothing but whitespace
// may follow it.
func TestAShorterOrTrailingFenceDoesNotCloseTheBlock(t *testing.T) {
	for _, tc := range []struct{ open, close string }{
		{"````", "```"},
		{"````", "```` ```"},
		{"~~~", "~~~ not a close"},
	} {
		body := tc.open + "\ncode\n" + tc.close + "\n\nsee [[Target]]\n"
		p := pageWithBody(t, body)
		if got := targets(p.Links()); len(got) != 0 {
			t.Errorf("opened %q and closed %q: links = %q, want none", tc.open, tc.close, got)
		}
	}
}

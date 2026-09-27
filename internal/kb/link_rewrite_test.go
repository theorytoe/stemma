package kb

import (
	"strings"
	"testing"
)

func rewrite(t *testing.T, raw string, rewrite func(Link) (string, bool)) string {
	t.Helper()
	p := mustParse(t, raw)
	p.RewriteLinks(rewrite)
	return string(p.Bytes())
}

func TestRewriteLinksReplacesTargets(t *testing.T) {
	got := rewrite(t, "---\ntitle: A\ntype: concept\n---\nsee [[Old]] and [[Other]]\n",
		func(l Link) (string, bool) {
			if l.Name == "old" {
				return "New", true
			}
			return "", false
		})
	want := "---\ntitle: A\ntype: concept\n---\nsee [[New]] and [[Other]]\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Everything the tool does not own comes back byte for byte, including the
// frontmatter it has no reason to touch.
func TestRewriteLinksTouchesNothingElse(t *testing.T) {
	in := "---\ntitle: A\ntype: concept\n# a comment\nunknown:\n  nested: true\n---\n" +
		"Line one stays.\n\n[[Old]] on its own line.\nTrailing text [[Old]] more.\n"
	got := rewrite(t, in, func(l Link) (string, bool) { return "New", true })

	for _, want := range []string{
		"# a comment",
		"unknown:\n  nested: true",
		"Line one stays.",
		"[[New]] on its own line.",
		"Trailing text [[New]] more.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Count(got, "\n") != strings.Count(in, "\n") {
		t.Errorf("the number of lines changed:\n%s", got)
	}
}

// A link inside code is not a link, so it is not offered and not rewritten.
func TestRewriteLinksLeavesCodeAlone(t *testing.T) {
	in := "---\ntitle: A\ntype: concept\n---\n" +
		"prose [[Old]] and `[[Old]]` inline.\n\n" +
		"```\n[[Old]]\n```\n\n" +
		"after [[Old]]\n"
	got := rewrite(t, in, func(l Link) (string, bool) { return "New", true })

	if !strings.Contains(got, "prose [[New]] and `[[Old]]` inline.") {
		t.Errorf("the inline code span was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "```\n[[Old]]\n```") {
		t.Errorf("the fenced block was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "after [[New]]") {
		t.Errorf("the link after the fence was not rewritten:\n%s", got)
	}
}

// The whole point of returning whether anything changed is that a caller can
// leave a file alone, so a rewrite with nothing to do must say so.
func TestRewriteLinksReportsNoChange(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: concept\n---\nsee [[Old]]\n")
	if changed := p.RewriteLinks(func(Link) (string, bool) { return "", false }); changed {
		t.Error("RewriteLinks claimed a change it did not make")
	}
	if got := string(p.Bytes()); !strings.Contains(got, "[[Old]]") {
		t.Errorf("the page changed anyway:\n%s", got)
	}
}

// Rewriting a link to the same thing it already says is not a change, which is
// what makes running a rewrite twice the same as running it once.
func TestRewriteLinksIsIdempotent(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: concept\n---\nsee [[Old]] and [[New]]\n")
	toNew := func(l Link) (string, bool) {
		if l.Name == "old" {
			return "New", true
		}
		return "", false
	}

	if changed := p.RewriteLinks(toNew); !changed {
		t.Fatal("the first rewrite changed nothing")
	}
	after := string(p.Bytes())
	if changed := p.RewriteLinks(toNew); changed {
		t.Errorf("the second rewrite claimed another change:\n%s", p.Bytes())
	}
	if got := string(p.Bytes()); got != after {
		t.Errorf("the second rewrite changed the page:\n%q\n%q", got, after)
	}
}

// The callback is given the same view of a link that the resolver works from,
// so a caller can decide using resolution without re-parsing anything.
func TestRewriteLinksOffersResolvableLinks(t *testing.T) {
	p := mustParse(t, "---\ntitle: A\ntype: concept\n---\nsee [[  Spaced Name  ]] on line 5\n")
	var seen []Link
	p.RewriteLinks(func(l Link) (string, bool) {
		seen = append(seen, l)
		return "", false
	})
	if len(seen) != 1 {
		t.Fatalf("offered %d links, want 1", len(seen))
	}
	if seen[0].Target != "Spaced Name" {
		t.Errorf("Target = %q, want it trimmed", seen[0].Target)
	}
	if seen[0].Name != "spaced-name" {
		t.Errorf("Name = %q", seen[0].Name)
	}
	if seen[0].Line != 5 {
		t.Errorf("Line = %d, want 5", seen[0].Line)
	}
}

func TestRewriteLinksHandlesBracketsAndEdges(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		// The callback here rewrites everything it is offered, and an empty
		// target is a link like any other, so it is rewritten too.
		{"empty target", "[[]]\n", "[[New]]\n"},
		{"unterminated", "[[Old\n", "[[Old\n"},
		{"back to back", "[[Old]][[Old]]\n", "[[New]][[New]]\n"},
		{"at the start", "[[Old]] then\n", "[[New]] then\n"},
		{"at the end", "then [[Old]]\n", "then [[New]]\n"},
		{"no links at all", "plain prose\n", "plain prose\n"},
		{"a bracket alone", "a [ b ] c\n", "a [ b ] c\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := "---\ntitle: A\ntype: concept\n---\n" + tc.body
			got := rewrite(t, in, func(l Link) (string, bool) { return "New", true })
			if !strings.HasSuffix(got, tc.want) {
				t.Errorf("got %q, want it to end with %q", got, tc.want)
			}
		})
	}
}

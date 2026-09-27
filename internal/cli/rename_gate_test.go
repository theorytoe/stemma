package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// This file is the evidence for the gate: whether rewriting inbound links is
// reliable enough to justify title-based links at all. It leans on the
// implementation harder than the ordinary tests do, and it records where the
// guarantee stops as carefully as it records where it holds.

// unresolvedSet is the set of links that resolve to nothing, so that two runs
// can be compared. The page and line identify one: a rewrite never changes how
// many lines a file has.
func unresolvedSet(t *testing.T, root string) map[string]bool {
	t.Helper()
	k, err := kb.Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	out := map[string]bool{}
	for _, f := range k.Graph.Findings(kb.Lenient) {
		if f.Code == kb.CodeUnresolvedLink {
			out[fmt.Sprintf("%s:%d", f.Path, f.Line)] = true
		}
	}
	return out
}

// Every spelling that names a page has to be rewritten, and every spelling that
// does not name it has to be left exactly as it was. Getting one of these wrong
// in either direction is the whole risk.
func TestRenameHandlesEverySpellingThatNamesThePage(t *testing.T) {
	root := freshKB(t)
	write := func(name, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Each of these names the page and must be rewritten.
	referrers := map[string]string{
		"exact":           "a [[Alpha Beta]] link\n",
		"lowercase":       "a [[alpha beta]] link\n",
		"uppercase":       "a [[ALPHA BETA]] link\n",
		"slug":            "a [[alpha-beta]] link\n",
		"spaced":          "a [[   Alpha    Beta   ]] link\n",
		"punctuated":      "a sentence, [[Alpha Beta]], with commas;\n",
		"adjacent":        "[[Alpha Beta]][[Alpha Beta]]\n",
		"line-edge":       "[[Alpha Beta]]\ntrailing [[Alpha Beta]]",
		"three-in-a-line": "[[Alpha Beta]] and [[Alpha Beta]] and [[Alpha Beta]]\n",
		"heading":         "## A heading naming [[Alpha Beta]]\n",
		"blockquote":      "> a quote naming [[Alpha Beta]]\n",
	}
	// Each of these does not name the page, or is not a link.
	untouched := map[string]string{
		"alias":     "a [[alfa]] link\n",
		"different": "a [[AlphaBeta]] link\n",
		"code-span": "write `[[Alpha Beta]]` to link\n",
		"fenced":    "```\n[[Alpha Beta]]\n```\n",
	}

	write("pages/index.md", page("Demo", "type: index", "the root, and [[alfa]]\n"))
	write("pages/target.md", page("Alpha Beta", "type: concept\naliases:\n  - alfa", ""))
	for name, body := range referrers {
		write("pages/ref-"+name+".md", page("Ref "+name, "type: concept", body))
	}
	for name, body := range untouched {
		write("pages/keep-"+name+".md", page("Keep "+name, "type: concept", body))
	}

	// One link never resolves, on purpose, to prove that a link which merely
	// looks like the title is not rewritten.
	before := unresolvedSet(t, root)
	if len(before) == 0 {
		t.Fatal("the fixture has no deliberately broken link; this test would prove nothing")
	}

	if code, _, stderr := run("rename", "--kb", root, "Alpha Beta", "Gamma Delta"); code != ExitOK {
		t.Fatalf("rename: exit %d: %s", code, stderr)
	}

	// No link anywhere may still name the old title.
	k, err := kb.Load(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	old := kb.Normalize("Alpha Beta")
	for _, path := range k.Graph.Paths() {
		page, _ := k.Graph.Page(path)
		for _, l := range page.Links() {
			if l.Name == old {
				t.Errorf("%s:%d still names the old title: %q", path, l.Line, l.Target)
			}
		}
	}

	// Nothing that should not have changed, did.
	for name, body := range untouched {
		path := "pages/keep-" + name + ".md"
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), strings.TrimSpace(body)) {
			t.Errorf("%s was changed when it should not have been:\n%s", path, got)
		}
	}

	// And no link that resolved before stopped resolving.
	for key := range unresolvedSet(t, root) {
		if !before[key] {
			t.Errorf("the rename introduced a broken link at %s", key)
		}
	}
}

// The same thing at the size a KB actually reaches, because a rewrite that
// works on three files and drops one in three hundred is not reliable.
func TestRenameRewritesEveryReferrerAtScale(t *testing.T) {
	const referrers = 300
	root := freshKB(t)

	index := "the root\n"
	files := map[string]string{"pages/target.md": page("Alpha Beta", "type: concept", "")}
	for i := 0; i < referrers; i++ {
		name := fmt.Sprintf("pages/ref-%03d.md", i)
		files[name] = page(fmt.Sprintf("Ref %d", i), "type: concept",
			"points at [[Alpha Beta]] and nothing else\n")
		index += fmt.Sprintf("[[Ref %d]]\n", i)
	}
	files["pages/index.md"] = page("Demo", "type: index", index)

	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if code, _, stderr := run("rename", "--kb", root, "Alpha Beta", "Gamma Delta"); code != ExitOK {
		t.Fatalf("rename: exit %d: %s", code, stderr)
	}

	if got := unresolvedSet(t, root); len(got) != 0 {
		t.Errorf("the rename left %d links unresolved", len(got))
	}
	k, err := kb.Load(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if r := k.Graph.Resolve("Gamma Delta"); r.Kind != kb.Resolved {
		t.Errorf("the new title does not resolve: %+v", r)
	}
	// Every referrer still links somewhere, and it is the renamed page.
	for i := 0; i < referrers; i++ {
		path := fmt.Sprintf("pages/ref-%03d.md", i)
		referrer, _ := k.Graph.Page(path)
		links := referrer.Links()
		if len(links) != 1 {
			t.Fatalf("%s has %d links, want 1", path, len(links))
		}
		if r := k.Graph.Resolve(links[0].Name); r.Kind != kb.Resolved || r.Path != "pages/gamma-delta.md" {
			t.Errorf("%s points at %+v", path, r)
		}
	}
}

// Where the guarantee stops.
//
// A draft is not part of the KB, so its links are not links and rename does not
// read it. The stale link is neither lost nor silent: promoting the draft turns
// it into a page, and lint reports it at once. The limit is a delay, not a
// corruption, and that is the difference between a bounded cost and a broken
// format.
func TestRenameDoesNotReachIntoTheInbox(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Demo", "type: index", "at [[Alpha]]\n"),
		"pages/alpha.md": page("Alpha", "type: concept", ""),
		"inbox/half.md":  page("Half", "type: concept", "a draft naming [[Alpha]]\n"),
	})

	if code, _, stderr := run("rename", "--kb", root, "Alpha", "Beta"); code != ExitOK {
		t.Fatalf("rename: exit %d: %s", code, stderr)
	}

	draft, err := os.ReadFile(filepath.Join(root, "inbox", "half.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(draft), "[[Alpha]]") {
		t.Fatal("the draft was rewritten; the assumption behind this test has changed")
	}

	if err := os.Rename(filepath.Join(root, "inbox", "half.md"), filepath.Join(root, "pages", "half.md")); err != nil {
		t.Fatal(err)
	}
	_, stdout, _ := run("lint", "--kb", root)
	if !strings.Contains(stdout, `no page is called "Alpha"`) {
		t.Errorf("the stale link was not reported once the draft became a page:\n%s", stdout)
	}
}

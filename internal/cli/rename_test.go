package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kbWith links a small KB together: an index that names every page, and an
// alpha page that carries an alias, an unknown field, and a link to itself.
func kbWith(t *testing.T) string {
	t.Helper()
	root := freshKB(t)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pages/index.md", page("Demo", "type: index",
		"Start at [[Alpha]] and also [[alfa]] and [[Beta]].\n"))
	write("pages/alpha.md", page("Alpha", "type: concept\naliases:\n  - alfa\nunknown_field: kept",
		"Links [[Beta]] and itself as [[Alpha]], and once as [[ALPHA]] and once as [[alpha]].\n"))
	write("pages/beta.md", page("Beta", "type: concept", "Back to [[Alpha]].\n"))
	return root
}

func TestRenameRewritesEveryLinkThatNamedThePage(t *testing.T) {
	root := kbWith(t)
	if code, _, stderr := run("lint", "--kb", root, "--strict"); code != ExitOK {
		t.Fatalf("the KB does not start clean: %s", stderr)
	}

	code, stdout, stderr := run("rename", "--kb", root, "Alpha", "Self Attention")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "pages/alpha.md -> pages/self-attention.md") {
		t.Errorf("stdout = %q", stdout)
	}

	// Every spelling of the title is gone, and the alias is untouched.
	for _, name := range []string{"pages/index.md", "pages/self-attention.md", "pages/beta.md"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "[[Alpha]]") || strings.Contains(string(body), "[[ALPHA]]") ||
			strings.Contains(string(body), "[[alpha]]") {
			t.Errorf("%s still names the old title:\n%s", name, body)
		}
	}
	index, _ := os.ReadFile(filepath.Join(root, "pages", "index.md"))
	if !strings.Contains(string(index), "[[Self Attention]]") {
		t.Errorf("the title link was not rewritten:\n%s", index)
	}
	// A link by alias still names the page, so it is left as the author wrote
	// it rather than edited for its own sake.
	if !strings.Contains(string(index), "[[alfa]]") {
		t.Errorf("a link by alias was rewritten:\n%s", index)
	}

	if code, stdout, stderr := run("lint", "--kb", root, "--strict"); code != ExitOK {
		t.Fatalf("the KB is not clean after the rename: exit %d\n%s\n%s", code, stdout, stderr)
	}
}

func TestRenameKeepsEverythingElse(t *testing.T) {
	root := kbWith(t)
	run("rename", "--kb", root, "Alpha", "Self Attention")

	body, err := os.ReadFile(filepath.Join(root, "pages", "self-attention.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"unknown_field: kept", "aliases:\n  - alfa", "title: Self Attention"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
}

// The old name must not still resolve to the page, or the rename would have
// added a second way to reach it rather than replaced one.
func TestRenameRetiresTheOldTitle(t *testing.T) {
	root := kbWith(t)
	run("rename", "--kb", root, "Alpha", "Self Attention")

	if code, _, _ := run("show", "--kb", root, "Alpha", "--path"); code == ExitOK {
		t.Error("the old title still resolves")
	}
	code, stdout, stderr := run("show", "--kb", root, "Self Attention", "--path")
	if code != ExitOK {
		t.Fatalf("the new title does not resolve: %s", stderr)
	}
	if strings.TrimSpace(stdout) != "pages/self-attention.md" {
		t.Errorf("--path = %q", stdout)
	}
}

func TestRenameByAliasOrSlugWorks(t *testing.T) {
	for _, name := range []string{"Alpha", "alpha", "alfa", "ALPHA"} {
		t.Run(name, func(t *testing.T) {
			root := kbWith(t)
			if code, _, stderr := run("rename", "--kb", root, name, "Renamed"); code != ExitOK {
				t.Errorf("rename by %q: exit %d: %s", name, code, stderr)
			}
		})
	}
}

func TestRenameRefusesWhatWouldBreakTheKB(t *testing.T) {
	for _, tc := range []struct {
		name    string
		page    string
		newName string
		want    string
	}{
		{"a title another page already has", "Alpha", "Beta", "already the name of"},
		{"a title another page answers to as an alias", "Beta", "alfa", "already the name of"},
		{"a title nothing could link to", "Alpha", "!!!", "nothing could link to it"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := kbWith(t)
			before := filepath.Join(root, "pages", "alpha.md")
			code, _, stderr := run("rename", "--kb", root, tc.page, tc.newName)
			if code != ExitError {
				t.Errorf("exit = %d, want %d", code, ExitError)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want it to mention %q", stderr, tc.want)
			}
			// Nothing was written: the KB is exactly as it was.
			if _, err := os.Lstat(before); err != nil {
				t.Errorf("a page moved anyway: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(root, "pages", "beta.md")); err != nil {
				t.Errorf("a page moved anyway: %v", err)
			}
		})
	}
}

// Renaming a page to a name it already answers to as an alias is allowed, if
// pointless: the alias becomes the title and the page keeps one name, not two.
func TestRenameToItsOwnAliasIsAllowed(t *testing.T) {
	root := kbWith(t)
	if code, _, stderr := run("rename", "--kb", root, "Alpha", "alfa"); code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if code, _, stderr := run("lint", "--kb", root, "--strict"); code != ExitOK {
		t.Errorf("the KB is not clean: %s", stderr)
	}
}

// A page nothing links to is the easy case: there is nothing to rewrite, and it
// must still work.
func TestRenameAPageWithNoInboundLinks(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Lonely", "--type", "concept")

	code, stdout, stderr := run("rename", "--kb", root, "Lonely", "Still Lonely")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "rewriting 0 links") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Lstat(filepath.Join(root, "pages", "still-lonely.md")); err != nil {
		t.Errorf("the file was not renamed: %v", err)
	}
}

// A link inside code is not a link and must not be rewritten, or a page
// documenting the old title would be edited behind the author's back.
func TestRenameLeavesLinksInCodeAlone(t *testing.T) {
	root := freshKB(t)
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pages/index.md", page("Demo", "type: index", "at [[Alpha]]\n"))
	write("pages/alpha.md", page("Alpha", "type: concept",
		"write `[[Alpha]]` to link here\n\n```\n[[Alpha]]\n```\n"))

	run("rename", "--kb", root, "Alpha", "Beta Page")

	body, _ := os.ReadFile(filepath.Join(root, "pages", "beta-page.md"))
	if !strings.Contains(string(body), "write `[[Alpha]]` to link here") {
		t.Errorf("the inline code span was rewritten:\n%s", body)
	}
	if !strings.Contains(string(body), "```\n[[Alpha]]\n```") {
		t.Errorf("the fenced block was rewritten:\n%s", body)
	}
}

// When the old title is also an alias of another page, every link to it was
// ambiguous and so did not mean this page. Those links are left alone and
// reported, rather than being quietly retargeted at the page being renamed.
func TestRenameLeavesAmbiguousLinksAlone(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":  page("Demo", "type: index", "refers to [[go]] here\n"),
		"pages/go.md":     page("Go", "type: concept", ""),
		"pages/golang.md": page("Golang", "type: concept\naliases:\n  - go", ""),
	})

	if code, _, _ := run("rename", "--kb", root, "go", "Go Language"); code != ExitError {
		t.Fatal("an ambiguous name was accepted as the page to rename")
	}

	// By path there is no ambiguity, which is what makes a collision repairable
	// with the tool rather than only by hand.
	code, stdout, stderr := run("rename", "--kb", root, "pages/go.md", "Go Language")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stderr, "ambiguous link") {
		t.Errorf("the ambiguous link was not reported: stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "rewriting 0 links") {
		t.Errorf("stdout = %q", stdout)
	}

	index, _ := os.ReadFile(filepath.Join(root, "pages", "index.md"))
	if !strings.Contains(string(index), "[[go]]") {
		t.Errorf("the ambiguous link was rewritten:\n%s", index)
	}
	// And now it resolves, to the page that still answers to that name.
	_, shown, _ := run("show", "--kb", root, "Demo")
	if !strings.Contains(shown, "[[go]] -> pages/golang.md") {
		t.Errorf("[[go]] no longer resolves to Golang:\n%s", shown)
	}
}

// A rename writes several files. Interrupting it must leave a KB that the same
// command completes, rather than one that needs hand repair.
func TestRenameResumesAfterAnInterruption(t *testing.T) {
	for _, tc := range []struct {
		name string
		// The state a run would leave behind at each point it could stop.
		files map[string]string
	}{
		{
			// Stopped after moving the file, before rewriting anything.
			name: "after the move",
			files: map[string]string{
				"pages/index.md":     page("Demo", "type: index", "see [[Old Title]] and [[Old Title]]\n"),
				"pages/new-title.md": page("Old Title", "type: concept", ""),
			},
		},
		{
			// Stopped after rewriting some links, before the move or retitle.
			name: "after some rewrites",
			files: map[string]string{
				"pages/index.md":     page("Demo", "type: index", "see [[New Title]] and [[Old Title]]\n"),
				"pages/old-title.md": page("Old Title", "type: concept", ""),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := writeTree(t, tc.files)
			code, _, stderr := run("rename", "--kb", root, "Old Title", "New Title")
			if code != ExitOK {
				t.Fatalf("the resumed rename failed: exit %d: %s", code, stderr)
			}
			index, _ := os.ReadFile(filepath.Join(root, "pages", "index.md"))
			if strings.Contains(string(index), "Old Title") {
				t.Errorf("a link still names the old title:\n%s", index)
			}
			if code, stdout, stderr := run("lint", "--kb", root, "--strict"); code != ExitOK {
				t.Fatalf("the resumed KB is not clean: exit %d\n%s\n%s", code, stdout, stderr)
			}
		})
	}
}

func TestRenameJSON(t *testing.T) {
	root := kbWith(t)
	_, stdout, _ := run("rename", "--kb", root, "Alpha", "Self Attention", "--json")

	var got struct {
		From      string `json:"from"`
		To        string `json:"to"`
		OldTitle  string `json:"old_title"`
		NewTitle  string `json:"new_title"`
		Rewritten int    `json:"links_rewritten"`
		Files     int    `json:"files_rewritten"`
	}
	decodeData(t, stdout, &got)
	if got.From != "pages/alpha.md" || got.To != "pages/self-attention.md" {
		t.Errorf("got %+v", got)
	}
	if got.OldTitle != "Alpha" || got.NewTitle != "Self Attention" {
		t.Errorf("got %+v", got)
	}
	// Five spellings in three files: index once, beta once, alpha three times.
	if got.Rewritten != 5 || got.Files != 3 {
		t.Errorf("rewrote %d links in %d files, want 5 in 3", got.Rewritten, got.Files)
	}
}

// Renaming a page that is not there must fail without touching anything.
func TestRenameNeedsAPageThatExists(t *testing.T) {
	root := freshKB(t)
	if code, _, stderr := run("rename", "--kb", root, "Nothing", "Something"); code != ExitError {
		t.Errorf("exit = %d: %s", code, stderr)
	}
	if code, _, _ := run("rename", "--kb", root, "Demo"); code != ExitError {
		t.Error("rename with one argument was accepted")
	}
}

// The check at the end of a rename compares the links that resolved before
// against the links that resolve after. A dangling link is a warning rather
// than an error, so a KB may have them, and renaming a page that carries one
// must not be reported as having broken it.
//
// The comparison is keyed by page and line, and the renamed page's own path
// changes when its file moves, so the page has to be named by where it was
// before for its links to compare equal to themselves.
func TestRenameSurvivesADanglingLinkInThePageBeingRenamed(t *testing.T) {
	root := freshKB(t)
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("pages/index.md", page("Demo", "type: index", "[[Alpha]] and [[Beta]].\n"))
	write("pages/alpha.md", page("Alpha", "type: concept", "A pre-existing dangling link to [[Nowhere]].\n"))
	write("pages/beta.md", page("Beta", "type: concept", "Back to [[Alpha]].\n"))

	// Renaming to a title that slugs differently moves the file, which is what
	// changes the path the dangling link is reported under.
	code, stdout, stderr := run("rename", "--kb", root, "Alpha", "Alpha Renamed")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\n%s\n%s", code, ExitOK, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "pages", "alpha-renamed.md")); err != nil {
		t.Errorf("the page did not move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "pages", "alpha.md")); err == nil {
		t.Error("the old file is still there")
	}

	// The link that was already dangling is still dangling, and it is still the
	// only thing wrong: the rename added nothing.
	code, stdout, _ = run("lint", "--kb", root)
	if code != ExitFindings {
		t.Fatalf("exit = %d, want %d for the one pre-existing warning", code, ExitFindings)
	}
	if got := strings.Count(stdout, "no page is called"); got != 1 {
		t.Errorf("unresolved links = %d, want 1:\n%s", got, stdout)
	}
	if strings.Contains(stdout, ": error:") {
		t.Errorf("the rename introduced an error:\n%s", stdout)
	}
}

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/index"
)

func searchTestKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md":     page("Index", "type: index", ""),
		"pages/retrieval.md": page("Retrieval", "type: concept\ntags: [machine-learning]", "a short page\n"),
		"pages/alpha.md":     page("Alpha", "type: concept", "retrieval retrieval here\n"),
		"inbox/idea.md":      "---\ntitle: An Idea\ntype: concept\n---\na draft about retrieval\n",
	})
}

type searchResults struct {
	Tier    string `json:"tier"`
	Count   int    `json:"count"`
	Results []struct {
		Path    string  `json:"path"`
		Title   string  `json:"title"`
		Score   float64 `json:"score"`
		Snippet string  `json:"snippet"`
	} `json:"results"`
}

func runSearch(t *testing.T, root string, args ...string) searchResults {
	t.Helper()
	full := append([]string{"search", "--kb", root, "--json"}, args...)
	code, stdout, stderr := run(full...)
	if code != ExitOK {
		t.Fatalf("search %v: exit %d: %s", args, code, stderr)
	}
	var got searchResults
	decodeData(t, stdout, &got)
	return got
}

func TestSearchCommandRanksTitleFirst(t *testing.T) {
	root := searchTestKB(t)
	code, stdout, stderr := run("search", "--kb", root, "retrieval")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "pages/retrieval.md") {
		t.Errorf("the title match did not rank first:\n%s", stdout)
	}
}

// Every result is one line, however much the body it was cut from wrapped.
func TestSearchCommandPutsEachResultOnOneLine(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/prose.md": page("Prose", "type: concept",
			"alpha beta\n\nretrieval happens here\n\n- and then more\n"),
	})
	code, stdout, stderr := run("search", "--kb", root, "retrieval")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("search printed %d lines for one hit:\n%s", len(lines), stdout)
	}
	if !strings.Contains(lines[0], "[retrieval] happens here") {
		t.Errorf("line = %q, want the match and its context", lines[0])
	}
}

// A pipe gets plain, uncolored text with the snippet's marks left as brackets,
// because that is what a caller can split and a downstream tool can read.
func TestSearchCommandPlainWhenPiped(t *testing.T) {
	root := searchTestKB(t)
	code, stdout, stderr := run("search", "--kb", root, "retrieval")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("a pipe got ANSI escapes:\n%q", stdout)
	}
	if !strings.Contains(stdout, "[retrieval]") {
		t.Errorf("the plain form lost its marks:\n%q", stdout)
	}
}

func TestRenderSnippetPlainKeepsBrackets(t *testing.T) {
	parts := index.SplitSnippet("before [retrieval] after", []string{"retrieval"})
	if got := renderSnippet(parts, false, 0); got != "before [retrieval] after" {
		t.Errorf("plain snippet = %q", got)
	}
}

func TestRenderSnippetColorReplacesBrackets(t *testing.T) {
	parts := index.SplitSnippet("before [retrieval] after", []string{"retrieval"})
	got := renderSnippet(parts, true, 0)
	want := "before " + ansiMatch + "retrieval" + ansiReset + " after"
	if got != want {
		t.Errorf("colored snippet = %q, want %q", got, want)
	}
}

// Clipping counts what a reader sees: escapes do not eat into the width, and a
// mark's brackets do not either.
func TestRenderSnippetClipsToWidth(t *testing.T) {
	long := "start [retrieval] " + strings.Repeat("word ", 20)
	parts := index.SplitSnippet(long, []string{"retrieval"})

	for _, tc := range []struct {
		name  string
		color bool
	}{
		{"plain", false},
		{"color", true},
	} {
		got := renderSnippet(parts, tc.color, 30)
		if !strings.HasSuffix(got, "...") {
			t.Errorf("%s: snippet = %q, want a trailing ellipsis", tc.name, got)
		}
		if n := len([]rune(stripANSI(got))); n > 30 {
			t.Errorf("%s: snippet is %d columns, want at most 30: %q", tc.name, n, got)
		}
	}
}

// A term that does not fit is dropped whole rather than half-drawn, and the
// line still ends with the ellipsis that says so.
func TestRenderSnippetDoesNotSplitAMark(t *testing.T) {
	parts := index.SplitSnippet(strings.Repeat("x", 40)+" [retrieval]", []string{"retrieval"})
	got := stripANSI(renderSnippet(parts, true, 10))
	if strings.Contains(got, "ret") {
		t.Errorf("snippet = %q, want no partial mark", got)
	}
}

func stripANSI(s string) string {
	r := strings.NewReplacer(ansiPath, "", ansiMatch, "", ansiReset, "")
	return r.Replace(s)
}

func TestColorDisabledOffATerminal(t *testing.T) {
	var b bytes.Buffer
	if colorEnabled(&b) {
		t.Error("a buffer was treated as a terminal")
	}
	if terminalWidth(&b) != 0 {
		t.Error("a buffer reported a terminal width")
	}
}

func TestSearchCommandJSONAndFallback(t *testing.T) {
	root := searchTestKB(t)

	got := runSearch(t, root, "retrieval")
	if got.Tier != "kb" {
		t.Errorf("tier without an index = %q, want kb", got.Tier)
	}
	if got.Count != 2 {
		t.Errorf("count = %d, want 2: %+v", got.Count, got.Results)
	}
	if got.Results[0].Path != "pages/retrieval.md" || got.Results[0].Score <= 0 {
		t.Errorf("top result = %+v", got.Results[0])
	}

	// Building the index changes which tier answers, not the answer.
	if code, _, stderr := run("index", "--kb", root, "--json"); code != ExitOK {
		t.Fatalf("index: exit %d: %s", code, stderr)
	}
	indexed := runSearch(t, root, "retrieval")
	if indexed.Tier != "index" {
		t.Errorf("tier with a fresh index = %q, want index", indexed.Tier)
	}
	if indexed.Count != got.Count || indexed.Results[0].Path != got.Results[0].Path {
		t.Errorf("the tiers disagree: %+v vs %+v", indexed.Results, got.Results)
	}
}

func TestSearchCommandFilters(t *testing.T) {
	root := searchTestKB(t)

	if got := runSearch(t, root, "retrieval", "--tag", "machine-learning"); got.Count != 1 || got.Results[0].Path != "pages/retrieval.md" {
		t.Errorf("by tag = %+v", got.Results)
	}
	if got := runSearch(t, root, "retrieval", "--type", "note"); got.Count != 0 {
		t.Errorf("by type = %+v, want none", got.Results)
	}
	if got := runSearch(t, root, "retrieval", "--limit", "1"); got.Count != 1 {
		t.Errorf("limit = %+v", got.Results)
	}
}

func TestSearchCommandIncludeInbox(t *testing.T) {
	root := searchTestKB(t)

	if got := runSearch(t, root, "retrieval"); got.Count != 2 {
		t.Errorf("drafts leaked into a plain search: %+v", got.Results)
	}
	got := runSearch(t, root, "retrieval", "--include-inbox")
	if got.Count != 3 {
		t.Fatalf("include-inbox count = %d, want 3: %+v", got.Count, got.Results)
	}
	found := false
	for _, r := range got.Results {
		if r.Path == "inbox/idea.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("the draft was not returned: %+v", got.Results)
	}
}

func TestSearchRejectsBadInput(t *testing.T) {
	root := searchTestKB(t)
	for _, args := range [][]string{
		{"search", "--kb", root, "a", "b"},
		{"search", "--kb", root, "--status", "draft"},
		{"search", "--kb", root, "--limit", "-1"},
		{"search", "--kb", root, "--dir", "../outside"},
	} {
		if code, _, _ := run(args...); code != ExitError {
			t.Errorf("%v: exit %d, want %d", args, code, ExitError)
		}
	}
}

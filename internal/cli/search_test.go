package cli

import (
	"strings"
	"testing"
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
	if !strings.HasPrefix(stdout, "pages/retrieval.md\t") {
		t.Errorf("the title match did not rank first:\n%s", stdout)
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

package index

import (
	"strings"
	"testing"
)

func TestSnippetMarksAndTruncates(t *testing.T) {
	body := strings.Repeat("filler ", 30) + "the retrieval of documents " + strings.Repeat("more ", 30)
	got := Snippet(body, []string{"retrieval"}, 60)
	if !strings.Contains(got, "[retrieval]") {
		t.Errorf("snippet = %q, want the match marked", got)
	}
	if !strings.HasPrefix(got, "...") || !strings.HasSuffix(got, "...") {
		t.Errorf("a match in the middle should be truncated at both ends: %q", got)
	}
}

// A word reached through the accent fold is shown as the page spells it, not as
// the query typed it.
func TestSnippetKeepsTheOriginalSpelling(t *testing.T) {
	got := Snippet("a café here", []string{"cafe"}, 0)
	if !strings.Contains(got, "[café]") {
		t.Errorf("snippet = %q, want the original spelling highlighted", got)
	}
}

func TestSnippetNoMatch(t *testing.T) {
	if got := Snippet("nothing here", []string{"absent"}, 0); got != "" {
		t.Errorf("snippet = %q, want empty", got)
	}
	if got := Snippet("", []string{"absent"}, 0); got != "" {
		t.Errorf("snippet of an empty body = %q, want empty", got)
	}
}

// A snippet never cuts a word in half, even when the window is too small to
// hold the match comfortably.
func TestSnippetDoesNotSplitWords(t *testing.T) {
	got := Snippet("alpha retrieval omega", []string{"retrieval"}, 12)
	if !strings.Contains(got, "[retrieval]") {
		t.Fatalf("snippet = %q, want the whole word marked", got)
	}
}

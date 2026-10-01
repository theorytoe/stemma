package index

import "testing"

func TestSplitSnippetSeparatesMarks(t *testing.T) {
	got := SplitSnippet("before [retrieval] after", []string{"retrieval"})
	want := []SnippetPart{
		{Text: "before ", Match: false},
		{Text: "retrieval", Match: true},
		{Text: " after", Match: false},
	}
	if len(got) != len(want) {
		t.Fatalf("parts = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("part %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// A body carries its own brackets, as every wikilink does, and they must not be
// read as marks; the word inside one still matches.
func TestSplitSnippetKeepsLiteralBrackets(t *testing.T) {
	got := SplitSnippet("see [[Beta]] about it", []string{"beta"})
	if len(got) != 3 {
		t.Fatalf("parts = %+v, want three", got)
	}
	if got[0].Text != "see [" || got[0].Match {
		t.Errorf("leading text = %+v, want the literal bracket kept", got[0])
	}
	if got[1].Text != "Beta" || !got[1].Match {
		t.Errorf("mark = %+v, want the word inside the link", got[1])
	}
	if got[2].Text != "] about it" || got[2].Match {
		t.Errorf("trailing text = %+v, want the closing bracket kept", got[2])
	}
}

// The terms are the tokenizer's output, so a query typed with an accent matches
// a snippet that spells the word without one.
func TestSplitSnippetMatchesByToken(t *testing.T) {
	got := SplitSnippet("a [café] here", []string{"cafe"})
	if len(got) != 3 || !got[1].Match || got[1].Text != "café" {
		t.Errorf("parts = %+v, want the folded term marked", got)
	}
}

func TestSplitSnippetNoMarks(t *testing.T) {
	got := SplitSnippet("plain text", []string{"absent"})
	if len(got) != 1 || got[0].Match || got[0].Text != "plain text" {
		t.Errorf("parts = %+v, want one literal run", got)
	}
}

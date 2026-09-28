package kb

import (
	"reflect"
	"testing"
)

func TestDraftPath(t *testing.T) {
	if got := DraftPath("Half A Thought"); got != "inbox/half-a-thought.md" {
		t.Errorf("DraftPath = %q", got)
	}
}

func TestDraftPathsAreSortedAndIgnoreTheInboxLessKB(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md":    pageSource("Index", "type: index", ""),
		"inbox/b.md":        "---\ntitle: B\n---\n",
		"inbox/a.md":        "---\ntitle: A\n---\n",
		"inbox/deep/c.md":   "---\ntitle: C\n---\n",
		"inbox/notes.txt":   "not a draft\n",
		"pages/a-page.md":   pageSource("A Page", "type: concept", ""),
		"bibliography.bib":  "@article{k,}\n",
		"inbox/ignored.md":  "---\ntitle: Ignored\n---\n",
		"inbox/.hidden/tmp": "x",
	})

	// A path the manifest ignores is not a draft either.
	raw := []byte("ignore = [\"inbox/ignored.md\"]\n")
	m, err := ParseManifest("kb", raw)
	if err != nil {
		t.Fatal(err)
	}
	k.Manifest = m

	got, err := k.DraftPaths()
	if err != nil {
		t.Fatalf("DraftPaths: %v", err)
	}
	want := []string{"inbox/a.md", "inbox/b.md", "inbox/deep/c.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DraftPaths = %q, want %q", got, want)
	}
}

func TestDraftPathsOnAKBWithNoInbox(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md": pageSource("Index", "type: index", ""),
	})
	got, err := k.DraftPaths()
	if err != nil {
		t.Fatalf("DraftPaths: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("DraftPaths = %q, want none", got)
	}
}

func TestReadDraft(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md": pageSource("Index", "type: index", ""),
		"inbox/half.md":  "---\ntitle: Half A Thought\n---\nbody\n",
	})
	p, err := k.ReadDraft("inbox/half.md")
	if err != nil {
		t.Fatalf("ReadDraft: %v", err)
	}
	if p.Title() != "Half A Thought" {
		t.Errorf("Title = %q", p.Title())
	}
	if !p.AnswersTo("half a thought") {
		t.Error("the draft's title did not answer to its own name")
	}
}

// A draft is not a page, so Load must not see it, and a draft that cannot be
// parsed is still only read when something asks about the inbox.
func TestLoadDoesNotReadTheInbox(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md":  pageSource("Index", "type: index", ""),
		"inbox/broken.md": "---\ntitle: Unclosed\n",
	})
	if _, ok := k.Graph.Page("inbox/broken.md"); ok {
		t.Error("Load read a draft as a page")
	}
	if k.Graph.Len() != 1 {
		t.Errorf("pages = %d, want 1", k.Graph.Len())
	}
}

package kb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesAKBRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new-kb")
	if err := Init(root, "Demo"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for _, name := range []string{ManifestName, EntryDocument, BibliographyName, InboxDir} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("init did not create %s: %v", name, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, SourcesDir)); !os.IsNotExist(err) {
		t.Error("init created sources/, which is absent unless something is vendored")
	}

	// What init writes has to be a KB the tool can read back, because that is
	// the only thing that makes the scaffold true rather than decorative.
	k, err := Load(root)
	if err != nil {
		t.Fatalf("the KB init wrote does not load: %v", err)
	}
	if k.Manifest.Title != "Demo" {
		t.Errorf("title = %q", k.Manifest.Title)
	}
	if len(k.Manifest.Unknown) != 0 {
		t.Errorf("init wrote keys the tool does not know: %q", k.Manifest.Unknown)
	}
	if got := k.Lint(Strict); len(got) != 0 {
		t.Errorf("a fresh KB is not clean: %+v", got)
	}
}

// The title goes through a TOML file and back, so a title that needs quoting
// has to survive it.
func TestInitQuotesTheTitle(t *testing.T) {
	for _, title := range []string{
		`Alan's "big" KB`,
		`back\slash`,
		`tab	here`,
		"日本語の知識ベース",
	} {
		root := filepath.Join(t.TempDir(), "kb")
		if err := Init(root, title); err != nil {
			t.Fatalf("Init(%q): %v", title, err)
		}
		k, err := Load(root)
		if err != nil {
			t.Fatalf("Load after Init(%q): %v", title, err)
		}
		if k.Manifest.Title != title {
			t.Errorf("title round-tripped as %q, want %q", k.Manifest.Title, title)
		}
	}
}

func TestInitDefaultsTheTitleToTheDirectoryName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my-notes")
	if err := Init(root, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}
	k, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if k.Manifest.Title != "my-notes" {
		t.Errorf("title = %q, want my-notes", k.Manifest.Title)
	}
}

// Every file init writes is a file it would otherwise be replacing.
func TestInitRefusesToRunOverAKB(t *testing.T) {
	root := filepath.Join(t.TempDir(), "kb")
	if err := Init(root, "First"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(root, EntryDocument))
	if err != nil {
		t.Fatal(err)
	}

	if err := Init(root, "Second"); err == nil {
		t.Fatal("Init ran over a KB that already existed")
	}
	after, err := os.ReadFile(filepath.Join(root, EntryDocument))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the refused init changed the entry document anyway")
	}
}

func TestInitRefusesWhenOnlyOneThingIsInTheWay(t *testing.T) {
	for _, name := range []string{ManifestName, PagesDir, BibliographyName} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := Init(root, "Demo"); err == nil {
				t.Errorf("Init ran with %s in the way", name)
			}
		})
	}
}

func TestNewPageHasTheRequiredFrontmatter(t *testing.T) {
	p, err := NewPage("Attention Is All You Need", TypeConcept)
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if p.Title() != "Attention Is All You Need" {
		t.Errorf("Title = %q", p.Title())
	}
	if p.Type() != TypeConcept {
		t.Errorf("Type = %q", p.Type())
	}
	if len(p.Body()) != 0 {
		t.Errorf("a new page has a body: %q", p.Body())
	}
	if got := string(p.Bytes()); got != "---\ntitle: Attention Is All You Need\ntype: concept\n---\n" {
		t.Errorf("Bytes = %q", got)
	}
}

func TestPagePathUsesTheSlug(t *testing.T) {
	for _, tc := range []struct{ dir, title, want string }{
		{"pages", "Attention Is All You Need", "pages/attention-is-all-you-need.md"},
		{"pages", "C++", "pages/c++.md"},
		{"inbox", "half a thought", "inbox/half-a-thought.md"},
		{"pages", "  padded  ", "pages/padded.md"},
	} {
		if got := PagePath(tc.dir, tc.title); got != tc.want {
			t.Errorf("PagePath(%q, %q) = %q, want %q", tc.dir, tc.title, got, tc.want)
		}
	}
}

func TestCreatePageRefusesToReplace(t *testing.T) {
	root := t.TempDir()
	k, err := Load(writeTree(t, map[string]string{"pages/a.md": pageSource("A", "type: concept", "")}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_ = root

	page, err := NewPage("B", TypeConcept)
	if err != nil {
		t.Fatal(err)
	}
	if err := k.CreatePage("pages/b.md", page); err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if err := k.CreatePage("pages/b.md", page); err == nil {
		t.Error("CreatePage replaced a page that was already there")
	}
}

// A failed write must not cost a page. The write goes to a temporary neighbour
// and is renamed, so the old bytes are never truncated.
func TestWritePageReplacesWholly(t *testing.T) {
	k, err := Load(writeTree(t, map[string]string{
		"pages/a.md": pageSource("A", "type: concept", "original body\n"),
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	page, _ := k.Graph.Page("pages/a.md")
	if err := page.Set(FieldStatus, StatusArchived); err != nil {
		t.Fatal(err)
	}
	if err := k.WritePage("pages/a.md", page); err != nil {
		t.Fatalf("WritePage: %v", err)
	}

	reloaded, err := Load(k.Root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, _ := reloaded.Graph.Page("pages/a.md")
	if got.Status() != StatusArchived {
		t.Errorf("status = %q", got.Status())
	}
	if string(got.Body()) != "original body\n" {
		t.Errorf("body = %q", got.Body())
	}
}

// No temporary file may survive a write, and none may be mistaken for a page.
func TestWriteLeavesNoTemporaryFiles(t *testing.T) {
	k, err := Load(writeTree(t, map[string]string{
		"pages/a.md": pageSource("A", "type: concept", ""),
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	page, _ := k.Graph.Page("pages/a.md")
	if err := k.WritePage("pages/a.md", page); err != nil {
		t.Fatalf("WritePage: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(k.Root, PagesDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "a.md" {
			t.Errorf("a write left %q behind", e.Name())
		}
	}
}

func TestMovePageRefusesToReplace(t *testing.T) {
	k, err := Load(writeTree(t, map[string]string{
		"pages/a.md":     pageSource("A", "type: concept", ""),
		"pages/b.md":     pageSource("B", "type: concept", ""),
		"pages/sub/c.md": pageSource("C", "type: concept", ""),
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if err := k.MovePage("pages/a.md", "pages/b.md"); err == nil {
		t.Error("MovePage replaced a page")
	}
	if err := k.MovePage("pages/a.md", "pages/sub/a.md"); err != nil {
		t.Fatalf("MovePage: %v", err)
	}
	if _, err := os.Lstat(k.Path("pages/a.md")); !os.IsNotExist(err) {
		t.Error("the page is still at its old path")
	}
	if _, err := os.Lstat(k.Path("pages/sub/a.md")); err != nil {
		t.Errorf("the page is not at its new path: %v", err)
	}
}

func TestMovePageNeedsTheSource(t *testing.T) {
	k, err := Load(writeTree(t, map[string]string{"pages/a.md": pageSource("A", "type: concept", "")}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := k.MovePage("pages/nope.md", "pages/sub/nope.md"); err == nil {
		t.Error("MovePage moved a page that is not there")
	}
}

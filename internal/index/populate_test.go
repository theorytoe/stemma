package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// buildKB writes a KB to a temporary directory and loads it, so population is
// exercised against the same page model the commands use.
func buildKB(t *testing.T, files map[string]string) *kb.KB {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		writeFile(t, root, name, body)
	}
	k, err := kb.Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return k
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func pageFile(title, extra, body string) string {
	s := "---\ntitle: " + title + "\n"
	if extra != "" {
		s += extra + "\n"
	}
	return s + "---\n" + body
}

func count(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

const findMatch = `SELECT pages.path FROM pages_fts JOIN pages ON pages.id = pages_fts.rowid
	WHERE pages_fts MATCH ?`

func TestPopulateWritesEverything(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/index.md":   pageFile("Index", "type: index", "start at [[Alpha]] and cite [@key]\n"),
		"pages/alpha.md":   pageFile("Alpha", "type: concept\ntags: [Machine-Learning]", "prose about retrieval\n"),
		"bibliography.bib": "@article{key,}\n",
	})
	s := open(t, k.Root)
	if _, err := s.Populate(k); err != nil {
		t.Fatalf("Populate: %v", err)
	}

	if got := count(t, s, `SELECT count(*) FROM pages`); got != 2 {
		t.Errorf("pages = %d, want 2", got)
	}
	if got := count(t, s, `SELECT count(*) FROM names`); got != 2 {
		t.Errorf("names = %d, want 2", got)
	}
	if got := count(t, s, `SELECT count(*) FROM links`); got != 1 {
		t.Errorf("links = %d, want 1", got)
	}
	if got := count(t, s, `SELECT count(*) FROM citations`); got != 1 {
		t.Errorf("citations = %d, want 1", got)
	}
	if got := count(t, s, `SELECT count(*) FROM tags WHERE tag = 'machine-learning'`); got != 1 {
		t.Errorf("normalised tag rows = %d, want 1", got)
	}
	if got := queryString(t, s.db, findMatch, "retrieval"); got != "pages/alpha.md" {
		t.Errorf("search for retrieval = %q, want pages/alpha.md", got)
	}
	if got := count(t, s, `SELECT count(*) FROM pages WHERE length <= 0`); got != 0 {
		t.Errorf("%d pages have no weighted length", got)
	}
	if got := count(t, s, `SELECT count(DISTINCT hash) FROM pages`); got != 2 {
		t.Errorf("distinct content hashes = %d, want 2", got)
	}
}

func TestPopulateIsRepeatable(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/a.md": pageFile("A", "type: concept", "alpha\n"),
		"pages/b.md": pageFile("B", "type: concept", "beta\n"),
	})
	s := open(t, k.Root)
	if _, err := s.Populate(k); err != nil {
		t.Fatal(err)
	}
	first := count(t, s, `SELECT count(*) FROM pages`)
	if _, err := s.Populate(k); err != nil {
		t.Fatal(err)
	}
	if got := count(t, s, `SELECT count(*) FROM pages`); got != first {
		t.Errorf("pages after a second build = %d, want %d", got, first)
	}
	if got := count(t, s, `SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'alpha'`); got != 1 {
		t.Errorf("matches after a second build = %d, want 1", got)
	}
}

// The index stores the same folded tokens Tier 0 would produce, so an accented
// word is found by its unaccented spelling.
func TestPopulateIndexesFoldedAccents(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/a.md": pageFile("Café", "type: concept", "a naïve turn\n"),
	})
	s := open(t, k.Root)
	if _, err := s.Populate(k); err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{"cafe", "naive"} {
		if got := queryString(t, s.db, findMatch, term); got != "pages/a.md" {
			t.Errorf("search for %q = %q, want pages/a.md", term, got)
		}
	}
}

// A source page answers to no name, matching the in-memory graph, so a title
// that happens to equal a citation key cannot collide with it.
func TestPopulateSkipsSourceNames(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/s.md": pageFile("Some Key", "type: source\nkey: somekey", "text\n"),
	})
	s := open(t, k.Root)
	if _, err := s.Populate(k); err != nil {
		t.Fatal(err)
	}
	if got := count(t, s, `SELECT count(*) FROM names WHERE path = 'pages/s.md'`); got != 0 {
		t.Errorf("a source page claimed %d names, want 0", got)
	}
}

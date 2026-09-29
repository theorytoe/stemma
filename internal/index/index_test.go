package index

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// open opens the index and fails the test if it cannot, remembering to close it.
func open(t *testing.T, root string) *Store {
	t.Helper()
	s, err := Open(root)
	if err != nil {
		t.Fatalf("Open(%s): %v", root, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// insertTestPage writes one page row directly. Population is exercised separately;
// these tests need the schema to have something to hold.
func insertTestPage(t *testing.T, s *Store, path, title, body string, tags []string) {
	t.Helper()
	titleTokens := Tokenize(title)
	var tagTokens []string
	for _, tag := range tags {
		tagTokens = append(tagTokens, Tokenize(tag)...)
	}
	bodyTokens := Tokenize(body)
	doc := Doc{Title: titleTokens, Tags: tagTokens, Body: bodyTokens}
	all := append(append(append([]string{}, titleTokens...), tagTokens...), bodyTokens...)

	if _, err := s.db.Exec(
		`INSERT INTO pages(path, title, type, status, body, tokens, hash, length)
		 VALUES (?, ?, 'concept', 'active', ?, ?, 'sha256:x', ?)`,
		path, title, body, Join(all), doc.Length()); err != nil {
		t.Fatalf("insert %s: %v", path, err)
	}
	if _, err := s.db.Exec(`INSERT INTO names(path, name, kind) VALUES (?, ?, 'title')`, path, strings.ToLower(title)); err != nil {
		t.Fatalf("insert name for %s: %v", path, err)
	}
	for _, tag := range tags {
		if _, err := s.db.Exec(`INSERT INTO tags(path, tag) VALUES (?, ?)`, path, tag); err != nil {
			t.Fatalf("insert tag for %s: %v", path, err)
		}
	}
}

// queryString runs a query expected to return one row and one column.
func queryString(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var got string
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return got
}

func TestPathIsInsideTheGeneratedDirectory(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, kb.GeneratedDir, "index.sqlite")
	if got := Path(root); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestOpenCreatesTheSchema(t *testing.T) {
	root := t.TempDir()
	s := open(t, root)

	if s.Path() != Path(root) {
		t.Errorf("Path() = %q, want %q", s.Path(), Path(root))
	}
	if _, err := os.Stat(Path(root)); err != nil {
		t.Fatalf("the cache was not created: %v", err)
	}
	if got := queryString(t, s.db, "PRAGMA user_version"); got != strconv.Itoa(SchemaVersion) {
		t.Errorf("user_version = %s, want %d", got, SchemaVersion)
	}

	// Every object the schema names is really there, looked up independently of
	// verify so this test cannot pass by agreeing with itself.
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		have[name] = true
	}
	for _, name := range required {
		if !have[name] {
			t.Errorf("the schema has no %s", name)
		}
	}
}

// Opening an index a second time leaves what is in it alone: the schema matches,
// so there is nothing to rebuild.
func TestOpenIsIdempotent(t *testing.T) {
	root := t.TempDir()
	first := open(t, root)
	insertTestPage(t, first, "pages/a.md", "A", "alpha", nil)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second := open(t, root)
	if got := queryString(t, second.db, `SELECT title FROM pages WHERE path = 'pages/a.md'`); got != "A" {
		t.Errorf("title after reopening = %q, want %q", got, "A")
	}
}

// A file written by another schema version is rebuilt rather than read: the
// version marker is what makes that decision instead of a silent misparse.
func TestOpenRebuildsAnIncompatibleSchemaVersion(t *testing.T) {
	root := t.TempDir()
	s := open(t, root)
	insertTestPage(t, s, "pages/a.md", "A", "alpha", nil)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open(driverName, Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	rebuilt := open(t, root)
	if got := queryString(t, rebuilt.db, "PRAGMA user_version"); got != strconv.Itoa(SchemaVersion) {
		t.Errorf("user_version after rebuild = %s, want %d", got, SchemaVersion)
	}
	var count int
	if err := rebuilt.db.QueryRow(`SELECT count(*) FROM pages`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("pages survived a rebuild: %d", count)
	}
}

// A file that is not a database at all is replaced, because a cache has nothing
// worth refusing over.
func TestOpenRebuildsAFileThatIsNotADatabase(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, kb.GeneratedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte("this is not a database"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := open(t, root)
	if got := queryString(t, s.db, "PRAGMA user_version"); got != strconv.Itoa(SchemaVersion) {
		t.Errorf("user_version = %s, want %d", got, SchemaVersion)
	}
	insertTestPage(t, s, "pages/a.md", "A", "alpha", nil)
}

// A version match with an object missing is still rebuilt: a build that died
// partway through creating the schema must not be trusted because its header
// says the right number.
func TestOpenRebuildsAPartialSchema(t *testing.T) {
	root := t.TempDir()
	s := open(t, root)
	if _, err := s.db.Exec(`DROP TABLE pages_fts`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	rebuilt := open(t, root)
	if got := queryString(t, rebuilt.db,
		`SELECT name FROM sqlite_master WHERE name = 'pages_fts'`); got != "pages_fts" {
		t.Errorf("pages_fts is still missing: %q", got)
	}
}

// The text table is real FTS5 over the tokens Go produced: a page is
// searchable, a prefix reaches a longer word, a hyphenated phrase is found in
// the right order, and the contentless table can be cleared.
func TestFTS5IndexesGoTokens(t *testing.T) {
	root := t.TempDir()
	s := open(t, root)
	insertTestPage(t, s, "pages/retrieval.md", "Retrieval", "indexing documents for retrieval", []string{"machine-learning"})

	const find = `SELECT pages.path FROM pages_fts JOIN pages ON pages.id = pages_fts.rowid
		WHERE pages_fts MATCH ?`
	if got := queryString(t, s.db, find, "retrieval"); got != "pages/retrieval.md" {
		t.Errorf("word match = %q, want pages/retrieval.md", got)
	}
	if got := queryString(t, s.db, find, "retriev*"); got != "pages/retrieval.md" {
		t.Errorf("prefix match = %q, want pages/retrieval.md", got)
	}
	// A hyphenated tag is two tokens, and a bare `-` is FTS5's NOT operator, so
	// the correct query is a quoted phrase. Escaping a user's query into FTS5
	// syntax is the search command's job, not the schema's.
	if got := queryString(t, s.db, find, `"machine-learning"`); got != "pages/retrieval.md" {
		t.Errorf("tag match = %q, want pages/retrieval.md", got)
	}

	if _, err := s.db.Exec(`DELETE FROM pages`); err != nil {
		t.Fatalf("delete pages: %v", err)
	}
	var remaining int
	if err := s.db.QueryRow(
		`SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'retrieval'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("delete-all left %d matches", remaining)
	}
}

// Filtering columns are plain SQL, independent of the text table: type, status
// and tag are what the query commands narrow on.
func TestFilterColumns(t *testing.T) {
	root := t.TempDir()
	s := open(t, root)
	insertTestPage(t, s, "pages/a.md", "A", "body", []string{"one", "two"})

	if got := queryString(t, s.db, `SELECT tag FROM tags WHERE path = 'pages/a.md' ORDER BY tag LIMIT 1`); got != "one" {
		t.Errorf("first tag = %q, want one", got)
	}
	if got := queryString(t, s.db, `SELECT type FROM pages WHERE status = 'active'`); got != "concept" {
		t.Errorf("type = %q, want concept", got)
	}
}

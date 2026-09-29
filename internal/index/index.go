// Package index is the Tier-1 cache: a SQLite FTS5 store over a KB that makes
// search, backlinks and citation maps fast without ever becoming a source of
// truth.
//
// Tier 0 is the KB read directly, through internal/kb: files parsed on demand,
// the graph resolved in memory. It needs no setup, and every command works
// through it, so the index is an accelerator and never a prerequisite (D23,
// P1). The cache can be deleted at any time and rebuilt with no loss, which is
// what keeps it honest: a fact that exists only here is a bug.
//
// The store lives at .stemma/index.sqlite inside the KB root, in the same
// generated-artifact directory as the extraction script and the fetch scratch
// area. Nothing under .stemma/ is ever committed (D23, P8).
//
// The schema is versioned with SQLite's user_version. A file whose version is
// not this one, and a file that is not a database at all, are discarded and
// rebuilt rather than read with the wrong shape. Discarding is always safe
// here, because everything the cache holds can be regenerated from the KB, so
// the code takes the simple road: it deletes the file and starts over.
package index

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/theorytoe/stemma/internal/kb"

	// The pure-Go SQLite driver. It is registered as "sqlite" and needs no cgo,
	// so the binary stays statically linkable. FTS5 is compiled in by default.
	_ "modernc.org/sqlite"
)

// driverName is how the modernc driver registers itself with database/sql.
const driverName = "sqlite"

// FileName is the cache's name inside the generated-artifact directory.
const FileName = "index.sqlite"

// SchemaVersion identifies the schema this build writes. It is stored in the
// database's user_version header and bumped whenever the schema changes; a file
// carrying any other value is rebuilt rather than interpreted.
const SchemaVersion = 3

// Path returns the cache's location for a KB root. The file need not exist.
func Path(root string) string {
	return filepath.Join(root, kb.GeneratedDir, FileName)
}

// Store is an open index. It owns one database handle and nothing else; the
// pages it describes live in the KB on disk.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens the index for a KB root, creating the generated directory and the
// schema when they are absent.
//
// It repairs rather than refuses. A missing file is created, a file written by
// a different schema version is rebuilt, and a file that is not a database is
// replaced. Only a failure of the filesystem itself — a root that cannot be
// written — comes back as an error, and even then the caller is expected to
// fall back to Tier 0 rather than fail its command.
func Open(root string) (*Store, error) {
	dir := filepath.Join(root, kb.GeneratedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("%s: %w", dir, err)
	}

	path := Path(root)
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// One connection: pragmas are per-connection, and a build reads and writes
	// through a single handle anyway.
	db.SetMaxOpenConns(1)

	s := &Store{db: db, path: path}
	if err := s.prepare(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle. The file stays on disk.
func (s *Store) Close() error { return s.db.Close() }

// Path returns the file this store is backed by.
func (s *Store) Path() string { return s.path }

// prepare makes the file on disk match the schema this build expects, rebuilding
// it when it does not.
func (s *Store) prepare() error {
	if err := s.pragma(); err != nil {
		return err
	}
	// A version match is not enough on its own: a build that died partway
	// through creating the schema can leave the header claiming a version the
	// objects do not back. verify catches that, and rebuild fixes it.
	if v, err := s.schemaVersion(); err == nil && v == SchemaVersion && s.verify() == nil {
		return nil
	}
	return s.rebuild()
}

// rebuild discards whatever is on disk and writes the schema fresh.
//
// The file is removed rather than emptied, because a file that is not a
// database cannot be dropped from, and because removal is the one operation
// that works whatever state the cache is in. Nothing is lost: the cache is
// derived from the KB and the next build repopulates it.
func (s *Store) rebuild() error {
	if err := s.db.Close(); err != nil {
		return err
	}
	for _, name := range []string{s.path, s.path + "-journal", s.path + "-wal", s.path + "-shm"} {
		if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	db, err := sql.Open(driverName, s.path)
	if err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	db.SetMaxOpenConns(1)
	s.db = db

	if err := s.pragma(); err != nil {
		return err
	}
	if err := s.create(); err != nil {
		return err
	}
	if _, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)); err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	return nil
}

// pragma applies the connection settings. busy_timeout is the one that matters
// for a cache two processes might touch; the default journal keeps the index a
// single file, with no -wal sidecar to explain.
func (s *Store) pragma() error {
	if _, err := s.db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	return nil
}

// schemaVersion reads the version marker from the database header.
func (s *Store) schemaVersion() (int, error) {
	var v int
	err := s.db.QueryRow("PRAGMA user_version").Scan(&v)
	return v, err
}

// verify reports whether every object the schema defines is actually present.
func (s *Store) verify() error {
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	have := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		have[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, name := range required {
		if !have[name] {
			return fmt.Errorf("the index has no %s", name)
		}
	}
	return nil
}

// create writes every object in the schema. Every statement is idempotent, so
// this is safe on a file that already has part of the schema.
func (s *Store) create() error {
	for _, stmt := range schema {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", s.path, err)
		}
	}
	return nil
}

// required names the objects verify looks for. Shadow tables FTS5 creates for
// itself are left out: they are the virtual table's business, not the schema's.
var required = []string{
	"pages",
	"names",
	"links",
	"citations",
	"tags",
	"pages_fts",
	"pages_ai",
	"pages_ad",
	"pages_au",
}

// schema is the whole index, in the order it is created.
//
// It stores what the KB already holds and nothing else: the pages with the
// frontmatter the commands filter on (type, status, tags) and the body the
// search reads, the names pages answer to for resolution and backlinks, the
// links and citations found in bodies, and the FTS5 text table.
//
// The text table is external content over pages.tokens, which holds the tokens
// Tokenize produced rather than the raw prose. FTS5 is an inverted index over
// tokens this package defined, so its own tokenizer cannot decide what a page
// contains; that is what keeps the two tiers from disagreeing about a match.
// Snippets come from the body stored in pages, not from FTS5, for the same
// reason: one snippet function serves both tiers. The triggers keep the index in
// step when a page is inserted, updated or deleted, which is what lets a
// refresh touch only the pages that changed.
//
// pages is keyed by an integer id because FTS5 needs one for its rowid; path
// stays unique and is what everything else joins on. length is the weighted
// token count BM25 needs, computed by the same function that scores.
var schema = []string{
	`CREATE TABLE IF NOT EXISTS pages (
		id     INTEGER PRIMARY KEY,
		path   TEXT NOT NULL UNIQUE,
		title  TEXT NOT NULL,
		type   TEXT NOT NULL,
		status TEXT NOT NULL,
		body   TEXT NOT NULL DEFAULT '',
		tokens TEXT NOT NULL DEFAULT '',
		hash   TEXT NOT NULL,
		length REAL NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS pages_type ON pages(type)`,
	`CREATE INDEX IF NOT EXISTS pages_status ON pages(status)`,

	`CREATE TABLE IF NOT EXISTS names (
		path TEXT NOT NULL,
		name TEXT NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('title','alias')),
		PRIMARY KEY (path, name)
	)`,
	`CREATE INDEX IF NOT EXISTS names_name ON names(name)`,

	`CREATE TABLE IF NOT EXISTS links (
		from_path TEXT NOT NULL,
		ordinal   INTEGER NOT NULL,
		target    TEXT NOT NULL,
		name      TEXT NOT NULL,
		line      INTEGER NOT NULL,
		PRIMARY KEY (from_path, ordinal)
	)`,
	`CREATE INDEX IF NOT EXISTS links_name ON links(name)`,

	`CREATE TABLE IF NOT EXISTS citations (
		path    TEXT NOT NULL,
		ordinal INTEGER NOT NULL,
		key     TEXT NOT NULL,
		line    INTEGER NOT NULL,
		PRIMARY KEY (path, ordinal)
	)`,
	`CREATE INDEX IF NOT EXISTS citations_key ON citations(key)`,

	`CREATE TABLE IF NOT EXISTS tags (
		path TEXT NOT NULL,
		tag  TEXT NOT NULL,
		PRIMARY KEY (path, tag)
	)`,
	`CREATE INDEX IF NOT EXISTS tags_tag ON tags(tag)`,

	`CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
		tokens,
		content='pages',
		content_rowid='id',
		tokenize='unicode61 remove_diacritics 0',
		prefix='2 3 4'
	)`,

	`CREATE TRIGGER IF NOT EXISTS pages_ai AFTER INSERT ON pages BEGIN
		INSERT INTO pages_fts(rowid, tokens) VALUES (new.id, new.tokens);
	END`,
	`CREATE TRIGGER IF NOT EXISTS pages_ad AFTER DELETE ON pages BEGIN
		INSERT INTO pages_fts(pages_fts, rowid, tokens)
		VALUES ('delete', old.id, old.tokens);
	END`,
	`CREATE TRIGGER IF NOT EXISTS pages_au AFTER UPDATE OF tokens ON pages BEGIN
		INSERT INTO pages_fts(pages_fts, rowid, tokens)
		VALUES ('delete', old.id, old.tokens);
		INSERT INTO pages_fts(rowid, tokens) VALUES (new.id, new.tokens);
	END`,
}

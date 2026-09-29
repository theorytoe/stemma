package index

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"

	"github.com/theorytoe/stemma/internal/kb"
)

// Report says what a build or refresh did to the index.
type Report struct {
	// Pages is how many pages the index holds after the run.
	Pages int `json:"pages"`
	// Added, Updated and Removed count the pages this run rewrote.
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	// Rebuilt is true when the whole index was discarded first.
	Rebuilt bool `json:"rebuilt"`
}

// Refresh updates the index in place from the KB.
//
// A page whose content hash is unchanged is left alone: its row, text entry,
// names, links, citations and tags are not touched. That is what makes the
// stamp a content hash rather than a modification time. A file that was touched
// but not changed must not cause a rewrite, and a file changed on a machine
// whose clock is wrong must not be missed, and only the content itself can tell
// those apart.
func (s *Store) Refresh(k *kb.KB) (Report, error) {
	current := KBHashes(k)
	stored, err := s.storedHashes()
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", s.path, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", s.path, err)
	}
	defer tx.Rollback()

	var rep Report
	for _, path := range sortedKeys(stored) {
		if _, ok := current[path]; !ok {
			if err := removePage(tx, path); err != nil {
				return Report{}, fmt.Errorf("%s: %w", s.path, err)
			}
			rep.Removed++
		}
	}
	for _, path := range k.Graph.Paths() {
		old, present := stored[path]
		if present && old == current[path] {
			continue
		}
		page, ok := k.Graph.Page(path)
		if !ok {
			continue
		}
		// An updated page is removed and rewritten rather than patched: the
		// text entry, names, links, citations and tags all change together, and
		// one path through the writer is easier to keep correct than five.
		if present {
			if err := removePage(tx, path); err != nil {
				return Report{}, fmt.Errorf("%s: %w", s.path, err)
			}
			rep.Updated++
		} else {
			rep.Added++
		}
		if err := insertPage(tx, path, page); err != nil {
			return Report{}, fmt.Errorf("%s: %w", s.path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Report{}, fmt.Errorf("%s: %w", s.path, err)
	}
	rep.Pages = k.Graph.Len()
	return rep, nil
}

// removePage deletes a page and everything derived from it. Its text entry goes
// with the row, through the delete trigger.
func removePage(tx *sql.Tx, path string) error {
	if _, err := tx.Exec(`DELETE FROM pages WHERE path = ?`, path); err != nil {
		return err
	}
	for _, stmt := range []string{
		`DELETE FROM names WHERE path = ?`,
		`DELETE FROM links WHERE from_path = ?`,
		`DELETE FROM citations WHERE path = ?`,
		`DELETE FROM tags WHERE path = ?`,
	} {
		if _, err := tx.Exec(stmt, path); err != nil {
			return err
		}
	}
	return nil
}

// storedHashes returns the content digest the index recorded for each page.
func (s *Store) storedHashes() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT path, hash FROM pages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		out[path] = hash
	}
	return out, rows.Err()
}

// KBHashes returns the content digest of every page in a loaded KB. It is what
// a build stamps and what a refresh compares, so both use one definition of a
// page's content.
func KBHashes(k *kb.KB) map[string]string {
	out := make(map[string]string, k.Graph.Len())
	for _, path := range k.Graph.Paths() {
		if page, ok := k.Graph.Page(path); ok {
			out[path] = kb.HashOf(page.Bytes())
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// State says how an index compares with the KB it describes.
type State int

const (
	// Absent means there is no index to use.
	Absent State = iota
	// Stale means an index exists but does not match the KB.
	Stale
	// Fresh means the index matches the KB and can be used.
	Fresh
)

func (s State) String() string {
	switch s {
	case Fresh:
		return "fresh"
	case Stale:
		return "stale"
	default:
		return "absent"
	}
}

// Check reports whether the index for a KB root is absent, stale or fresh.
//
// It reads the KB's page bytes to hash them, which is cheaper than parsing but
// still reads every page. There is no shortcut: the stamp is a content hash, so
// the only way to know a page is unchanged is to hash it again. An error means
// the question could not be answered, and the state alongside it is not
// meaningful; a caller that cannot check should fall back to the KB.
func Check(root string) (State, error) {
	hashes, err := kb.Hashes(root)
	if err != nil {
		return Stale, err
	}
	return check(root, hashes)
}

// CheckKB is Check for a KB the caller has already loaded. It hashes the pages
// it already holds rather than reading them again.
func CheckKB(k *kb.KB) (State, error) {
	return check(k.Root, KBHashes(k))
}

// check compares an index with a set of page hashes.
func check(root string, hashes map[string]string) (State, error) {
	s, err := openReadOnly(root)
	if err != nil {
		return Absent, err
	}
	if s == nil {
		return Absent, nil
	}
	defer s.Close()

	// A file of the wrong version, or one the objects do not back, is not read:
	// it is stale, and rebuilding it is the index command's job, not a query's.
	if v, err := s.schemaVersion(); err != nil || v != SchemaVersion || s.verify() != nil {
		return Stale, nil
	}
	synced, err := s.Synced(hashes)
	if err != nil {
		return Stale, err
	}
	if synced {
		return Fresh, nil
	}
	return Stale, nil
}

// Synced reports whether the index's stamps equal the given page hashes.
func (s *Store) Synced(hashes map[string]string) (bool, error) {
	stored, err := s.storedHashes()
	if err != nil {
		return false, err
	}
	if len(stored) != len(hashes) {
		return false, nil
	}
	for path, hash := range stored {
		if hashes[path] != hash {
			return false, nil
		}
	}
	return true, nil
}

// openReadOnly opens an existing index without creating, repairing or
// rebuilding anything, and returns nil when there is no file.
//
// A freshness check must not write: reporting that an index is absent is a
// valid answer, and repairing one belongs to a command, not a query.
func openReadOnly(root string) (*Store, error) {
	path := Path(root)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err := s.pragma(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

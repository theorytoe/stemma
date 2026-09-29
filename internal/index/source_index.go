package index

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// indexSource is Tier 1: the SQLite cache. It answers from the index alone,
// never from the KB, which is what makes it worth having — the pages are not
// read at all.
type indexSource struct {
	s     *Store
	pages []PageMeta
}

func newIndexSource(s *Store) *indexSource { return &indexSource{s: s} }

func (s *indexSource) Tier() Tier { return TierIndex }

func (s *indexSource) Pages() ([]PageMeta, error) {
	if s.pages != nil {
		return s.pages, nil
	}
	tags, err := s.pageTags()
	if err != nil {
		return nil, err
	}
	rows, err := s.s.db.Query(`SELECT path, title, type, status FROM pages ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m PageMeta
		if err := rows.Scan(&m.Path, &m.Title, &m.Type, &m.Status); err != nil {
			return nil, err
		}
		m.Tags = tags[m.Path]
		s.pages = append(s.pages, m)
	}
	return s.pages, rows.Err()
}

// pageTags reads every page's normalised tags at once, so Pages is two queries
// rather than one per page.
func (s *indexSource) pageTags() (map[string][]string, error) {
	rows, err := s.s.db.Query(`SELECT path, tag FROM tags ORDER BY path, tag`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var path, tag string
		if err := rows.Scan(&path, &tag); err != nil {
			return nil, err
		}
		out[path] = append(out[path], tag)
	}
	return out, rows.Err()
}

func (s *indexSource) Doc(path string) (Doc, error) {
	var title, body string
	err := s.s.db.QueryRow(`SELECT title, body FROM pages WHERE path = ?`, path).Scan(&title, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return Doc{}, nil
	}
	if err != nil {
		return Doc{}, err
	}

	rows, err := s.s.db.Query(`SELECT tag FROM tags WHERE path = ? ORDER BY tag`, path)
	if err != nil {
		return Doc{}, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return Doc{}, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return Doc{}, err
	}
	return Doc{Title: Tokenize(title), Tags: tagTokens(tags), Body: Tokenize(body)}, nil
}

func (s *indexSource) Body(path string) (string, error) {
	var body string
	err := s.s.db.QueryRow(`SELECT body FROM pages WHERE path = ?`, path).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return body, err
}

func (s *indexSource) Match(terms []string) ([]string, error) {
	query := ftsQuery(terms)
	if query == "" {
		return nil, nil
	}
	rows, err := s.s.db.Query(
		`SELECT pages.path FROM pages_fts JOIN pages ON pages.id = pages_fts.rowid
		 WHERE pages_fts MATCH ? ORDER BY pages.path`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, rows.Err()
}

func (s *indexSource) DF(term string) (int, error) {
	if term == "" {
		return 0, nil
	}
	var n int
	if err := s.s.db.QueryRow(`SELECT count(*) FROM pages_fts WHERE pages_fts MATCH ?`, term).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *indexSource) AvgLength() (float64, error) {
	var avg float64
	err := s.s.db.QueryRow(`SELECT coalesce(avg(length), 0) FROM pages`).Scan(&avg)
	return avg, err
}

func (s *indexSource) Claimants(name string) ([]string, error) {
	return s.column(`SELECT path FROM names WHERE name = ? ORDER BY path`, kb.Normalize(name))
}

func (s *indexSource) Links(path string) ([]kb.Link, error) {
	rows, err := s.s.db.Query(
		`SELECT target, name, line FROM links WHERE from_path = ? ORDER BY ordinal`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []kb.Link
	for rows.Next() {
		var l kb.Link
		if err := rows.Scan(&l.Target, &l.Name, &l.Line); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Backlinks joins a page's names to the links naming them, and excludes a page
// that links to itself, which is the same rule the in-memory graph applies.
func (s *indexSource) Backlinks(path string) ([]string, error) {
	return s.column(
		`SELECT DISTINCT links.from_path FROM links
		 JOIN names ON links.name = names.name
		 WHERE names.path = ? AND links.from_path <> ?
		 ORDER BY links.from_path`, path, path)
}

func (s *indexSource) Citations(path string) ([]CitationRef, error) {
	rows, err := s.s.db.Query(
		`SELECT key, line FROM citations WHERE path = ? ORDER BY ordinal`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CitationRef
	for rows.Next() {
		var c CitationRef
		if err := rows.Scan(&c.Key, &c.Line); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *indexSource) CitedBy(key string) ([]string, error) {
	return s.column(`SELECT DISTINCT path FROM citations WHERE key = ? ORDER BY path`, key)
}

func (s *indexSource) Close() error { return s.s.Close() }

// column runs a query expected to return one text column.
func (s *indexSource) column(query string, args ...any) ([]string, error) {
	rows, err := s.s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ftsQuery turns Go tokens into an FTS5 OR expression.
//
// The tokens come from Tokenize, so they contain only letters and digits and
// none of FTS5's operators; there is nothing to escape and nothing a user could
// type that would become syntax. Empty terms are dropped.
func ftsQuery(terms []string) string {
	clean := make([]string, 0, len(terms))
	for _, t := range terms {
		if t != "" {
			clean = append(clean, t)
		}
	}
	return strings.Join(clean, " OR ")
}

package index

import (
	"database/sql"
	"fmt"

	"github.com/theorytoe/stemma/internal/kb"
)

// Populate replaces everything in the index with what the KB now holds.
//
// It is a rebuild of the contents, not of the file: the schema stays and every
// row is replaced inside one transaction, so a reader never sees the index half
// written. Incremental refresh is a later task's job; this is what it will fall
// back to, and what a first build does.
func (s *Store) Populate(k *kb.KB) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("%s: %w", s.path, err)
	}
	defer tx.Rollback()

	// A contentless FTS table cannot be emptied with DELETE; 'delete-all' is
	// the command it does understand.
	for _, stmt := range []string{
		`DELETE FROM pages`,
		`DELETE FROM names`,
		`DELETE FROM links`,
		`DELETE FROM citations`,
		`DELETE FROM tags`,
		`INSERT INTO pages_fts(pages_fts) VALUES ('delete-all')`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("%s: %w", s.path, err)
		}
	}

	// Sorted, because Graph.Paths is, so a build is reproducible byte for byte
	// when nothing changed.
	for _, path := range k.Graph.Paths() {
		page, ok := k.Graph.Page(path)
		if !ok {
			continue
		}
		if err := insertPage(tx, path, page); err != nil {
			return fmt.Errorf("%s: %w", s.path, err)
		}
	}
	return tx.Commit()
}

// insertPage writes one page and everything the graph knows about it.
func insertPage(tx *sql.Tx, path string, page *kb.Page) error {
	title := page.Title()
	body := string(page.Body())

	titleTokens := Tokenize(title)
	var tagTokens []string
	var normTags []string
	for _, tag := range page.Tags() {
		if norm := kb.Normalize(tag); norm != "" {
			normTags = append(normTags, norm)
		}
		tagTokens = append(tagTokens, Tokenize(tag)...)
	}
	bodyTokens := Tokenize(body)

	doc := Doc{Title: titleTokens, Tags: tagTokens, Body: bodyTokens}
	all := make([]string, 0, len(titleTokens)+len(tagTokens)+len(bodyTokens))
	all = append(all, titleTokens...)
	all = append(all, tagTokens...)
	all = append(all, bodyTokens...)

	res, err := tx.Exec(
		`INSERT INTO pages(path, title, type, status, body, hash, length)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		path, title, page.Type(), page.Status(), body, kb.HashOf(page.Bytes()), doc.Length())
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO pages_fts(rowid, tokens) VALUES (?, ?)`, id, Join(all)); err != nil {
		return err
	}

	if err := insertNames(tx, path, page); err != nil {
		return err
	}
	for i, link := range page.Links() {
		if _, err := tx.Exec(
			`INSERT INTO links(from_path, ordinal, target, name, line) VALUES (?, ?, ?, ?, ?)`,
			path, i, link.Target, link.Name, link.Line); err != nil {
			return err
		}
	}
	for i, c := range page.Citations() {
		if _, err := tx.Exec(
			`INSERT INTO citations(path, ordinal, key, line) VALUES (?, ?, ?, ?)`,
			path, i, c.Key, c.Line); err != nil {
			return err
		}
	}
	for _, tag := range normTags {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO tags(path, tag) VALUES (?, ?)`, path, tag); err != nil {
			return err
		}
	}
	return nil
}

// insertNames records the names a page answers to for resolution and backlinks.
//
// A source page answers to none, which is what keeps a virtual source page from
// colliding with a page whose title happens to equal a citation key. The rule
// matches the one the in-memory graph applies, and it has to: an index that
// resolved a name the fallback does not would be authoritative, not a cache.
func insertNames(tx *sql.Tx, path string, page *kb.Page) error {
	if page.Type() == kb.TypeSource {
		return nil
	}
	add := func(raw, kind string) error {
		name := kb.Normalize(raw)
		if name == "" {
			return nil
		}
		_, err := tx.Exec(
			`INSERT OR IGNORE INTO names(path, name, kind) VALUES (?, ?, ?)`,
			path, name, kind)
		return err
	}
	if err := add(page.Title(), "title"); err != nil {
		return err
	}
	for _, alias := range page.Aliases() {
		if err := add(alias, "alias"); err != nil {
			return err
		}
	}
	return nil
}

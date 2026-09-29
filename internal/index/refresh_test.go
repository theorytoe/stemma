package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/theorytoe/stemma/internal/kb"
)

// The hash a build stamps must be the hash of the file on disk, or a fresh
// index would look stale and every query would fall back.
func TestKBHashesMatchFileHashes(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/crlf.md":       "---\r\ntitle: CRLF\r\ntype: concept\r\n---\r\nbody\r\n",
		"pages/no-newline.md": "---\ntitle: No Newline\ntype: concept\n---\nbody",
		"pages/unicode.md":    "---\ntitle: Ünicode\ntype: concept\ntags: [naïve]\n---\ncafé\n",
	})
	fromKB := KBHashes(k)
	fromDisk, err := kb.Hashes(k.Root)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromKB) != len(fromDisk) {
		t.Fatalf("KBHashes has %d pages, Hashes has %d", len(fromKB), len(fromDisk))
	}
	for path, hash := range fromDisk {
		if fromKB[path] != hash {
			t.Errorf("%s: parsed hash %q, file hash %q", path, fromKB[path], hash)
		}
	}
}

func TestCheckReportsAbsentStaleFresh(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/a.md": pageFile("A", "type: concept", "alpha\n"),
	})
	if st, err := Check(k.Root); err != nil || st != Absent {
		t.Fatalf("before building: %v (err %v), want absent", st, err)
	}

	s := open(t, k.Root)
	if _, err := s.Populate(k); err != nil {
		t.Fatal(err)
	}
	if st, err := Check(k.Root); err != nil || st != Fresh {
		t.Errorf("after building: %v (err %v), want fresh", st, err)
	}
	if st, err := CheckKB(k); err != nil || st != Fresh {
		t.Errorf("CheckKB after building: %v (err %v), want fresh", st, err)
	}

	writeFile(t, k.Root, "pages/a.md", pageFile("A", "type: concept", "beta\n"))
	if st, err := Check(k.Root); err != nil || st != Stale {
		t.Errorf("after an edit: %v (err %v), want stale", st, err)
	}
}

func TestRefreshRewritesOnlyWhatChanged(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/index.md": pageFile("Index", "type: index", "[[A]] [[B]]\n"),
		"pages/a.md":     pageFile("A", "type: concept", "alpha\n"),
		"pages/b.md":     pageFile("B", "type: concept", "beta\n"),
	})
	s := open(t, k.Root)
	if _, err := s.Populate(k); err != nil {
		t.Fatal(err)
	}

	// Nothing changed: a refresh writes nothing.
	if rep, err := s.Refresh(k); err != nil {
		t.Fatal(err)
	} else if rep.Added != 0 || rep.Updated != 0 || rep.Removed != 0 {
		t.Errorf("a no-op refresh reported %+v", rep)
	}

	// Touching a file without changing it must not count as a change: the
	// stamp is content, not a timestamp.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(k.Root, "pages", "a.md"), future, future); err != nil {
		t.Fatal(err)
	}
	if st, err := Check(k.Root); err != nil || st != Fresh {
		t.Errorf("a touched but unchanged page made the index %v (err %v)", st, err)
	}

	// Change one page, add another.
	writeFile(t, k.Root, "pages/a.md", pageFile("A", "type: concept", "gamma\n"))
	writeFile(t, k.Root, "pages/c.md", pageFile("C", "type: concept", "delta\n"))
	k2 := reload(t, k.Root)

	rep, err := s.Refresh(k2)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Added != 1 || rep.Updated != 1 || rep.Removed != 0 {
		t.Errorf("refresh = %+v, want 1 added, 1 updated", rep)
	}
	if got := queryString(t, s.db, findMatch, "gamma"); got != "pages/a.md" {
		t.Errorf("the new body is not searchable: %q", got)
	}
	if n := count(t, s, `SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'alpha'`); n != 0 {
		t.Errorf("the old body is still searchable: %d matches", n)
	}
	if got := queryString(t, s.db, findMatch, "delta"); got != "pages/c.md" {
		t.Errorf("the added page is not searchable: %q", got)
	}

	// Remove a page.
	if err := os.Remove(filepath.Join(k.Root, "pages", "b.md")); err != nil {
		t.Fatal(err)
	}
	k3 := reload(t, k.Root)
	rep, err = s.Refresh(k3)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Removed != 1 || rep.Added != 0 || rep.Updated != 0 {
		t.Errorf("refresh after a removal = %+v, want 1 removed", rep)
	}
	if n := count(t, s, `SELECT count(*) FROM pages WHERE path = 'pages/b.md'`); n != 0 {
		t.Errorf("the removed page is still in the index")
	}
	if n := count(t, s, `SELECT count(*) FROM pages_fts WHERE pages_fts MATCH 'beta'`); n != 0 {
		t.Errorf("the removed page is still searchable: %d matches", n)
	}
	if n := count(t, s, `SELECT count(*) FROM names WHERE path = 'pages/b.md'`); n != 0 {
		t.Errorf("the removed page still claims a name")
	}

	if st, err := Check(k3.Root); err != nil || st != Fresh {
		t.Errorf("after a refresh: %v (err %v), want fresh", st, err)
	}
}

// reload reads the KB again after its files changed.
func reload(t *testing.T, root string) *kb.KB {
	t.Helper()
	k, err := kb.Load(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	return k
}

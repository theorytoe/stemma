package build

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeKB lays down a small KB on disk. A build reads files, so its fixture has
// to be files.
func writeKB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "stemma.toml", "title = \"Test KB\"\n")
	writeFile(t, dir, "pages/index.md", "---\ntitle: Home\ntype: index\n---\nStart at [[Alpha]].\n")
	writeFile(t, dir, "pages/alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nAlpha links [[Beta]].\n")
	writeFile(t, dir, "pages/beta.md", "---\ntitle: Beta\ntype: concept\n---\nBeta.\n")
	return dir
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil
}

func siteDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "site")
}

// What a build produces is the site, not a subset of it, and every URL it
// reports is a file that is actually there.
func TestWriteWritesTheWholeSite(t *testing.T) {
	dir := siteDir(t)
	res, err := Write(writeKB(t), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"index.html", "all.html", "types.html", "tags.html", "graph.html",
		"alpha.html", "beta.html", "types/concept.html", "types/index.html",
		"assets/style.css", "assets/theme.js", "assets/graph.js",
	} {
		if !exists(t, dir, name) {
			t.Errorf("%s was not written", name)
		}
	}
	for _, u := range res.Written {
		if !exists(t, dir, u) {
			t.Errorf("the result names %s but nothing is on disk for it", u)
		}
	}
}

// A build reports the page under its URL, but writes it under the name a server
// looks for once it has decoded the request. Writing the escaped name would
// leave a file no request could match.
func TestWriteDecodesURLs(t *testing.T) {
	root := writeKB(t)
	writeFile(t, root, "pages/two words.md", "---\ntitle: Two Words\ntype: note\n---\nBody.\n")

	dir := siteDir(t)
	res, err := Write(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(t, dir, "two words.html") {
		t.Error("the page was not written under its decoded name")
	}
	if exists(t, dir, "two%20words.html") {
		t.Error("the page was written under its escaped URL")
	}
	if !named(res.Written, "two%20words.html") {
		t.Errorf("the result does not name the page by its URL: %v", res.Written)
	}
}

// Nothing written points at an absolute path, so the directory can be moved,
// copied, or opened straight from disk and every link still resolves.
func TestWriteUsesNoAbsolutePaths(t *testing.T) {
	dir := siteDir(t)
	if _, err := Write(writeKB(t), dir); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, bad := range []string{`href="/`, `src="/`} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s carries the absolute address %s", p, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A rebuild removes what an earlier build wrote and the site no longer has, so
// a renamed page is not still reachable at its old address.
func TestWriteRemovesWhatTheSiteNoLongerHas(t *testing.T) {
	root := writeKB(t)
	dir := siteDir(t)
	if _, err := Write(root, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, "pages/beta.md"), filepath.Join(root, "pages/gamma.md")); err != nil {
		t.Fatal(err)
	}

	res, err := Write(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if exists(t, dir, "beta.html") {
		t.Error("beta.html survived a build that no longer has Beta")
	}
	if !exists(t, dir, "gamma.html") {
		t.Error("gamma.html was not written")
	}
	if len(res.Removed) != 1 || res.Removed[0] != "beta.html" {
		t.Errorf("Removed = %v, want [beta.html]", res.Removed)
	}
}

// A directory that held only a page that moved is removed with the page, so the
// output is the site and not the site plus its history.
func TestWriteRemovesDirectoriesItEmptied(t *testing.T) {
	root := writeKB(t)
	writeFile(t, root, "pages/notes/one.md", "---\ntitle: One\ntype: note\n---\nBody.\n")

	dir := siteDir(t)
	if _, err := Write(root, dir); err != nil {
		t.Fatal(err)
	}
	if !exists(t, dir, "notes/one.html") {
		t.Fatal("notes/one.html was not written")
	}
	if err := os.Rename(filepath.Join(root, "pages/notes/one.md"), filepath.Join(root, "pages/one.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes")); !os.IsNotExist(err) {
		t.Error("the directory a moved page left behind is still there")
	}
}

// The build removes only what it wrote, so a file the author left in the output
// directory is not a casualty of a rebuild (P4).
func TestWriteLeavesFilesItDidNotWrite(t *testing.T) {
	root := writeKB(t)
	dir := siteDir(t)
	if _, err := Write(root, dir); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(dir, "CNAME")
	if err := os.WriteFile(mine, []byte("example.org\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, dir); err != nil {
		t.Fatal(err)
	}
	if !exists(t, dir, "CNAME") {
		t.Error("a rebuild removed a file it did not write")
	}
}

// A second build of an unchanged KB removes nothing, which is what makes the
// removal above worth having.
func TestWriteIsIdempotent(t *testing.T) {
	root := writeKB(t)
	dir := siteDir(t)
	if _, err := Write(root, dir); err != nil {
		t.Fatal(err)
	}
	res, err := Write(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 {
		t.Errorf("a rebuild of an unchanged KB removed %v", res.Removed)
	}
}

func named(urls []string, want string) bool {
	for _, u := range urls {
		if u == want {
			return true
		}
	}
	return false
}

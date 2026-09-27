package kb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree lays out a KB on disk and returns its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func pageSource(title, extra, body string) string {
	s := "---\ntitle: " + title + "\n"
	if extra != "" {
		s += extra + "\n"
	}
	return s + "---\n" + body
}

func loadTree(t *testing.T, files map[string]string) *KB {
	t.Helper()
	k, err := Load(writeTree(t, files))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return k
}

func TestLoadReadsPagesAndManifest(t *testing.T) {
	k := loadTree(t, map[string]string{
		"stemma.toml":       "title = \"Demo\"\ntypes = [\"person\"]\n",
		"pages/a.md":        pageSource("A", "type: concept", "sees [[B]]\n"),
		"pages/b.md":        pageSource("B", "type: person", ""),
		"bibliography.bib":  "@article{key,}\n",
		"inbox/draft.md":    "not a page at all\n",
		"notes/whatever.md": "outside pages, so not a page\n",
		"pages/ignored.txt": "not markdown\n",
	})

	if k.Manifest.Title != "Demo" {
		t.Errorf("Title = %q", k.Manifest.Title)
	}
	if !k.Vocabulary.Has("person") {
		t.Error("the manifest's types were not applied")
	}
	if got := k.Graph.Paths(); len(got) != 2 {
		t.Errorf("pages = %q, want two", got)
	}
	if got := k.Bibliography.Keys(); len(got) != 1 {
		t.Errorf("bibliography keys = %q", got)
	}
}

func TestLintOnACleanKB(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md":   pageSource("Index", "type: index", "start at [[A]]\n"),
		"pages/a.md":       pageSource("A", "type: concept", "cites [@key] and links [[B]]\n"),
		"pages/b.md":       pageSource("B", "type: concept", ""),
		"bibliography.bib": "@article{key,}\n",
	})
	if got := k.Lint(Strict); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

// A draft is not part of the knowledge, so nothing about one is reported: not
// its missing frontmatter, not its broken links.
func TestLintDoesNotReadTheInbox(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md": pageSource("Index", "type: index", ""),
		"inbox/draft.md": "[[]] broken [[Nothing]] and no frontmatter\n",
	})
	if got := k.Lint(Strict); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestLintSkipsIgnoredPaths(t *testing.T) {
	k := loadTree(t, map[string]string{
		"stemma.toml":      "ignore = [\"pages/attic/**\"]\n",
		"pages/index.md":   pageSource("Index", "type: index", ""),
		"pages/attic/a.md": pageSource("A", "type: concept", "broken [[Nothing]]\n"),
	})
	if got := k.Graph.Paths(); len(got) != 1 {
		t.Fatalf("pages = %q, want only the index", got)
	}
	if got := k.Lint(Strict); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestLintFindsEverything(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/a.md":       pageSource("A", "type: nonsense\nstatus: draft", "broken [[Nothing]] and [@absent]\n"),
		"bibliography.bib": "@article{unused,}\n@book{dup,}\n@misc{dup,}\n",
	})

	codes := map[string]bool{}
	for _, f := range k.Lint(Lenient) {
		codes[f.Code] = true
	}
	for _, want := range []string{
		CodeUnknownType,
		CodeInvalidStatus,
		CodeUnresolvedLink,
		CodeCitationMissing,
		CodeCitationUncited,
		CodeCitationDuplicate,
		CodeOrphanPage,
	} {
		if !codes[want] {
			t.Errorf("no %s finding in %v", want, sortedKeys(codes))
		}
	}
}

func TestLintFindsAnAmbiguousLink(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/a.md": pageSource("Go", "type: concept", ""),
		"pages/b.md": pageSource("Golang", "type: concept\naliases:\n  - go", ""),
		"pages/c.md": pageSource("C", "type: concept", "see [[go]]\n"),
	})

	for _, mode := range []Mode{Lenient, Strict} {
		got := withCode(k.Lint(mode), CodeAmbiguousLink)
		if len(got) != 1 {
			t.Fatalf("mode %v: findings = %+v", mode, got)
		}
		if got[0].Severity != Error {
			t.Errorf("mode %v: severity = %q, want %q", mode, got[0].Severity, Error)
		}
	}
}

func TestLintAttributesFindingsToTheirPath(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md": pageSource("Index", "type: index", ""),
		"pages/a.md":     pageSource("A", "type: concept", "broken [[Nothing]]\n"),
	})
	got := withCode(k.Lint(Lenient), CodeUnresolvedLink)
	if len(got) != 1 {
		t.Fatalf("findings = %+v", got)
	}
	if got[0].Path != "pages/a.md" {
		t.Errorf("path = %q", got[0].Path)
	}
}

// The bibliography may be one file or a directory of them, and a finding about
// a key points at the file that actually defines it.
func TestBibliographyDirectoryForm(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md":         pageSource("Index", "type: index", "cites [@one]\n"),
		"bibliography/one.bib":   "@article{one,}\n@book{two,}\n",
		"bibliography/two.bib":   "@article{three,}\n",
		"bibliography/notes.txt": "not a bibliography\n",
	})

	if got := k.Bibliography.Len(); got != 3 {
		t.Errorf("keys = %d, want 3", got)
	}
	if got := k.Bibliography.PathOf("three"); got != "bibliography/two.bib" {
		t.Errorf("PathOf(three) = %q", got)
	}

	uncited := withCode(k.Lint(Lenient), CodeCitationUncited)
	if len(uncited) != 2 {
		t.Fatalf("uncited = %+v", uncited)
	}
	if uncited[0].Path != "bibliography/one.bib" {
		t.Errorf("uncited finding path = %q, want the file defining it", uncited[0].Path)
	}
}

func TestBibliographyBothFormsAtOnce(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md":       pageSource("Index", "type: index", ""),
		"bibliography.bib":     "@article{one,}\n",
		"bibliography/two.bib": "@article{two,}\n",
	})
	if got := k.Bibliography.Len(); got != 2 {
		t.Errorf("keys = %d, want both files read", got)
	}
}

func TestABibliographyKeyDefinedInTwoFilesIsADuplicate(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/index.md":       pageSource("Index", "type: index", ""),
		"bibliography.bib":     "@article{one,}\n",
		"bibliography/two.bib": "@article{one,}\n",
	})
	got := withCode(k.Lint(Lenient), CodeCitationDuplicate)
	if len(got) != 1 {
		t.Fatalf("duplicate findings = %+v", got)
	}
}

func TestLoadRefusesAFileItCannotRead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"unclosed frontmatter", map[string]string{"pages/a.md": "---\ntitle: A\n"}},
		{"not utf-8", map[string]string{"pages/a.md": "---\ntitle: \xff\xfe\n---\n"}},
		{"frontmatter is not a mapping", map[string]string{"pages/a.md": "---\n- a\n- b\n---\n"}},
		{"duplicate frontmatter key", map[string]string{"pages/a.md": "---\ntitle: A\ntitle: B\n---\n"}},
		{"unclosed bibliography entry", map[string]string{"bibliography.bib": "@article{k, title = {x}\n"}},
		{"bibliography is not utf-8", map[string]string{"bibliography.bib": "@article{k\xff,}\n"}},
		{"broken manifest", map[string]string{"stemma.toml": "title = \n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(writeTree(t, tc.files)); err == nil {
				t.Error("Load accepted a file it cannot read")
			}
		})
	}
}

// When several files are unreadable, all of them are named at once. Fixing a KB
// should not take one run per mistake.
func TestLoadReportsEveryUnreadableFile(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/a.md": "---\ntitle: A\n",
		"pages/b.md": "---\ntitle: B\n",
	})
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load accepted unreadable pages")
	}
	for _, want := range []string{"pages/a.md", "pages/b.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %s: %v", want, err)
		}
	}
}

func TestLoadRefusesADirectoryThatIsNotAKB(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root); err == nil {
		t.Error("Load accepted an empty directory as a KB root")
	}
}

// A manifest alone, or pages alone, is enough to be a KB root.
func TestLoadAcceptsAKBWithOnlyOneOfManifestAndPages(t *testing.T) {
	if _, err := Load(writeTree(t, map[string]string{"stemma.toml": "title = \"X\"\n"})); err != nil {
		t.Errorf("a manifest alone was refused: %v", err)
	}
	if _, err := Load(writeTree(t, map[string]string{"pages/a.md": pageSource("A", "type: concept", "")})); err != nil {
		t.Errorf("pages alone were refused: %v", err)
	}
}

func TestLintResultIsSorted(t *testing.T) {
	k := loadTree(t, map[string]string{
		"pages/a.md":     pageSource("A", "type: concept", "one [[X]]\n"),
		"pages/b.md":     pageSource("B", "type: concept", "\ntwo [[Y]]\n"),
		"pages/index.md": pageSource("Index", "type: index", ""),
	})
	var lastPath string
	for _, f := range k.Lint(Lenient) {
		if f.Path < lastPath {
			t.Fatalf("findings are not sorted: %+v", k.Lint(Lenient))
		}
		lastPath = f.Path
	}
}

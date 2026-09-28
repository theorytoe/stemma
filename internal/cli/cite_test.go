package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/source"
)

// withResolver replaces the resolver `cite add` builds, so a resolution test
// runs against a local server instead of the network.
func withResolver(t *testing.T, res *source.Resolver) {
	t.Helper()
	old := newResolver
	newResolver = func() *source.Resolver { return res }
	t.Cleanup(func() { newResolver = old })
}

func sourceResolver(t *testing.T, h http.Handler) *source.Resolver {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	r := source.New()
	r.Endpoints = source.Endpoints{DOI: srv.URL, ArXiv: srv.URL, OpenLibrary: srv.URL}
	r.Now = func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }
	return r
}

// kbWithPages is a KB with a page and no bibliography, so a test can watch one
// be created.
func kbWithPages(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
	})
}

func readBib(t *testing.T, root, name string) *kb.BibFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := kb.ParseBibFile(name, raw)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCiteAddByHandCreatesTheBibliography(t *testing.T) {
	root := kbWithPages(t)
	code, stdout, stderr := run("cite", "add", "--kb", root,
		"--title", "A Study of Things", "--author", "Doe, Jane", "--year", "2019")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s%s", code, stdout, stderr)
	}

	f := readBib(t, root, "bibliography.bib")
	if f.Len() != 1 {
		t.Fatalf("entries = %d, want 1", f.Len())
	}
	e, ok := f.Entry("doe2019study")
	if !ok {
		t.Fatalf("entries = %v, want doe2019study", f.Entries())
	}
	if got, _ := e.Value("title"); got != "A Study of Things" {
		t.Errorf("title = %q", got)
	}
	if got, _ := e.Value("author"); got != "Doe, Jane" {
		t.Errorf("author = %q", got)
	}
	// A record entered by hand carries no retrieval date, because nothing was
	// retrieved.
	if _, ok := e.Raw(kb.FieldRetrieved); ok {
		t.Error("a hand-entered record carries a retrieval date")
	}
}

func TestCiteAddAppendsToAnExistingBibliography(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"bibliography.bib": "% The bibliography.\n\n@article{old,}\n",
	})
	code, _, stderr := run("cite", "add", "--kb", root,
		"--title", "A New Work", "--author", "Roe, John", "--year", "2020")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	f := readBib(t, root, "bibliography.bib")
	if f.Len() != 2 || !f.Has("old") {
		t.Errorf("entries = %d, has old = %v", f.Len(), f.Has("old"))
	}
	// The comment that was already there is still there.
	if raw, _ := os.ReadFile(filepath.Join(root, "bibliography.bib")); !strings.Contains(string(raw), "% The bibliography.") {
		t.Error("the existing comment was lost")
	}
}

func TestCiteAddUpdatesTheSameKey(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"bibliography.bib": "@article{key, custom = {keep}}\n",
	})
	code, _, stderr := run("cite", "add", "--kb", root, "--key", "key",
		"--title", "Changed", "--year", "2020")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	f := readBib(t, root, "bibliography.bib")
	if f.Len() != 1 {
		t.Fatalf("entries = %d, want 1", f.Len())
	}
	e, _ := f.Entry("key")
	if got, _ := e.Value("title"); got != "Changed" {
		t.Errorf("title = %q", got)
	}
	if got, _ := e.Value("custom"); got != "keep" {
		t.Errorf("custom = %q, want the unknown field kept", got)
	}
}

func TestCiteAddRecognisesTheSameWorkUnderAFreshKey(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"bibliography.bib": "@article{old, doi = {10.1145/x}, title = {Original}}\n",
	})
	code, _, stderr := run("cite", "add", "--kb", root,
		"--doi", "10.1145/x", "--title", "Renamed", "--year", "2020")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	f := readBib(t, root, "bibliography.bib")
	if f.Len() != 1 {
		t.Fatalf("entries = %d, want the existing one updated rather than a second copy", f.Len())
	}
	e, ok := f.Entry("old")
	if !ok {
		t.Fatal("the original key was not kept")
	}
	if got, _ := e.Value("title"); got != "Renamed" {
		t.Errorf("title = %q", got)
	}
}

func TestCiteAddForceAppendsASecondCopy(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"bibliography.bib": "@article{old, doi = {10.1145/x}}\n",
	})
	code, _, stderr := run("cite", "add", "--kb", root, "--force",
		"--doi", "10.1145/x", "--title", "A Deliberate Second Copy")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if f := readBib(t, root, "bibliography.bib"); f.Len() != 2 {
		t.Errorf("entries = %d, want 2", f.Len())
	}
}

func TestCiteAddDryRunWritesNothing(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"bibliography.bib": "@article{old,}\n",
	})
	before, err := os.ReadFile(filepath.Join(root, "bibliography.bib"))
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run("cite", "add", "--kb", root, "--dry-run", "--title", "Preview")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "would add") {
		t.Errorf("stdout = %q, want it to say what it would do", stdout)
	}
	after, err := os.ReadFile(filepath.Join(root, "bibliography.bib"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("--dry-run changed the bibliography:\n%s", after)
	}
}

func TestCiteAddJSON(t *testing.T) {
	root := kbWithPages(t)
	code, stdout, stderr := run("cite", "add", "--kb", root, "--json",
		"--title", "A Study", "--author", "Doe, Jane", "--year", "2019")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var got citeAddReport
	e := decodeData(t, stdout, &got)
	if e.Command != "cite add" {
		t.Errorf("command = %q, want %q", e.Command, "cite add")
	}
	if got.Action != "added" || got.Key != "doe2019study" || got.Path != "bibliography.bib" {
		t.Errorf("report = %+v", got)
	}
	if !strings.Contains(got.Entry, "A Study") {
		t.Errorf("entry = %q", got.Entry)
	}
}

func TestCiteAddNeedsSomethingToAdd(t *testing.T) {
	root := kbWithPages(t)
	if code, _, _ := run("cite", "add", "--kb", root); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
}

func TestCiteAddRefusesAnIdentifierAndFieldsTogether(t *testing.T) {
	root := kbWithPages(t)
	code, _, stderr := run("cite", "add", "--kb", root, "10.1145/x", "--title", "Both")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "identifier") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestCiteAddResolvesAnIdentifier(t *testing.T) {
	const record = `@article{Chen_2020,
  title = {Attention in Practice},
  doi = {10.1145/3375637},
}
`
	withResolver(t, sourceResolver(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(record))
	})))

	root := kbWithPages(t)
	code, _, stderr := run("cite", "add", "--kb", root, "10.1145/3375637")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	f := readBib(t, root, "bibliography.bib")
	e, ok := f.Entry("Chen_2020")
	if !ok {
		t.Fatal("the resolved entry is missing")
	}
	if got, _ := e.Raw(kb.FieldRetrieved); got != "{2026-09-28}" {
		t.Errorf("retrieved = %q", got)
	}
	if got, _ := e.Raw(kb.FieldContentHash); !strings.HasPrefix(got, "{sha256:") {
		t.Errorf("content hash = %q", got)
	}
}

func TestCiteAddOfflineRefusesTheNetwork(t *testing.T) {
	root := kbWithPages(t)
	code, _, stderr := run("cite", "add", "--kb", root, "--offline", "10.1145/3375637")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "offline") {
		t.Errorf("stderr = %q", stderr)
	}
	if _, err := os.Lstat(filepath.Join(root, "bibliography.bib")); !os.IsNotExist(err) {
		t.Error("an offline run wrote a bibliography")
	}
}

func TestCiteAddAnUnresolvedIdentifierIsAFinding(t *testing.T) {
	withResolver(t, sourceResolver(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})))

	root := kbWithPages(t)
	code, stdout, _ := run("cite", "add", "--kb", root, "--json", "10.1145/0000000")
	if code != ExitFindings {
		t.Fatalf("exit = %d, want %d", code, ExitFindings)
	}
	e := decodeJSON(t, stdout)
	if len(e.Findings) != 1 || e.Findings[0].Code != CodeCiteUnresolved {
		t.Errorf("findings = %+v", e.Findings)
	}
	if _, err := os.Lstat(filepath.Join(root, "bibliography.bib")); !os.IsNotExist(err) {
		t.Error("a failed resolution wrote a bibliography")
	}
}

package serve

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeKB lays down a tiny KB on disk, because the server reads files and
// reloads when they change; an in-memory KB could not show that.
func writeKB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "stemma.toml", "title = \"Test KB\"\n")
	writeFile(t, dir, "pages/index.md", "---\ntitle: Home\ntype: index\n---\nStart at [[Alpha]].\n")
	writeFile(t, dir, "pages/alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nAlpha is about retrieval. See [[Beta]].\n")
	writeFile(t, dir, "pages/beta.md", "---\ntitle: Beta\ntype: concept\n---\nBeta mentions retrieval too.\n")
	return dir
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, url string) (status int, contentType, body string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(b)
}

func TestServesPagesAndAssets(t *testing.T) {
	srv, err := New(writeKB(t))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	status, ctype, body := get(t, ts.URL+"/")
	if status != http.StatusOK || !strings.HasPrefix(ctype, "text/html") {
		t.Fatalf("GET / = %d %s", status, ctype)
	}
	if !strings.Contains(body, "<h1>Home</h1>") {
		t.Errorf("the home page is not the index page:\n%s", body)
	}
	if !strings.Contains(body, `class="search"`) {
		t.Errorf("a served page has no search form:\n%s", body)
	}
	if !strings.Contains(body, "__reload") {
		t.Errorf("a served page has no reload helper:\n%s", body)
	}

	if status, _, _ := get(t, ts.URL+"/alpha.html"); status != http.StatusOK {
		t.Errorf("GET /alpha.html = %d", status)
	}
	for _, u := range []string{"/all.html", "/types.html", "/tags.html"} {
		if status, _, _ := get(t, ts.URL+u); status != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", u, status)
		}
	}
	if status, ctype, _ := get(t, ts.URL+"/assets/style.css"); status != http.StatusOK || !strings.HasPrefix(ctype, "text/css") {
		t.Errorf("GET /assets/style.css = %d %s", status, ctype)
	}
	if status, _, _ := get(t, ts.URL+"/missing.html"); status != http.StatusNotFound {
		t.Errorf("GET a missing page = %d, want 404", status)
	}
}

// TestServerSideSearch is the P9 requirement for search: the form is answered
// on the server, so results arrive without any JavaScript running.
func TestServerSideSearch(t *testing.T) {
	srv, err := New(writeKB(t))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	status, ctype, body := get(t, ts.URL+"/search?q=retrieval")
	if status != http.StatusOK || !strings.HasPrefix(ctype, "text/html") {
		t.Fatalf("GET /search = %d %s", status, ctype)
	}
	for _, want := range []string{"Alpha", "Beta", "<mark>retrieval</mark>"} {
		if !strings.Contains(body, want) {
			t.Errorf("the results are missing %q:\n%s", want, body)
		}
	}

	if _, _, empty := get(t, ts.URL+"/search"); strings.Contains(empty, "<mark>") {
		t.Errorf("an empty query showed results:\n%s", empty)
	}

	// A body carries its own brackets, as every wikilink does, and they must
	// not be mistaken for the snippet's marks.
	_, _, beta := get(t, ts.URL+"/search?q=beta")
	if strings.Contains(beta, "<mark>[") {
		t.Errorf("a literal bracket was read as a snippet mark:\n%s", beta)
	}
	if !strings.Contains(beta, "<mark>Beta</mark>") {
		t.Errorf("the matched word inside a wikilink was not marked:\n%s", beta)
	}
}

// TestRefreshPicksUpEdits checks live reload's two halves: the version the
// browser polls changes, and the new page is served without a restart.
func TestRefreshPicksUpEdits(t *testing.T) {
	dir := writeKB(t)
	srv, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	if status, _, _ := get(t, ts.URL+"/gamma.html"); status != http.StatusNotFound {
		t.Fatalf("gamma.html exists before it is written")
	}
	_, _, before := get(t, ts.URL+"/__reload")

	writeFile(t, dir, "pages/gamma.md", "---\ntitle: Gamma\ntype: note\n---\nA new page.\n")

	_, _, after := get(t, ts.URL+"/__reload")
	if before == after {
		t.Errorf("the version did not change after an edit: %s", before)
	}
	if status, _, body := get(t, ts.URL+"/gamma.html"); status != http.StatusOK || !strings.Contains(body, "<h1>Gamma</h1>") {
		t.Errorf("the new page is not served: %d\n%s", status, body)
	}
}

// TestVersionIgnoresTouches checks the version is content-derived: rewriting a
// file with the same bytes does not trigger a reload.
func TestVersionIgnoresTouches(t *testing.T) {
	dir := writeKB(t)
	srv, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	_, _, before := get(t, ts.URL+"/__reload")
	writeFile(t, dir, "pages/alpha.md", "---\ntitle: Alpha\ntype: concept\n---\nAlpha is about retrieval. See [[Beta]].\n")
	_, _, after := get(t, ts.URL+"/__reload")
	if before != after {
		t.Errorf("the version changed though no content did: %s -> %s", before, after)
	}
}

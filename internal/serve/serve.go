// Package serve answers HTTP requests for a rendered KB.
//
// It is the local entry point of the renderer. The documents that build writes
// are the ones served here, plus two things that exist only while a server
// runs: a search page, answered on the server so it needs no JavaScript, and a
// reload helper that lets a browser reload itself when a page changes.
package serve

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/theorytoe/stemma/internal/index"
	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/render"
)

// Server serves one KB as a website.
type Server struct {
	root string

	mu       sync.RWMutex
	version  string
	docs     map[string]render.Document
	renderer *render.Renderer
	kb       *kb.KB
}

// New returns a server for the KB at root, rendered and ready to answer.
func New(root string) (*Server, error) {
	s := &Server{root: root}
	if err := s.refresh(); err != nil {
		return nil, err
	}
	return s, nil
}

// refresh reloads the KB when its files changed, so an edit shows up on the
// next request without restarting the server. When nothing changed it returns
// after only the freshness check, which is why a page is not re-rendered per
// request.
func (s *Server) refresh() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hashes, err := kb.Hashes(s.root)
	if err != nil {
		return err
	}
	version := versionOf(hashes)
	if s.docs != nil && version == s.version {
		return nil
	}

	k, err := kb.Load(s.root)
	if err != nil {
		return err
	}
	r, err := render.New(k)
	if err != nil {
		return err
	}
	r.EnableSearch()
	docs, err := r.Documents()
	if err != nil {
		return err
	}
	// Keyed by the decoded URL, because that is what a request carries: net/http
	// hands over URL.Path already unescaped, so a request for
	// "two%20words.html" arrives as "two words.html". Keying by the URL as
	// written would 404 exactly the pages whose names have to be escaped.
	byPath := make(map[string]render.Document, len(docs))
	for _, d := range docs {
		p, err := render.FilePath(d.URL)
		if err != nil {
			return err
		}
		byPath[p] = d
	}
	s.kb, s.renderer, s.docs, s.version = k, r, byPath, version
	return nil
}

// ServeHTTP answers one request from the current rendering of the KB.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := s.refresh(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch {
	case r.URL.Path == "/__reload":
		s.reload(w)
	case r.URL.Path == "/search":
		s.search(w, r)
	default:
		s.document(w, r)
	}
}

// document serves one rendered file. "/" is the home page.
func (s *Server) document(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}

	s.mu.RLock()
	doc, ok := s.docs[name]
	version := s.version
	s.mu.RUnlock()

	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", doc.Type)
	if strings.HasPrefix(doc.Type, "text/html") {
		_, _ = w.Write(injectReload(doc.Body, version))
		return
	}
	_, _ = w.Write(doc.Body)
}

// reload is what the page's helper polls: the current version, which changes
// exactly when the KB does.
func (s *Server) reload(w http.ResponseWriter) {
	s.mu.RLock()
	version := s.version
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, version)
}

// search answers a query with the same ranked search the command uses. It runs
// on the server, so the form works with JavaScript off.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	var hits []render.SearchHit
	if query != "" {
		s.mu.RLock()
		k := s.kb
		s.mu.RUnlock()

		terms := index.Tokenize(query)

		src := index.NewSource(k, nil)
		defer src.Close()
		found, err := index.Search(src, nil, index.SearchRequest{Query: query})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, h := range found {
			title := h.Title
			if title == "" {
				title = h.Path
			}
			hits = append(hits, render.SearchHit{
				Title:   title,
				URL:     render.PageURL(h.Path),
				Snippet: markSnippet(h.Snippet, terms),
			})
		}
	}

	s.mu.RLock()
	renderer, version := s.renderer, s.version
	s.mu.RUnlock()

	page, err := renderer.Search(query, hits)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(injectReload(page, version))
}

// markSnippet turns the "[term]" the index puts around a matched word into
// <mark>, escaping everything else so a result is safe to put in a page.
//
// Which brackets are marks is decided by the index, which knows the query's
// terms; this only renders the decision.
func markSnippet(s string, terms []string) template.HTML {
	var b strings.Builder
	for _, part := range index.SplitSnippet(s, terms) {
		if part.Match {
			b.WriteString("<mark>")
			b.WriteString(html.EscapeString(part.Text))
			b.WriteString("</mark>")
			continue
		}
		b.WriteString(html.EscapeString(part.Text))
	}
	return template.HTML(b.String())
}

// versionOf is a short digest of every page's content hash. It changes when the
// KB changes and not when a file is merely touched, which is what the reload
// helper polls.
func versionOf(hashes map[string]string) string {
	names := make([]string, 0, len(hashes))
	for name := range hashes {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		fmt.Fprintf(h, "%s\x00%s\n", name, hashes[name])
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// reloadScript polls the server and reloads the tab when the KB has changed.
// It is polish: every page reads without it, and it is the one script in the
// project, and only ever served, never written by build.
const reloadScript = `<script>
(function () {
  var version = %q;
  function check() {
    fetch("/__reload", { cache: "no-store" })
      .then(function (r) { return r.text(); })
      .then(function (v) { if (v !== version) { location.reload(); } })
      .catch(function () {});
  }
  setInterval(check, 1000);
})();
</script>
`

// injectReload puts the reload script just before </body>.
func injectReload(body []byte, version string) []byte {
	script := []byte(fmt.Sprintf(reloadScript, version))
	i := bytes.LastIndex(body, []byte("</body>"))
	if i < 0 {
		return append(body, script...)
	}
	out := make([]byte, 0, len(body)+len(script))
	out = append(out, body[:i]...)
	out = append(out, script...)
	out = append(out, body[i:]...)
	return out
}

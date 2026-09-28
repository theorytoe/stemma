package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/theorytoe/stemma/internal/kb"
)

// testServer points every endpoint at one local server and takes the clock and
// the sleeping out of the run, so the tests are offline and instant. The server
// is returned for the tests whose identifier is itself a URL.
func testServer(t *testing.T, h http.Handler) (*Resolver, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	r := New()
	r.Endpoints = Endpoints{DOI: srv.URL, ArXiv: srv.URL, OpenLibrary: srv.URL}
	r.Client.sleep = func(context.Context, time.Duration) error { return nil }
	r.Now = func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC) }
	return r, srv
}

func testResolver(t *testing.T, h http.Handler) *Resolver {
	t.Helper()
	r, _ := testServer(t, h)
	return r
}

const doiRecord = `@article{Chen_2020,
  title = {Attention in Practice},
  author = {Chen, Yifan and Smith, John A.},
  journal = {Communications of the ACM},
  year = {2020},
  doi = {10.1145/3375637},
}
`

func TestResolveDOI(t *testing.T) {
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if got := req.Header.Get("Accept"); got != "application/x-bibtex" {
			t.Errorf("accept = %q, want application/x-bibtex", got)
		}
		if req.URL.Path != "/10.1145/3375637" {
			t.Errorf("path = %q", req.URL.Path)
		}
		w.Write([]byte(doiRecord))
	}))

	e, err := r.Resolve(context.Background(), "10.1145/3375637")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Key(); got != "Chen_2020" {
		t.Errorf("key = %q, want the key the resolver sent", got)
	}
	if got, _ := e.Value("title"); got != "Attention in Practice" {
		t.Errorf("title = %q", got)
	}
	if got, _ := e.Raw(kb.FieldRetrieved); got != "{2026-09-28}" {
		t.Errorf("retrieval date = %q, want {2026-09-28}", got)
	}
}

func TestResolveDOINormalizesTheIdentifier(t *testing.T) {
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/10.1145/3375637" {
			t.Errorf("path = %q, want the bare DOI", req.URL.Path)
		}
		w.Write([]byte(doiRecord))
	}))
	if _, err := r.Resolve(context.Background(), "https://doi.org/10.1145/3375637"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveDOINotFound(t *testing.T) {
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.NotFound(w, req)
	}))
	_, err := r.Resolve(context.Background(), "10.1145/0000000")
	if k, ok := KindOf(err); !ok || k != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if Operational(err) {
		t.Error("a missing record was reported as operational")
	}
}

func TestResolveDOINetworkFailureAfterRetries(t *testing.T) {
	var n int32
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&n, 1)
		http.Error(w, "down", http.StatusBadGateway)
	}))
	_, err := r.Resolve(context.Background(), "10.1145/3375637")
	if k, ok := KindOf(err); !ok || k != ErrNetwork {
		t.Fatalf("err = %v, want ErrNetwork", err)
	}
	if !Operational(err) {
		t.Error("a network failure was not reported as operational")
	}
	if got := atomic.LoadInt32(&n); got != int32(r.Client.Retries+1) {
		t.Errorf("requests = %d, want %d", got, r.Client.Retries+1)
	}
}

func TestResolvearXiv(t *testing.T) {
	const record = `@misc{vaswani2017attention,
      title={Attention Is All You Need},
      author={Ashish Vaswani and Noam Shazeer},
      year={2017},
      eprint={1706.03762},
      archivePrefix={arXiv},
}
`
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/bibtex/1706.03762" {
			t.Errorf("path = %q", req.URL.Path)
		}
		w.Write([]byte(record))
	}))

	e, err := r.Resolve(context.Background(), "arXiv:1706.03762")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Key(); got != "vaswani2017attention" {
		t.Errorf("key = %q", got)
	}
	if got, _ := e.Value("eprint"); got != "1706.03762" {
		t.Errorf("eprint = %q", got)
	}
}

func TestResolveISBNFollowsTheWorkForAuthors(t *testing.T) {
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/isbn/9780262033848.json":
			w.Write([]byte(`{
				"title": "Introduction to Algorithms",
				"publishers": ["The MIT Press"],
				"publish_date": "2009",
				"number_of_pages": 1292,
				"works": [{"key": "/works/OL123W"}],
				"key": "/books/OL456M"
			}`))
		case "/works/OL123W.json":
			w.Write([]byte(`{"authors":[{"key":"/authors/OL1A"},{"key":"/authors/OL2A"}]}`))
		case "/authors/OL1A.json":
			w.Write([]byte(`{"name":"Cormen, Thomas H."}`))
		case "/authors/OL2A.json":
			w.Write([]byte(`{"name":"Leiserson, Charles E."}`))
		default:
			http.NotFound(w, req)
		}
	}))

	e, err := r.Resolve(context.Background(), "978-0-262-03384-8")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Type(); got != "book" {
		t.Errorf("type = %q, want book", got)
	}
	if got := e.Key(); got != "cormen2009introduction" {
		t.Errorf("key = %q", got)
	}
	if got, _ := e.Value("author"); got != "Cormen, Thomas H. and Leiserson, Charles E." {
		t.Errorf("author = %q", got)
	}
	if got, _ := e.Value("publisher"); got != "The MIT Press" {
		t.Errorf("publisher = %q", got)
	}
	if got, _ := e.Value("year"); got != "2009" {
		t.Errorf("year = %q", got)
	}
	if got, _ := e.Value("isbn"); got != "9780262033848" {
		t.Errorf("isbn = %q", got)
	}
}

func TestResolveISBNRejectsABadChecksumWithoutCallingOut(t *testing.T) {
	var n int32
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&n, 1)
		http.NotFound(w, req)
	}))
	_, err := r.Resolve(context.Background(), "9780262033849")
	if k, ok := KindOf(err); !ok || k != ErrInvalid {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if got := atomic.LoadInt32(&n); got != 0 {
		t.Errorf("requests = %d, want none for a malformed ISBN", got)
	}
}

func TestResolveURLReadsCitationMetadata(t *testing.T) {
	const page = `<!doctype html>
<html><head>
  <title>Fallback title</title>
  <meta name="citation_title" content="A Study of &amp; Things">
  <meta name="citation_author" content="Doe, Jane">
  <meta name="citation_author" content="Roe, John">
  <meta name="citation_journal_title" content="Journal of Things">
  <meta name="citation_publication_date" content="2019-04-01">
  <meta property="og:site_name" content="Things Press">
</head><body>prose</body></html>`
	r, srv := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(page))
	}))

	e, err := r.Resolve(context.Background(), srv.URL+"/paper")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Type(); got != "article" {
		t.Errorf("type = %q, want article", got)
	}
	if got := e.Key(); got != "doe2019study" {
		t.Errorf("key = %q", got)
	}
	if got, _ := e.Value("title"); got != "A Study of & Things" {
		t.Errorf("title = %q", got)
	}
	if got, _ := e.Value("author"); got != "Doe, Jane and Roe, John" {
		t.Errorf("author = %q", got)
	}
	if got, _ := e.Value("url"); got != srv.URL+"/paper" {
		t.Errorf("url = %q", got)
	}
}

func TestResolveURLDelegatesToADOIItFinds(t *testing.T) {
	r, srv := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/10.1145/x" {
			w.Write([]byte("@article{FromDOI,\n  title = {Authoritative},\n}\n"))
			return
		}
		w.Write([]byte(`<html><head>
  <meta name="citation_doi" content="10.1145/x">
  <meta name="citation_title" content="A Landing Page">
</head></html>`))
	}))

	e, err := r.Resolve(context.Background(), srv.URL+"/landing")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Key(); got != "FromDOI" {
		t.Errorf("key = %q, want the DOI resolver's record", got)
	}
}

func TestResolveURLFallsBackToTheTitle(t *testing.T) {
	r, srv := testServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`<html><head><title>Plain Page</title></head><body>x</body></html>`))
	}))

	e, err := r.Resolve(context.Background(), srv.URL+"/plain")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Type(); got != "misc" {
		t.Errorf("type = %q, want misc", got)
	}
	if got, _ := e.Value("title"); got != "Plain Page" {
		t.Errorf("title = %q", got)
	}
	if got, _ := e.Value("url"); got != srv.URL+"/plain" {
		t.Errorf("url = %q", got)
	}
}

func TestResolveOfflineDoesNotTouchTheNetwork(t *testing.T) {
	var n int32
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&n, 1)
		w.Write([]byte(doiRecord))
	}))
	r.Offline = true

	_, err := r.Resolve(context.Background(), "10.1145/3375637")
	if k, ok := KindOf(err); !ok || k != ErrOffline {
		t.Fatalf("err = %v, want ErrOffline", err)
	}
	if !Operational(err) {
		t.Error("offline was not reported as operational")
	}
	if got := atomic.LoadInt32(&n); got != 0 {
		t.Errorf("requests = %d, want none while offline", got)
	}
}

func TestResolveRefusesWhatItCannotClassify(t *testing.T) {
	var n int32
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		atomic.AddInt32(&n, 1)
	}))
	_, err := r.Resolve(context.Background(), "just some words")
	if k, ok := KindOf(err); !ok || k != ErrInvalid {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if got := atomic.LoadInt32(&n); got != 0 {
		t.Errorf("requests = %d, want none", got)
	}
}

func TestResolveRefusesANonBibTeXAnswer(t *testing.T) {
	r := testResolver(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("<html>not a record</html>"))
	}))
	_, err := r.Resolve(context.Background(), "10.1145/3375637")
	if k, ok := KindOf(err); !ok || k != ErrNetwork {
		t.Fatalf("err = %v, want ErrNetwork", err)
	}
}

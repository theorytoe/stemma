package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/theorytoe/stemma/internal/version"
)

// fakeShim writes a shell script that stands in for the interpreter, so the whole
// boundary — arguments, stdout, exit status, stderr, the clock — can be exercised
// on a machine with no Python. It is invoked exactly as the real script is.
func fakeShim(t *testing.T, body string) *Runner {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shim")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Runner{Python: "/bin/sh", Script: path, Limit: 1 << 20}
}

// requireError asserts the kind of a failure and hands back the error itself.
func requireError(t *testing.T, err error, want Kind) *Error {
	t.Helper()
	if err == nil {
		t.Fatalf("want a %s failure, got success", want)
	}
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("failure is %T, want *extract.Error: %v", err, err)
	}
	if e.Kind != want {
		t.Fatalf("Kind = %q, want %q (%v)", e.Kind, want, err)
	}
	return e
}

func textFailure(t *testing.T, r *Runner) error {
	t.Helper()
	_, err := r.Text(context.Background(), "any.txt")
	if err == nil {
		t.Fatal("want a failure, got success")
	}
	return err
}

func TestTextCarriesTheWholeAnswerThrough(t *testing.T) {
	r := fakeShim(t, `printf '%s' '{"contract":1,"ok":true,"kind":"text","text":"hi\nthere\n",`+
		`"extractor":"stdlib","truncated":false,"notes":["a note"]}'`)

	got, err := r.Text(context.Background(), "note.txt")
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if got.Text != "hi\nthere\n" {
		t.Errorf("Text = %q", got.Text)
	}
	if got.Kind != "text" || got.Extractor != "stdlib" {
		t.Errorf("Kind = %q, Extractor = %q", got.Kind, got.Extractor)
	}
	if got.Truncated {
		t.Error("Truncated = true for an untruncated answer")
	}
	if len(got.Notes) != 1 || got.Notes[0] != "a note" {
		t.Errorf("Notes = %v", got.Notes)
	}
}

func TestEveryClassTheShimCanReportBecomesItsOwnKind(t *testing.T) {
	classes := []Kind{
		KindUnsupported, KindMissingExtractor, KindUnreadable,
		KindEmpty, KindNetwork, KindTooLarge, KindInternal,
	}
	for _, want := range classes {
		t.Run(string(want), func(t *testing.T) {
			r := fakeShim(t, fmt.Sprintf(
				`printf '%%s' '{"contract":1,"ok":false,"error":{"class":"%s","message":"it went wrong"}}'`,
				want))

			e := requireError(t, textFailure(t, r), want)
			if e.Message != "it went wrong" {
				t.Errorf("Message = %q, want the shim's own words", e.Message)
			}
		})
	}
}

func TestAClassThisBinaryDoesNotKnowIsNotGuessedAt(t *testing.T) {
	r := fakeShim(t, `printf '%s' '{"contract":1,"ok":false,"error":{"class":"teleported","message":"m"}}'`)

	e := requireError(t, textFailure(t, r), KindInternal)
	if !strings.Contains(e.Message, "teleported") {
		t.Errorf("Message = %q, want it to name the class that arrived", e.Message)
	}
}

func TestADyingShimIsReportedWithWhatItSaid(t *testing.T) {
	r := fakeShim(t, `echo "Traceback (most recent call last):" >&2; echo "ValueError: nope" >&2; exit 1`)

	e := requireError(t, textFailure(t, r), KindCrashed)
	if !strings.Contains(e.Detail, "ValueError: nope") {
		t.Errorf("Detail = %q, want the shim's stderr", e.Detail)
	}
}

func TestAnAbsentInterpreterIsItsOwnKind(t *testing.T) {
	r := &Runner{Python: filepath.Join(t.TempDir(), "absent"), Script: "any"}

	e := requireError(t, textFailure(t, r), KindMissingPython)
	if !strings.Contains(e.Error(), "absent") {
		t.Errorf("error = %q, want it to name what could not be run", e.Error())
	}
}

func TestAnAnswerThatIsNotOneObjectIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"not JSON":     `printf '%s' 'hello'`,
		"nothing":      `true`,
		"two objects":  `printf '%s' '{"contract":1,"ok":true}{"contract":1,"ok":true}'`,
		"no contract":  `printf '%s' '{"ok":true,"kind":"text","text":"x"}'`,
		"failure only": `printf '%s' '{"contract":1,"ok":false}'`,
	} {
		t.Run(name, func(t *testing.T) {
			requireError(t, textFailure(t, fakeShim(t, body)), KindCrashed)
		})
	}
}

func TestAContractFromAnotherVersionIsRefused(t *testing.T) {
	r := fakeShim(t, `printf '%s' '{"contract":99,"ok":true,"kind":"text","text":"x"}'`)

	e := requireError(t, textFailure(t, r), KindCrashed)
	if !strings.Contains(e.Error(), "out of date") {
		t.Errorf("error = %q, want it to say the script is out of date", e.Error())
	}
}

func TestOutputPastTheLimitIsRefused(t *testing.T) {
	// The shim is given a 1 KiB limit and prints 200 KiB anyway, which is past
	// everything the limit allows for. A shim that ignores its own limit must not
	// be able to exhaust this process.
	r := fakeShim(t, `printf '{"contract":1,"ok":true,"kind":"text","text":"';`+
		`head -c 200000 /dev/zero | tr '\0' 'a'; printf '"}'`)
	r.Limit = 1024

	requireError(t, textFailure(t, r), KindInternal)
}

func TestTheLimitBoundsTheTextAndNotTheEnvelope(t *testing.T) {
	// A quote is two bytes once JSON has carried it. Reading a document that is
	// largely quotation is ordinary — a JSON or LaTeX source is — so a drain sized
	// from the limit alone calls a correct shim a liar, and the reader is told the
	// tool is broken. The limit itself has to be met exactly, though: this shim
	// sends 128 KiB of text against a 128 KiB limit.
	r := fakeShim(t, `printf '{"contract":1,"ok":true,"kind":"text","text":"'`+"\n"+
		`yes '\"' | head -n 131072 | tr -d '\n'`+"\n"+
		`printf '","extractor":"shim"}'`)
	r.Limit = 128 << 10

	got, err := r.Text(context.Background(), "quotes.json")
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if len(got.Text) != 128<<10 {
		t.Errorf("text is %d bytes, want %d", len(got.Text), 128<<10)
	}
	if strings.Trim(got.Text, `"`) != "" {
		t.Errorf("text is not all quotes: %q", got.Text[:40])
	}
}

func TestASlowShimIsKilled(t *testing.T) {
	r := fakeShim(t, `exec sleep 5`)
	r.Timeout = 50 * time.Millisecond

	e := requireError(t, textFailure(t, r), KindTimeout)
	if !strings.Contains(e.Error(), "50ms") {
		t.Errorf("error = %q, want it to name the limit it exceeded", e.Error())
	}
}

func TestPDFCarriesThePagesAndTheNotes(t *testing.T) {
	r := fakeShim(t, `printf '%s' '{"contract":1,"ok":true,"kind":"pdf","text":"body",`+
		`"extractor":"pymupdf 1.28.2","pages":12,"truncated":false,`+
		`"notes":["1 of 12 pages have no text layer"]}'`)

	got, err := r.PDF(context.Background(), "p.pdf")
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	if got.Pages != 12 {
		t.Errorf("Pages = %d, want 12", got.Pages)
	}
	if got.Extractor != "pymupdf 1.28.2" {
		t.Errorf("Extractor = %q", got.Extractor)
	}
	if len(got.Notes) != 1 || !strings.Contains(got.Notes[0], "no text layer") {
		t.Errorf("Notes = %v", got.Notes)
	}
}

// The subcommand, the input and the limit are the half of the contract that is
// neither JSON nor an exit status, so they are checked by making the shim echo
// what it was given.
func TestTheArgumentsAreWhatTheContractSays(t *testing.T) {
	echo := func(t *testing.T, limit int, call func(*Runner) (*Result, error)) string {
		t.Helper()
		r := fakeShim(t, `printf '{"contract":1,"ok":true,"kind":"text","text":"%s","extractor":"x"}' "$*"`)
		r.Limit = limit
		got, err := call(r)
		if err != nil {
			t.Fatalf("call: %v", err)
		}
		return got.Text
	}

	text := echo(t, 1234, func(r *Runner) (*Result, error) {
		return r.Text(context.Background(), "a.txt")
	})
	if text != "text a.txt 1234" {
		t.Errorf("text was given %q, want %q", text, "text a.txt 1234")
	}

	pdf := echo(t, 4321, func(r *Runner) (*Result, error) {
		return r.PDF(context.Background(), "a.pdf")
	})
	if pdf != "pdf a.pdf 4321" {
		t.Errorf("pdf was given %q, want %q", pdf, "pdf a.pdf 4321")
	}

	link := echo(t, 2345, func(r *Runner) (*Result, error) {
		return r.URL(context.Background(), "http://example.test/page")
	})
	want := "url http://example.test/page 2345 " + version.UserAgent()
	if link != want {
		t.Errorf("url was given %q, want %q", link, want)
	}
}

func TestProbeReportsTheMachineRatherThanFailing(t *testing.T) {
	r := fakeShim(t, `printf '%s' '{"contract":1,"ok":true,"kind":"probe","python":"3.14.7",`+
		`"libs":{"pymupdf":"1.26.4","bs4":null}}'`)

	got, err := r.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if got.Python != "3.14.7" {
		t.Errorf("Python = %q", got.Python)
	}
	if !got.Has("pymupdf") || got.Version("pymupdf") != "1.26.4" {
		t.Errorf("pymupdf = %q, want a version", got.Version("pymupdf"))
	}
	// A library reported as absent is a fact, not a failure.
	if got.Has("bs4") || got.Version("bs4") != "" {
		t.Errorf("bs4 = %q, want it reported as not importable", got.Version("bs4"))
	}
}

func TestMaterializeWritesTheScriptFromTheBinary(t *testing.T) {
	root := t.TempDir()

	path, err := Materialize(root)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if path != ScriptPath(root) {
		t.Errorf("path = %q, want %q", path, ScriptPath(root))
	}
	have, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(have, script) {
		t.Error("what was written is not what the binary carries")
	}
	if got := ConditionOf(path); got != Current {
		t.Errorf("ConditionOf = %s, want current", got)
	}
}

func TestMaterializeReplacesACopyFromAnotherVersion(t *testing.T) {
	root := t.TempDir()
	path := ScriptPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# a copy from some older stemma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ConditionOf(path); got != Stale {
		t.Fatalf("ConditionOf = %s, want stale", got)
	}

	if _, err := Materialize(root); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if got := ConditionOf(path); got != Current {
		t.Errorf("ConditionOf = %s after Materialize, want current", got)
	}
}

func TestMaterializeLeavesACurrentCopyInPlace(t *testing.T) {
	root := t.TempDir()
	path, err := Materialize(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Materialize(root); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// A rewrite goes through a temporary neighbour and a rename, so it lands on a
	// new inode. This is what "it compared before writing" means concretely.
	if !os.SameFile(before, after) {
		t.Error("a copy that was already current was rewritten")
	}
}

func TestConditionOfTellsAbsenceFromStaleness(t *testing.T) {
	root := t.TempDir()
	missing := ScriptPath(root)
	if got := ConditionOf(missing); got != Absent {
		t.Errorf("ConditionOf(absent) = %s, want absent", got)
	}

	if err := os.MkdirAll(filepath.Dir(missing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(missing, []byte("something else"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ConditionOf(missing); got != Stale {
		t.Errorf("ConditionOf(other) = %s, want stale", got)
	}

	if got := ConditionOf(root); got != Stale {
		t.Errorf("ConditionOf(a directory) = %s, want stale", got)
	}
}

func TestPathPrefersTheOverride(t *testing.T) {
	t.Setenv(EnvShim, "/elsewhere/extract.py")
	if got := Path("/kb"); got != "/elsewhere/extract.py" {
		t.Errorf("Path = %q, want the override", got)
	}

	t.Setenv(EnvShim, "")
	if got := Path("/kb"); got != ScriptPath("/kb") {
		t.Errorf("Path = %q, want the KB's own copy", got)
	}
}

// The tests below are the ones that need a real interpreter. They skip rather
// than fail, because the core has to build and test on a machine without Python
// and these are evidence about the shim, not a precondition for the build.
func realRunner(t *testing.T) *Runner {
	t.Helper()
	python, err := FindPython()
	if err != nil {
		t.Skipf("no interpreter on this machine: %v", err)
	}
	path, err := Materialize(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Runner{Python: python, Script: path, Limit: 1 << 20}
}

func TestTextThroughTheRealShim(t *testing.T) {
	r := realRunner(t)
	file := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(file, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := r.Text(context.Background(), file)
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if got.Text != "hello\nworld\n" {
		t.Errorf("Text = %q", got.Text)
	}
	if got.Extractor != "stdlib" || got.Kind != "text" {
		t.Errorf("Extractor = %q, Kind = %q", got.Extractor, got.Kind)
	}
	if got.Truncated {
		t.Error("Truncated = true for a file well under the limit")
	}
}

func TestTruncationThroughTheRealShim(t *testing.T) {
	r := realRunner(t)
	r.Limit = 20
	file := filepath.Join(t.TempDir(), "lines.txt")
	if err := os.WriteFile(file, []byte(strings.Repeat("line\n", 50)), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := r.Text(context.Background(), file)
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if !got.Truncated {
		t.Error("Truncated = false for a file past the limit")
	}
	if len(got.Text) > 20 {
		t.Errorf("Text is %d bytes, want at most the 20 it was given", len(got.Text))
	}
	if !strings.HasSuffix(got.Text, "\n") {
		t.Errorf("Text = %q, want a cut on a line boundary", got.Text)
	}
}

func TestWhatTheShimCannotReadThroughTheRealShim(t *testing.T) {
	r := realRunner(t)
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	emptyErr := requireError(t, func() error { _, err := r.Text(context.Background(), empty); return err }(), KindEmpty)
	if !strings.Contains(emptyErr.Error(), "empty") {
		t.Errorf("error = %q", emptyErr.Error())
	}

	absent := filepath.Join(dir, "absent.txt")
	requireError(t, func() error { _, err := r.Text(context.Background(), absent); return err }(), KindUnreadable)
}

func TestProbeThroughTheRealShim(t *testing.T) {
	r := realRunner(t)

	got, err := r.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if got.Python == "" {
		t.Error("Python version is empty")
	}
	// probe reports on every library the contract names, importing none of them
	// being a fact rather than an error.
	for _, name := range []string{"pymupdf", "httpx", "bs4"} {
		if _, ok := got.Libs[name]; !ok {
			t.Errorf("probe did not report %s", name)
		}
	}
}

// The URL path needs a server, so this is the one place a real fetch happens end
// to end: Go's own test server, fetched by the real shim.
func TestURLThroughTheRealShim(t *testing.T) {
	r := realRunner(t)
	r.Timeout = 20 * time.Second

	probe, err := r.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	for _, needed := range []string{"httpx", "bs4"} {
		if !probe.Has(needed) {
			t.Skipf("%s is not installed, so a page cannot be read here", needed)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if agent := req.Header.Get("User-Agent"); agent != version.UserAgent() {
			t.Errorf("user agent = %q, want %q", agent, version.UserAgent())
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<html><body><header><nav>Menu</nav></header>"+
			"<article><h1>Heading</h1><p>%s</p></article><footer>Foot</footer></body></html>",
			strings.Repeat("Body text for the article. ", 20))
	}))
	defer server.Close()

	got, err := r.URL(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("URL: %v", err)
	}
	if got.Extractor != "httpx+bs4" {
		t.Errorf("Extractor = %q", got.Extractor)
	}
	if !strings.Contains(got.Text, "Heading") {
		t.Errorf("the article heading is missing: %q", got.Text[:min(80, len(got.Text))])
	}
	if !strings.Contains(got.Text, "Body text for the article") {
		t.Errorf("the article body is missing: %q", got.Text[:min(80, len(got.Text))])
	}
	if strings.Contains(got.Text, "Menu") || strings.Contains(got.Text, "Foot") {
		t.Errorf("the navigation or footer was kept: %q", got.Text)
	}
}

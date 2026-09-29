package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/extract"
)

// fakeRunner points fetch at a shell script standing in for the interpreter, so
// every path through the command is testable without Python.
func fakeRunner(t *testing.T, body string) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "shim")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := newRunner
	newRunner = func(string) (*extract.Runner, error) {
		return &extract.Runner{Python: "/bin/sh", Script: script, Limit: 1 << 20}, nil
	}
	t.Cleanup(func() { newRunner = old })
}

func fetchKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
	})
}

// document writes a file for fetch to read. Its contents do not matter: what the
// shim returns is canned, so the file only has to exist.
func document(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFetchWritesScratchAndPrintsTheText(t *testing.T) {
	root := fetchKB(t)
	fakeRunner(t, `printf '%s' '{"contract":1,"ok":true,"kind":"text","text":"the document\n","extractor":"stdlib"}'`)
	source := document(t, "note.txt", "whatever")

	code, stdout, stderr := run("fetch", "--kb", root, source)
	if code != ExitOK {
		t.Fatalf("fetch exited %d: %s", code, stderr)
	}
	// The text is the answer, so it is what a pipe carries.
	if stdout != "the document\n" {
		t.Errorf("stdout = %q, want the text", stdout)
	}

	path := extract.TextPath(root, source)
	if !strings.Contains(stderr, path) {
		t.Errorf("stderr = %q, want it to name %s", stderr, path)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the text was not written to scratch: %v", err)
	}
	if string(written) != "the document\n" {
		t.Errorf("the scratch file holds %q", written)
	}
}

func TestFetchReportsTheSameThingUnderJSON(t *testing.T) {
	root := fetchKB(t)
	fakeRunner(t, `printf '%s' '{"contract":1,"ok":true,"kind":"pdf","text":"body",`+
		`"extractor":"pymupdf 1.28.2","pages":12,"truncated":true,"notes":["1 of 12 pages have no text layer"]}'`)
	// A PDF is a PDF whatever it is called, so the name here says nothing and the
	// bytes decide which subcommand runs.
	source := document(t, "paper.txt", "%PDF-1.4\nrest of it")

	code, stdout, stderr := run("fetch", "--json", "--kb", root, source)
	if code != ExitOK {
		t.Fatalf("fetch exited %d: %s", code, stderr)
	}

	var env envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("the JSON envelope did not decode: %v", err)
	}
	var report fetchReport
	if err := json.Unmarshal(env.Data, &report); err != nil {
		t.Fatalf("the payload did not decode: %v", err)
	}
	if report.Kind != "pdf" {
		t.Errorf("Kind = %q, want the bytes to have chosen pdf", report.Kind)
	}
	if report.Extractor != "pymupdf 1.28.2" || report.Pages != 12 || !report.Truncated {
		t.Errorf("report = %+v", report)
	}
	if report.Text != "body" {
		t.Errorf("Text = %q, want the text in the payload as well as the file", report.Text)
	}
	if report.Path != extract.TextPath(root, source) {
		t.Errorf("Path = %q, want %q", report.Path, extract.TextPath(root, source))
	}
	if len(report.Notes) != 1 {
		t.Errorf("Notes = %v", report.Notes)
	}
}

func TestFetchRefusesWhatNamesAWorkRatherThanADocument(t *testing.T) {
	root := fetchKB(t)
	fakeRunner(t, `printf '%s' '{"contract":1,"ok":true,"kind":"text","text":"x"}'`)

	for _, pointer := range []string{"10.1145/3375637", "arXiv:1706.03762", "978-0-262-03384-8"} {
		code, _, stderr := run("fetch", "--kb", root, pointer)
		if code != ExitError {
			t.Errorf("fetch %q exited %d, want %d", pointer, code, ExitError)
		}
		if !strings.Contains(stderr, "cite add") {
			t.Errorf("fetch %q said %q, want it to point at cite add", pointer, stderr)
		}
	}
}

func TestFetchFailsWhenThereIsNothingToRead(t *testing.T) {
	root := fetchKB(t)
	fakeRunner(t, `printf '%s' '{"contract":1,"ok":true,"kind":"text","text":"x"}'`)

	code, _, stderr := run("fetch", "--kb", root, filepath.Join(t.TempDir(), "absent.txt"))
	if code != ExitError {
		t.Errorf("fetch exited %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "absent.txt") {
		t.Errorf("stderr = %q, want it to name the file", stderr)
	}
}

func TestFetchReportsAnExtractionFailure(t *testing.T) {
	root := fetchKB(t)
	fakeRunner(t, `printf '%s' '{"contract":1,"ok":false,"error":{"class":"missing_extractor",`+
		`"message":"pymupdf is not importable, so no PDF can be read"}}'`)
	source := document(t, "paper.pdf", "%PDF-1.4\nrest of it")

	code, _, stderr := run("fetch", "--kb", root, source)
	if code != ExitError {
		t.Errorf("fetch exited %d, want %d: extraction could not happen", code, ExitError)
	}
	if !strings.Contains(stderr, "missing_extractor") || !strings.Contains(stderr, "pymupdf") {
		t.Errorf("stderr = %q, want the class and the library", stderr)
	}
}

func TestFetchClearEmptiesTheScratchArea(t *testing.T) {
	root := fetchKB(t)
	dir := extract.ScratchDir(root)
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", filepath.Join("nested", "b.txt")} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("text"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	code, stdout, stderr := run("fetch", "--clear", "--kb", root)
	if code != ExitOK {
		t.Fatalf("fetch --clear exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "cleared 2 files") {
		t.Errorf("stdout = %q, want it to report two files", stdout)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("the scratch area is still there")
	}
}

func TestFetchClearWithNothingToClear(t *testing.T) {
	root := fetchKB(t)

	code, stdout, stderr := run("fetch", "--clear", "--kb", root)
	if code != ExitOK {
		t.Fatalf("fetch --clear exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "cleared 0 files") {
		t.Errorf("stdout = %q", stdout)
	}
}

// The one test here that needs a real interpreter. Text is the library-free case,
// so this much works wherever Python does.
func TestFetchThroughTheRealShim(t *testing.T) {
	root := fetchKB(t)
	if _, err := extract.FindPython(); err != nil {
		t.Skipf("no interpreter on this machine: %v", err)
	}
	source := document(t, "note.txt", "hello\nworld\n")

	code, stdout, stderr := run("fetch", "--kb", root, source)
	if code != ExitOK {
		t.Fatalf("fetch exited %d: %s", code, stderr)
	}
	if stdout != "hello\nworld\n" {
		t.Errorf("stdout = %q", stdout)
	}
	// Fetching materialises the script on the way past, because a KB that has had
	// its generated directory cleared is still a KB that can read a document.
	if got := extract.ConditionOf(extract.ScriptPath(root)); got != extract.Current {
		t.Errorf("the script is %s after a fetch, want current", got)
	}
}

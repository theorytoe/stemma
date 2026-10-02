package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/extract"
	"github.com/theorytoe/stemma/internal/kb"
)

// vendorKB builds a KB with one entry whose url is where its text comes from.
func vendorKB(t *testing.T, url string) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"bibliography.bib": "@article{bush1945,\n" +
			"  title = {As We May Think},\n" +
			"  url = {" + url + "},\n" +
			"}\n",
	})
}

// vendorPathKB is vendorKB for a source that is a file on this machine.
func vendorPathKB(t *testing.T, path string) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"bibliography.bib": "@article{bush1945,\n" +
			"  title = {As We May Think},\n" +
			"  path = {" + path + "},\n" +
			"}\n",
	})
}

// capture returns what is on disk for a key, and whether anything is.
func capture(t *testing.T, root, key string) (string, bool) {
	t.Helper()
	have, err := os.ReadFile(kb.VendoredPath(root, key))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(have), true
}

// recorded returns the vendored hash the bibliography holds for a key, which is
// read back from the file rather than from the command's own report.
func recorded(t *testing.T, root, key string) string {
	t.Helper()
	entry, ok := readBib(t, root, "bibliography.bib").Entry(key)
	if !ok {
		t.Fatalf("%s is not in the bibliography", key)
	}
	digest, _ := entry.Value(kb.FieldVendored)
	return digest
}

func shimReturning(t *testing.T, text string) {
	t.Helper()
	body, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	fakeRunner(t, `printf '%s' '{"contract":1,"ok":true,"kind":"url","text":`+string(body)+
		`,"extractor":"httpx+bs4"}'`)
}

func TestCiteVendorCapturesTheTextAndRecordsIt(t *testing.T) {
	root := vendorKB(t, "https://example.com/article")
	shimReturning(t, "the article text\n")

	code, stdout, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitOK {
		t.Fatalf("cite vendor exited %d: %s", code, stderr)
	}
	name := kb.VendoredName("bush1945")
	if !strings.Contains(stdout, name) {
		t.Errorf("stdout = %q, want it to name %s", stdout, name)
	}

	have, present := capture(t, root, "bush1945")
	if !present || have != "the article text\n" {
		t.Errorf("the capture is %q (present %v)", have, present)
	}
	// The hash has to reach the file, not just the report: a run that wrote the
	// text and forgot the record would look identical from the outside.
	if got, want := recorded(t, root, "bush1945"), kb.HashOf([]byte(have)); got != want {
		t.Errorf("recorded hash = %q, want %q", got, want)
	}
}

func TestCiteVendorLeavesAMatchingCaptureAlone(t *testing.T) {
	root := vendorKB(t, "https://example.com/article")
	shimReturning(t, "the article text\n")

	if code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945"); code != ExitOK {
		t.Fatalf("the first vendoring exited %d: %s", code, stderr)
	}
	code, stdout, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitOK {
		t.Fatalf("the second vendoring exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "already captured") {
		t.Errorf("stdout = %q, want it to say nothing changed", stdout)
	}
}

func TestCiteVendorRefusesToReplaceACaptureWithDifferentText(t *testing.T) {
	root := vendorKB(t, "https://example.com/article")
	shimReturning(t, "the first text\n")
	if code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945"); code != ExitOK {
		t.Fatalf("the first vendoring exited %d: %s", code, stderr)
	}

	shimReturning(t, "a completely different text\n")
	code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitError {
		t.Fatalf("exited %d, want %d: replacing a capture is a decision", code, ExitError)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr = %q, want it to say how to replace it", stderr)
	}
	// Neither side is touched by a refusal, which is what "trust neither" means.
	if have, _ := capture(t, root, "bush1945"); have != "the first text\n" {
		t.Errorf("the capture became %q", have)
	}
	if got := recorded(t, root, "bush1945"); got != kb.HashOf([]byte("the first text\n")) {
		t.Errorf("the recorded hash became %q", got)
	}
}

func TestCiteVendorForceReplacesACapture(t *testing.T) {
	root := vendorKB(t, "https://example.com/article")
	shimReturning(t, "the first text\n")
	if code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945"); code != ExitOK {
		t.Fatalf("the first vendoring exited %d: %s", code, stderr)
	}

	shimReturning(t, "the second text\n")
	code, stdout, stderr := run("cite", "vendor", "--force", "--kb", root, "bush1945")
	if code != ExitOK {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "captured") {
		t.Errorf("stdout = %q", stdout)
	}
	if have, _ := capture(t, root, "bush1945"); have != "the second text\n" {
		t.Errorf("the capture is %q", have)
	}
	if got := recorded(t, root, "bush1945"); got != kb.HashOf([]byte("the second text\n")) {
		t.Errorf("recorded hash = %q", got)
	}
}

// The text can be right while the record lags behind it — a hand-removed field, or
// a run that died between the two writes. Recording it again writes no text.
func TestCiteVendorRecordsACaptureThatIsAlreadyThere(t *testing.T) {
	root := vendorKB(t, "https://example.com/article")
	text := "the article text\n"
	full := kb.VendoredPath(root, "bush1945")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	shimReturning(t, text)

	code, stdout, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitOK {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "recorded") {
		t.Errorf("stdout = %q", stdout)
	}
	if got := recorded(t, root, "bush1945"); got != kb.HashOf([]byte(text)) {
		t.Errorf("recorded hash = %q", got)
	}
}

func TestCiteVendorNeedsSomewhereToReadFrom(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"bibliography.bib": "@article{bush1945,\n" +
			"  title = {As We May Think},\n" +
			"}\n",
	})

	code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitError {
		t.Fatalf("exited %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "url") {
		t.Errorf("stderr = %q, want it to say the entry needs a url", stderr)
	}
}

func TestCiteVendorNeedsAnEntry(t *testing.T) {
	root := vendorKB(t, "https://example.com/article")

	code, _, stderr := run("cite", "vendor", "--kb", root, "nobody")
	if code != ExitError {
		t.Fatalf("exited %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "nobody") {
		t.Errorf("stderr = %q, want it to name the key", stderr)
	}
}

func TestCiteVendorRefusesAPartialCapture(t *testing.T) {
	root := vendorKB(t, "https://example.com/huge")
	fakeRunner(t, `printf '%s' '{"contract":1,"ok":true,"kind":"url","text":"the start of it",`+
		`"extractor":"httpx+bs4","truncated":true}'`)

	code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitError {
		t.Fatalf("exited %d, want %d: half a document is not a capture", code, ExitError)
	}
	if !strings.Contains(stderr, "limit") {
		t.Errorf("stderr = %q, want it to say the text was cut", stderr)
	}
	if _, present := capture(t, root, "bush1945"); present {
		t.Error("a truncated text was captured anyway")
	}
}

func TestCiteVendorReportsUnderJSON(t *testing.T) {
	root := vendorKB(t, "https://example.com/article")
	shimReturning(t, "the article text\n")

	code, stdout, stderr := run("cite", "vendor", "--json", "--kb", root, "bush1945")
	if code != ExitOK {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	var env envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("the envelope did not decode: %v", err)
	}
	var report vendorReport
	if err := json.Unmarshal(env.Data, &report); err != nil {
		t.Fatalf("the payload did not decode: %v", err)
	}
	if report.Action != "captured" || report.Key != "bush1945" {
		t.Errorf("report = %+v", report)
	}
	if report.Hash != kb.HashOf([]byte("the article text\n")) {
		t.Errorf("Hash = %q", report.Hash)
	}
	if report.Bytes != len("the article text\n") {
		t.Errorf("Bytes = %d", report.Bytes)
	}
	if report.Path != kb.VendoredName("bush1945") {
		t.Errorf("Path = %q, want %q", report.Path, kb.VendoredName("bush1945"))
	}
}

// TestCiteVendorCopiesALocalOriginal checks that a source which is a file on
// this machine keeps the file itself beside the text drawn out of it, so the
// site can serve the original format.
func TestCiteVendorCopiesALocalOriginal(t *testing.T) {
	document := filepath.Join(t.TempDir(), "paper.md")
	body := "# A Paper\n\nThe body in markdown.\n"
	if err := os.WriteFile(document, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	root := vendorPathKB(t, document)
	shimReturning(t, "the extracted text\n")

	code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitOK {
		t.Fatalf("exited %d: %s", code, stderr)
	}

	// The extracted text is the capture the entry records a hash of.
	if have, present := capture(t, root, "bush1945"); !present || have != "the extracted text\n" {
		t.Errorf("the capture is %q (present %v)", have, present)
	}
	// The original is beside it, under the extension it was read from.
	have, err := os.ReadFile(kb.OriginalPath(root, "bush1945", ".md"))
	if err != nil {
		t.Fatalf("the original was not captured: %v", err)
	}
	if string(have) != body {
		t.Errorf("the original is %q, want %q", have, body)
	}
	if got := recorded(t, root, "bush1945"); got != kb.HashOf([]byte(body)) {
		t.Errorf("recorded hash = %q, want the hash of the original", got)
	}
}

// A local source has no resolver, so before it is captured it carries no
// provenance. The capture is the evidence, and once it is there the record is
// no longer asked for a hash and a date a local file cannot supply.
func TestCiteVendorSatisfiesTheProvenanceCheck(t *testing.T) {
	document := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(document, []byte("# Note\n\nsome text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := vendorPathKB(t, document)
	shimReturning(t, "some text\n")

	code, stdout, _ := run("cite", "check", "--kb", root)
	if code != ExitFindings {
		t.Fatalf("before vendoring: exit %d, want %d", code, ExitFindings)
	}
	if !strings.Contains(stdout, "no content hash") {
		t.Fatalf("before vendoring, check said %q", stdout)
	}

	if code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945"); code != ExitOK {
		t.Fatalf("vendor: exit %d: %s", code, stderr)
	}

	if code, stdout, stderr := run("cite", "check", "--kb", root); code != ExitOK {
		t.Errorf("after vendoring: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}

// The one test here that reads a real document. Text needs no library, so this
// works wherever Python does.
func TestCiteVendorThroughTheRealShim(t *testing.T) {
	if _, err := extract.FindPython(); err != nil {
		t.Skipf("no interpreter on this machine: %v", err)
	}
	document := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(document, []byte("hello from a file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A path in the path field, which is how a source that is only on this
	// machine is recorded.
	root := vendorPathKB(t, document)

	code, _, stderr := run("cite", "vendor", "--kb", root, "bush1945")
	if code != ExitOK {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	if have, _ := capture(t, root, "bush1945"); have != "hello from a file\n" {
		t.Errorf("the capture is %q", have)
	}
}

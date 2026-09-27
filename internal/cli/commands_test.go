package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// freshKB creates a KB with the tool itself, so the tests exercise init rather
// than a fixture that could drift from it.
func freshKB(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "kb")
	if code, _, stderr := run("init", root, "--title", "Demo"); code != ExitOK {
		t.Fatalf("init: exit %d: %s", code, stderr)
	}
	return root
}

func TestInitReportsWhatItMade(t *testing.T) {
	root := filepath.Join(t.TempDir(), "kb")
	code, stdout, stderr := run("init", root, "--title", "Demo")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Demo") {
		t.Errorf("stdout = %q", stdout)
	}
	for _, name := range []string{"stemma.toml", "pages/index.md", "bibliography.bib", "inbox"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("init did not create %s", name)
		}
	}
}

func TestInitRefusesASecondTime(t *testing.T) {
	root := freshKB(t)
	code, _, stderr := run("init", root)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "looks like a KB root") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestInitJSON(t *testing.T) {
	root := filepath.Join(t.TempDir(), "kb")
	code, stdout, _ := run("init", root, "--title", "Demo", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if got["title"] != "Demo" {
		t.Errorf("title = %q", got["title"])
	}
}

// The standard library stops parsing at the first positional argument, so this
// order only works because the arguments are rearranged before parsing. Without
// that, --draft would have been read as part of a title and a page would have
// been created instead of a draft.
func TestFlagsMayFollowPositionalArguments(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("new", "A New Page", "--kb", root, "--type", "concept")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pages/a-new-page.md" {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestNewCreatesAPage(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("new", "--kb", root, "Attention Is All You Need", "--type", "concept")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pages/attention-is-all-you-need.md" {
		t.Errorf("stdout = %q", stdout)
	}

	body, err := os.ReadFile(filepath.Join(root, "pages", "attention-is-all-you-need.md"))
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntitle: Attention Is All You Need\ntype: concept\n---\n"
	if string(body) != want {
		t.Errorf("page =\n%q\nwant\n%q", body, want)
	}
}

func TestNewCreatesADraft(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("new", "--kb", root, "Half A Thought", "--draft")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "inbox/half-a-thought.md" {
		t.Errorf("stdout = %q", stdout)
	}
	// A draft is not a page, so it is not listed.
	_, listing, _ := run("list", "--kb", root)
	if strings.Contains(listing, "half-a-thought") {
		t.Errorf("a draft was listed as a page:\n%s", listing)
	}
}

func TestNewUsesTheManifestsDefaultType(t *testing.T) {
	root := writeTree(t, map[string]string{
		"stemma.toml":    "default_type = \"concept\"\ntypes = [\"person\"]\n",
		"pages/index.md": page("Index", "type: index", ""),
	})
	if code, _, stderr := run("new", "--kb", root, "Someone"); code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	body, err := os.ReadFile(filepath.Join(root, "pages", "someone.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "type: concept") {
		t.Errorf("the default type was not applied:\n%s", body)
	}

	// And the manifest's own vocabulary is usable.
	if code, _, stderr := run("new", "--kb", root, "Somebody", "--type", "person"); code != ExitOK {
		t.Errorf("a configured type was refused: %s", stderr)
	}
}

func TestNewRefusesWhatWouldBreakTheKB(t *testing.T) {
	root := freshKB(t)
	if code, _, _ := run("new", "--kb", root, "Attention"); code != ExitOK {
		t.Fatal("the first page was refused")
	}

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"a name another page answers to", []string{"attention"}, "already the name of"},
		{"a reserved type", []string{"Other", "--type", "source"}, "tool owns"},
		{"an unknown type", []string{"Other", "--type", "wizard"}, "not a type this KB knows"},
		{"a title nothing could link to", []string{"!!!"}, "nothing could link to it"},
		{"no title at all", nil, "takes one title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"new", "--kb", root}, tc.args...)
			code, _, stderr := run(args...)
			if code != ExitError {
				t.Errorf("exit = %d, want %d", code, ExitError)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want it to mention %q", stderr, tc.want)
			}
		})
	}
}

func TestList(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Alpha", "--type", "concept")
	run("new", "--kb", root, "Beta", "--type", "note")

	code, stdout, stderr := run("list", "--kb", root)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"pages/index.md\tDemo", "pages/alpha.md\tAlpha", "pages/beta.md\tBeta"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("listing is missing %q:\n%s", want, stdout)
		}
	}

	_, filtered, _ := run("list", "--kb", root, "--type", "note")
	if strings.Contains(filtered, "Alpha") || !strings.Contains(filtered, "Beta") {
		t.Errorf("--type did not filter:\n%s", filtered)
	}
}

func TestListJSON(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Alpha", "--type", "concept")

	_, stdout, _ := run("list", "--kb", root, "--json")
	var got struct {
		Count int `json:"count"`
		Pages []struct {
			Path, Title, Type, Status string
		} `json:"pages"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if got.Count != 2 || len(got.Pages) != 2 {
		t.Errorf("count = %d, pages = %d", got.Count, len(got.Pages))
	}
}

// The two script modes each print one thing and nothing else, so neither needs
// parsing.
func TestShowPathAndRaw(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Alpha", "--type", "concept")
	target := filepath.Join(root, "pages", "alpha.md")
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run("show", "--kb", root, "alpha", "--path")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if stdout != "pages/alpha.md\n" {
		t.Errorf("--path printed %q", stdout)
	}

	code, stdout, stderr = run("show", "--kb", root, "Alpha", "--raw")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if stdout != string(original) {
		t.Errorf("--raw is not byte-identical:\n%q\n%q", stdout, original)
	}

	if code, _, _ := run("show", "--kb", root, "alpha", "--path", "--raw"); code != ExitError {
		t.Error("--path and --raw together were accepted")
	}
}

func TestShowResolvesLinksAndCitations(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", "start at [[Alpha]]\n"),
		"pages/alpha.md":   page("Alpha", "type: concept\naliases:\n  - alfa", "see [[Beta]], [[Missing]] and [@present]\n"),
		"pages/beta.md":    page("Beta", "type: concept", ""),
		"bibliography.bib": "@article{present,}\n",
	})

	code, stdout, stderr := run("show", "--kb", root, "alfa")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{
		"title   Alpha",
		"aliases alfa",
		"[[Beta]] -> pages/beta.md",
		"[[Missing]] -> unresolved",
		"[@present] -> bibliography.bib",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("show is missing %q:\n%s", want, stdout)
		}
	}
}

func TestShowJSON(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Alpha", "--type", "concept")

	_, stdout, _ := run("show", "--kb", root, "alpha", "--json")
	var got struct {
		Path  string `json:"path"`
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if got.Path != "pages/alpha.md" || got.Title != "Alpha" || got.Type != "concept" {
		t.Errorf("got %+v", got)
	}
}

func TestShowRefusesANameItCannotResolveOneWay(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/a.md":     page("Go", "type: concept", ""),
		"pages/b.md":     page("Golang", "type: concept\naliases:\n  - go", ""),
	})

	if code, _, stderr := run("show", "--kb", root, "go"); code != ExitError {
		t.Errorf("an ambiguous name was accepted: %s", stderr)
	}
	if code, _, stderr := run("show", "--kb", root, "nothing"); code != ExitError {
		t.Errorf("an unknown name was accepted: %s", stderr)
	}
}

// Directories carry no meaning, so moving a page has to change no link and
// break nothing. If this ever needs rewriting to pass, the layout decision has
// been undone somewhere.
func TestMoveChangesNoLink(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", "start at [[Alpha]]\n"),
		"pages/alpha.md": page("Alpha", "type: concept", "see [[Beta]]\n"),
		"pages/beta.md":  page("Beta", "type: concept", ""),
	})
	before, err := os.ReadFile(filepath.Join(root, "pages", "index.md"))
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run("move", "--kb", root, "Alpha", "pages/deeper")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pages/alpha.md -> pages/deeper/alpha.md" {
		t.Errorf("stdout = %q", stdout)
	}

	after, err := os.ReadFile(filepath.Join(root, "pages", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("moving a page rewrote a link to it")
	}
	if code, _, stderr := run("lint", "--kb", root, "--strict"); code != ExitOK {
		t.Errorf("the KB does not lint clean after a move: %s", stderr)
	}
	_, shown, _ := run("show", "--kb", root, "Alpha")
	if !strings.Contains(shown, "pages/deeper/alpha.md") {
		t.Errorf("the page did not move:\n%s", shown)
	}
}

func TestMoveRefusesDestinationsOutsidePages(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Alpha", "--type", "concept")

	for _, destination := range []string{"inbox", "pages/../inbox", "/tmp/elsewhere", "..", "../elsewhere"} {
		t.Run(destination, func(t *testing.T) {
			code, _, stderr := run("move", "--kb", root, "alpha", destination)
			if code != ExitError {
				t.Errorf("exit = %d, want %d", code, ExitError)
			}
			if !strings.Contains(stderr, "outside the KB") && !strings.Contains(stderr, "not under") {
				t.Errorf("stderr = %q", stderr)
			}
		})
	}
}

func TestMoveRefusesToReplaceAPage(t *testing.T) {
	// Two pages whose files happen to share a name, which `new` would never
	// produce because it derives the filename from the title. An author who
	// chose a filename by hand can, and a move must not silently replace one.
	root := writeTree(t, map[string]string{
		"pages/index.md":     page("Index", "type: index", ""),
		"pages/alpha.md":     page("Alpha", "type: concept", ""),
		"pages/sub/alpha.md": page("Sub Thing", "type: concept", ""),
	})

	code, _, stderr := run("move", "--kb", root, "Alpha", "pages/sub")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr = %q", stderr)
	}
	// Both pages are still where they were.
	for _, name := range []string{"pages/alpha.md", "pages/sub/alpha.md"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("%s went missing: %v", name, err)
		}
	}
}

func TestArchiveRecordsWhy(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Alpha", "--type", "concept")

	if code, _, stderr := run("archive", "--kb", root, "alpha"); code != ExitError {
		t.Errorf("archive without a reason was accepted: %s", stderr)
	}

	code, stdout, stderr := run("archive", "--kb", root, "alpha", "--reason", "superseded")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "archived") {
		t.Errorf("stdout = %q", stdout)
	}

	body, err := os.ReadFile(filepath.Join(root, "pages", "alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status: archived") {
		t.Errorf("status was not set:\n%s", body)
	}
	if !strings.Contains(string(body), "archive_reason: superseded") {
		t.Errorf("the reason was not recorded:\n%s", body)
	}
	if _, err := os.Lstat(filepath.Join(root, "pages", "alpha.md")); err != nil {
		t.Error("archiving deleted the page")
	}
}

func TestArchiveJSON(t *testing.T) {
	root := freshKB(t)
	run("new", "--kb", root, "Alpha", "--type", "concept")

	_, stdout, _ := run("archive", "--kb", root, "alpha", "--reason", "done", "--json")
	var got map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	if got["status"] != "archived" || got["reason"] != "done" {
		t.Errorf("got %+v", got)
	}
}

// The whole surface, end to end, has to leave a KB that lints clean under
// strict.
//
// It cannot be clean the moment the verbs finish, and that is the point rather
// than a gap: a page nothing links to is an orphan, and the tool never writes
// prose, so linking pages to each other is the author's work. What the verbs
// must not do is introduce anything structural — a bad type, a name collision,
// a broken link — and that is what the first assertion checks.
func TestTheCoreVerbsIntroduceNothingStructural(t *testing.T) {
	root := freshKB(t)
	steps := [][]string{
		{"new", "--kb", root, "Attention Is All You Need", "--type", "concept"},
		{"new", "--kb", root, "Transformer", "--type", "concept"},
		{"new", "--kb", root, "A Draft", "--draft"},
		{"move", "--kb", root, "Transformer", "pages/papers"},
		{"archive", "--kb", root, "Transformer", "--reason", "superseded"},
	}
	for _, args := range steps {
		if code, _, stderr := run(args...); code != ExitOK {
			t.Fatalf("%v: exit %d: %s", args, code, stderr)
		}
	}

	// Everything reported is an orphan, and nothing else. A fresh KB has no
	// links yet, so two pages and an index give exactly two.
	code, stdout, _ := run("lint", "--kb", root, "--strict")
	if code != ExitFindings {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitFindings, stdout)
	}
	if got := strings.Count(stdout, "no page links here"); got != 2 {
		t.Errorf("orphans = %d, want 2:\n%s", got, stdout)
	}
	if others := strings.Count(stdout, ": error:") - 2; others != 0 {
		t.Errorf("%d findings that are not orphans:\n%s", others, stdout)
	}

	// Now the author links the pages in, which is the only thing standing
	// between this KB and clean.
	index := page("Demo", "type: index",
		"Start at [[Attention Is All You Need]] and [[Transformer]].\n")
	if err := os.WriteFile(filepath.Join(root, "pages", "index.md"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}

	if code, stdout, stderr := run("lint", "--kb", root, "--strict"); code != ExitOK {
		t.Fatalf("the KB is not clean once its pages are linked: exit %d\n%s\n%s", code, stdout, stderr)
	}
}

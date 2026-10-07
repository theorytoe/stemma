package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// manDateRE matches the .TH date field, which is the day the page was written.
var manDateRE = regexp.MustCompile(`"\d{4}-\d{2}-\d{2}"`)

// The pages are generated from the command table, so every verb and every
// family member has to have a page and no page may be empty or unrenderable.
// Comparing the set against manTargets is the point: a verb added to the table
// without a page fails here rather than going missing quietly.
func TestManWritesAPagePerCommand(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := run("help", "--man", "--out", dir)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != len(manTargets()) {
		t.Errorf("%d pages written, want %d", len(entries), len(manTargets()))
	}

	for _, target := range manTargets() {
		path := filepath.Join(dir, manFileName(target.page))
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", target.page, err)
			continue
		}
		for _, want := range []string{".TH ", ".SH NAME", ".SH SYNOPSIS", ".SH EXIT STATUS", ".SH SEE ALSO"} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s is missing %q:\n%s", target.page, want, b)
			}
		}
		// The .TH line carries a date, because an undated page is what mandoc
		// reports first and the generation date is the only true one available.
		if !manDateRE.MatchString(strings.SplitN(string(b), "\n", 2)[0]) {
			t.Errorf("%s has no .TH date: %s", target.page, b)
		}
	}

	// A family member is the case that is easy to get wrong: its page name
	// carries the family, and its synopsis is two words.
	if _, err := os.Stat(filepath.Join(dir, "stemma-cite-add.1")); err != nil {
		t.Errorf("a family member has no page: %v", err)
	}
}

// Naming a command with a directory writes that page and nothing else, so a
// caller can regenerate one page without touching the rest of the set.
func TestManWritesOneNamedPage(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := run("help", "lint", "--man", "--out", dir)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != "stemma-lint.1" {
		t.Errorf("a named page wrote %d files: %v", len(entries), entries)
	}
}

// A page names the pages around it, not itself.
func TestManDoesNotReferenceItself(t *testing.T) {
	for _, target := range manTargets() {
		body := manPage(target)
		seeAlso := body[strings.Index(body, ".SH SEE ALSO"):]
		if strings.Contains(seeAlso, manRef(target.page)) {
			t.Errorf("%s lists itself under SEE ALSO:\n%s", target.page, seeAlso)
		}
	}
}

// A command's page has to carry the flags the command actually defines, the
// environment it reads, and the exit codes, so that a person who reaches for a
// manual page instead of --help is not told less.
func TestManPageDocumentsItsFlags(t *testing.T) {
	code, stdout, stderr := run("help", "lint", "--man")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{
		".TH STEMMA-LINT",
		"stemma lint",
		"report everything wrong with the KB",
		".SH OPTIONS",
		`\-\-json`,
		`\-\-kb`,
		`\-\-strict`,
		".SH ENVIRONMENT",
		EnvKB,
		`\fBstemma\fR(1)`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the lint page is missing %q:\n%s", want, stdout)
		}
	}
}

// A command that takes no KB must not be told about STEMMA_KB, because a page
// that documents a variable the command ignores is worse than a shorter page.
func TestManPageOmitsAnEnvironmentItDoesNotRead(t *testing.T) {
	code, stdout, stderr := run("help", "init", "--man")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if strings.Contains(stdout, ".SH ENVIRONMENT") {
		t.Errorf("init reads no environment, but its page documents one:\n%s", stdout)
	}
}

// The family page is where a person lands after learning that `cite` is a noun,
// so it has to name the members and point at each one's page.
func TestManFamilyPageListsItsMembers(t *testing.T) {
	code, stdout, stderr := run("help", "cite", "--man")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{".SH COMMANDS", `\fBstemma\-cite\-add\fR(1)`, `\fBstemma\-cite\-check\fR(1)`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the cite page is missing %q:\n%s", want, stdout)
		}
	}
}

// The index is the page a person gets from `man stemma`, so it has to name
// every verb and offer a way to the page that describes it.
func TestManIndexNamesEveryCommand(t *testing.T) {
	code, stdout, stderr := run("help", "--man")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, c := range commands {
		want := `\fBstemma\-` + c.name + `\fR(1)`
		if !strings.Contains(stdout, want) {
			t.Errorf("the index does not point at %q:\n%s", c.name, stdout)
		}
	}
}

// Text the author wrote may contain a hyphen and may begin with a dot, and both
// mean something to roff. A page that renders differently from the sentence it
// was built from is a wrong page.
func TestManEscapesRoff(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"--json", `\-\-json`},
		{"Tier-1", `Tier\-1`},
		{`a\b`, `a\eb`},
		{"plain", "plain"},
	} {
		if got := manText(tc.in); got != tc.want {
			t.Errorf("manText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	var b strings.Builder
	manParagraph(&b, ".SH is a request")
	if !strings.HasPrefix(b.String(), `\&.SH`) {
		t.Errorf("a line beginning with a request was not guarded:\n%s", b.String())
	}
}

// --json carries the rendered page for one page and the written paths for a
// directory, so an agent neither parses roff out of a document nor guesses
// where the files went.
func TestManJSON(t *testing.T) {
	code, stdout, stderr := run("help", "lint", "--man", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var one struct {
		Man string `json:"man"`
	}
	decodeData(t, stdout, &one)
	if !strings.HasPrefix(one.Man, ".TH STEMMA-LINT") {
		t.Errorf("the page is not in the envelope:\n%s", one.Man)
	}

	dir := t.TempDir()
	code, stdout, stderr = run("help", "--man", "--out", dir, "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var report struct {
		Dir   string `json:"dir"`
		Pages []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"pages"`
	}
	decodeData(t, stdout, &report)
	if report.Dir != dir {
		t.Errorf("dir = %q, want %q", report.Dir, dir)
	}
	if len(report.Pages) != len(manTargets()) {
		t.Fatalf("%d pages reported, want %d", len(report.Pages), len(manTargets()))
	}
	for _, p := range report.Pages {
		if _, err := os.Stat(p.Path); err != nil {
			t.Errorf("%s was reported but not written: %v", p.Path, err)
		}
	}
}

// --out is a redirect for --man and nothing else, so asking for it alone is a
// usage error rather than a run that quietly writes nothing. Two renderings at
// once is a question, not a preference, and is refused for the same reason.
func TestManOutNeedsMan(t *testing.T) {
	code, _, stderr := run("help", "--out", t.TempDir())
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "--man") {
		t.Errorf("stderr = %q", stderr)
	}

	code, _, stderr = run("help", "--man", "--markdown")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "markdown") {
		t.Errorf("stderr = %q", stderr)
	}
}

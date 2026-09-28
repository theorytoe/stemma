package cli

import (
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// citeKB is a small bibliography with one cited entry and one that nothing
// points at, which is what the list filters are about.
func citeKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/a.md":     page("A", "type: concept", "uses [@bush1945]\n"),
		"bibliography.bib": "@article{bush1945,\n" +
			"  title = {As We May Think},\n" +
			"  author = {Bush, Vannevar},\n" +
			"  year = {1945},\n" +
			"}\n" +
			"@book{cormen2009,\n" +
			"  title = {Introduction to Algorithms},\n" +
			"  author = {Cormen, Thomas H.},\n" +
			"  year = {2009},\n" +
			"}\n",
	})
}

func TestCiteListShowsEverythingSortedByKey(t *testing.T) {
	code, stdout, stderr := run("cite", "list", "--kb", citeKB(t))
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"bush1945", "cormen2009", "As We May Think", "Cormen, Thomas H."} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}
	if strings.Index(stdout, "bush1945") > strings.Index(stdout, "cormen2009") {
		t.Errorf("rows are not sorted by key:\n%s", stdout)
	}
}

func TestCiteListFilters(t *testing.T) {
	root := citeKB(t)
	for _, tc := range []struct {
		name string
		args []string
		want []string
		not  []string
	}{
		{"uncited", []string{"--uncited"}, []string{"cormen2009"}, []string{"bush1945"}},
		{"cited", []string{"--cited"}, []string{"bush1945"}, []string{"cormen2009"}},
		{"by author", []string{"--author", "cormen"}, []string{"cormen2009"}, []string{"bush1945"}},
		{"by type", []string{"--type", "book"}, []string{"cormen2009"}, []string{"bush1945"}},
		{"by year", []string{"--year", "1945"}, []string{"bush1945"}, []string{"cormen2009"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"cite", "list", "--kb", root}, tc.args...)
			code, stdout, stderr := run(args...)
			if code != ExitOK {
				t.Fatalf("exit = %d: %s", code, stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout is missing %q:\n%s", want, stdout)
				}
			}
			for _, not := range tc.not {
				if strings.Contains(stdout, not) {
					t.Errorf("stdout unexpectedly has %q:\n%s", not, stdout)
				}
			}
		})
	}
}

func TestCiteListRefusesOppositeFilters(t *testing.T) {
	code, _, stderr := run("cite", "list", "--kb", citeKB(t), "--cited", "--uncited")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "opposite") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestCiteListJSON(t *testing.T) {
	code, stdout, stderr := run("cite", "list", "--kb", citeKB(t), "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var rows []citeRow
	e := decodeData(t, stdout, &rows)
	if e.Command != "cite list" {
		t.Errorf("command = %q", e.Command)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Key != "bush1945" || !rows[0].Cited || rows[0].Type != "article" {
		t.Errorf("first row = %+v", rows[0])
	}
	if rows[1].Key != "cormen2009" || rows[1].Cited {
		t.Errorf("second row = %+v", rows[1])
	}
}

func TestCiteListJSONEmptyIsAnArray(t *testing.T) {
	code, stdout, _ := run("cite", "list", "--kb", citeKB(t), "--json", "--year", "1500")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if strings.TrimSpace(string(decodeJSON(t, stdout).Data)) != "[]" {
		t.Errorf("data = %s, want []", decodeJSON(t, stdout).Data)
	}
}

func TestCiteShow(t *testing.T) {
	code, stdout, stderr := run("cite", "show", "--kb", citeKB(t), "bush1945")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"@article{bush1945", "defined in bibliography.bib", "pages/a.md"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}
}

func TestCiteShowJSON(t *testing.T) {
	code, stdout, stderr := run("cite", "show", "--kb", citeKB(t), "bush1945", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var got citeShowReport
	decodeData(t, stdout, &got)
	if got.Key != "bush1945" || got.Type != "article" || got.Path != "bibliography.bib" {
		t.Errorf("report = %+v", got)
	}
	if len(got.Fields) == 0 || got.Fields[0].Name == "" || got.Fields[0].Value == "" {
		t.Errorf("fields = %+v", got.Fields)
	}
	if len(got.CitedBy) != 1 || got.CitedBy[0] != "pages/a.md" {
		t.Errorf("cited_by = %q", got.CitedBy)
	}
}

func TestCiteShowUnknownKey(t *testing.T) {
	code, _, stderr := run("cite", "show", "--kb", citeKB(t), "nope")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "no entry") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestCiteCitedBy(t *testing.T) {
	code, stdout, stderr := run("cite", "cited-by", "--kb", citeKB(t), "bush1945")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "pages/a.md" {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestCiteCitedByUnknownKey(t *testing.T) {
	if code, _, _ := run("cite", "cited-by", "--kb", citeKB(t), "nope"); code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
}

func TestCiteCheckReportsProblems(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/a.md":     page("A", "type: concept", "cite [@absent]\n"),
		"bibliography.bib": "@article{dup,}\n" +
			"@book{dup,}\n" +
			"@article{one, doi = {10.1145/x}}\n" +
			"@article{two, doi = {10.1145/x}}\n",
	})
	code, stdout, stderr := run("cite", "check", "--kb", root)
	if code != ExitFindings {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitFindings, stderr)
	}
	for _, want := range []string{"defined more than once", "describe the same work", "no entry for"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}
}

func TestCiteCheckJSONCarriesFindings(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"bibliography.bib": "@article{dup,}\n@book{dup,}\n",
	})
	code, stdout, _ := run("cite", "check", "--kb", root, "--json")
	if code != ExitFindings {
		t.Fatalf("exit = %d, want %d", code, ExitFindings)
	}
	e := decodeJSON(t, stdout)
	codes := map[string]bool{}
	for _, f := range e.Findings {
		codes[f.Code] = true
	}
	if !codes[kb.CodeCitationDuplicate] {
		t.Errorf("findings = %+v", e.Findings)
	}
}

func TestCiteCheckClean(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/a.md":     page("A", "type: concept", "cite [@key]\n"),
		"bibliography.bib": "@article{key,\n" +
			"  stemma-retrieved = {2026-09-28},\n" +
			"  stemma-content-hash = {sha256:ab12},\n" +
			"}\n",
	})
	code, stdout, stderr := run("cite", "check", "--kb", root)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "clean") {
		t.Errorf("stdout = %q, want clean", stdout)
	}
}

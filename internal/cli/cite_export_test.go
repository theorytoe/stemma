package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCiteExportDefaultsToBibtex(t *testing.T) {
	code, stdout, stderr := run("cite", "export", "--kb", citeKB(t))
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"@article{bush1945", "@book{cormen2009"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}
}

func TestCiteExportCSLJSONIsJSON(t *testing.T) {
	code, stdout, stderr := run("cite", "export", "--kb", citeKB(t), "--format", "csl-json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(stdout), &items); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if len(items) != 2 {
		t.Errorf("items = %d, want 2", len(items))
	}
}

func TestCiteExportCitedOnly(t *testing.T) {
	code, stdout, stderr := run("cite", "export", "--kb", citeKB(t), "--cited")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "bush1945") {
		t.Errorf("the cited entry is missing:\n%s", stdout)
	}
	if strings.Contains(stdout, "cormen2009") {
		t.Errorf("an uncited entry was exported:\n%s", stdout)
	}
}

func TestCiteExportWritesAFile(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "out.bib")
	code, stdout, stderr := run("cite", "export", "--kb", citeKB(t), "-o", dest)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "@article{bush1945") {
		t.Errorf("the file does not hold the export:\n%s", data)
	}
	if !strings.Contains(stdout, dest) {
		t.Errorf("stdout does not say where it went: %q", stdout)
	}
}

func TestCiteExportRejectsAnUnknownFormat(t *testing.T) {
	code, _, stderr := run("cite", "export", "--kb", citeKB(t), "--format", "ris")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "bibtex") {
		t.Errorf("stderr does not name the formats it does write: %q", stderr)
	}
}

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The dump lands in the generated directory by default, like the built site, so
// neither artifact is something the author has to remember not to commit.
func TestExportJSONDefaultsIntoTheGeneratedDirectory(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("export", "json", "--kb", root)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "wrote") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".stemma", "export.json")); err != nil {
		t.Errorf("the dump is not in the generated directory: %v", err)
	}
}

// A relative --out is relative to the KB, the same rule `build` follows.
func TestExportJSONOutIsRelativeToTheKB(t *testing.T) {
	root := freshKB(t)
	code, _, stderr := run("export", "json", "--kb", root, "--out", "dump.json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "dump.json")); err != nil {
		t.Errorf("--out dump.json did not write into the KB: %v", err)
	}
}

// The dump is the artifact, so --out - puts it on standard output whole, with
// nothing written beside it.
func TestExportJSONToStdout(t *testing.T) {
	code, stdout, stderr := run("export", "json", "--kb", freshKB(t), "--out", "-")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var d struct {
		Version int `json:"version"`
		Counts  struct {
			Pages int `json:"pages"`
		} `json:"counts"`
	}
	if err := json.Unmarshal([]byte(stdout), &d); err != nil {
		t.Fatalf("standard output is not the dump on its own: %v\n%s", err, stdout)
	}
	if d.Version == 0 || d.Counts.Pages == 0 {
		t.Errorf("dump = %+v", d)
	}
}

// Writing to a file leaves room for a report, so --json carries the summary
// rather than the whole document inside another one.
func TestExportJSONEnvelope(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("export", "json", "--kb", root, "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var got struct {
		Out   string `json:"out"`
		Bytes int    `json:"bytes"`
		Pages int    `json:"pages"`
	}
	decodeData(t, stdout, &got)
	if got.Out == "" || got.Bytes == 0 || got.Pages == 0 {
		t.Errorf("data = %+v", got)
	}
}

func TestExportJSONTakesNoArguments(t *testing.T) {
	code, _, stderr := run("export", "json", "--kb", freshKB(t), "extra")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "takes no arguments") {
		t.Errorf("stderr = %q", stderr)
	}
}

// The extract lands in the generated directory by default, named after the page
// it is of, and it is a KB root that loads.
func TestExportPageDefaultsIntoTheGeneratedDirectory(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("export", "page", "--kb", root, "Demo")
	_ = stdout
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	// init names the KB "Demo" and its entry document after it, so the page is
	// found by the title the entry document carries.
	dir := filepath.Join(root, ".stemma", "extract", "index")
	if _, err := os.Stat(filepath.Join(dir, "stemma.toml")); err != nil {
		t.Fatalf("no extract at %s: %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pages", "index.md")); err != nil {
		t.Errorf("the extract has no entry document: %v", err)
	}
}

func TestExportPageOutIsRelativeToTheKB(t *testing.T) {
	root := freshKB(t)
	code, _, stderr := run("export", "page", "--kb", root, "Demo", "--out", "handoff")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "handoff", "stemma.toml")); err != nil {
		t.Errorf("--out handoff did not write into the KB: %v", err)
	}
}

// Depth is hops, "all" and 0 are the whole reachable set, and anything else that
// is not a number of hops is refused rather than guessed at.
func TestExportPageDepth(t *testing.T) {
	root := freshKB(t)
	for _, ok := range []string{"1", "2", "0", "all"} {
		if code, _, stderr := run("export", "page", "--kb", root, "Demo", "--depth", ok); code != ExitOK {
			t.Errorf("--depth %s: exit %d: %s", ok, code, stderr)
		}
	}
	for _, bad := range []string{"-1", "lots", "1.5"} {
		code, _, _ := run("export", "page", "--kb", root, "Demo", "--depth", bad)
		if code != ExitError {
			t.Errorf("--depth %s: exit %d, want %d", bad, code, ExitError)
		}
	}
}

// The JSON payload carries the pruned list, so a program does not have to ask
// twice for it.
func TestExportPageJSON(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("export", "page", "--kb", root, "Demo", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var got struct {
		Out     string   `json:"out"`
		Root    string   `json:"root"`
		Depth   int      `json:"depth"`
		Entry   string   `json:"entry"`
		Pages   []string `json:"pages"`
		Sources []string `json:"sources"`
		Pruned  []struct {
			Kind string `json:"kind"`
		} `json:"pruned"`
	}
	decodeData(t, stdout, &got)
	if got.Out == "" || got.Root == "" || got.Entry != "pages/index.md" {
		t.Errorf("data = %+v", got)
	}
	if len(got.Pages) == 0 {
		t.Error("the payload names no pages")
	}
	if got.Pruned == nil {
		t.Error("the payload carries no pruned list")
	}
}

func TestExportPageNeedsOnePage(t *testing.T) {
	root := freshKB(t)
	if code, _, _ := run("export", "page", "--kb", root); code != ExitError {
		t.Error("export page with no page was accepted")
	}
	if code, _, _ := run("export", "page", "--kb", root, "a", "b"); code != ExitError {
		t.Error("export page with two pages was accepted")
	}
}

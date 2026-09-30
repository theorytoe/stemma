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

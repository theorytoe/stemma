package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/theorytoe/stemma/internal/extract"
)

// A new KB should have its whole shape the moment it exists, so the extraction
// script is written by init rather than appearing the first time someone reads a
// document.
func TestInitWritesTheExtractionScript(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "kb")

	if code, _, stderr := run("init", dir); code != ExitOK {
		t.Fatalf("init exited %d: %s", code, stderr)
	}

	path := extract.ScriptPath(dir)
	if got := extract.ConditionOf(path); got != extract.Current {
		t.Errorf("the script at %s is %s after init, want current", path, got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".stemma")); err != nil {
		t.Errorf("init did not create the generated directory: %v", err)
	}
}

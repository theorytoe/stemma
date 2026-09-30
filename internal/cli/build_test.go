package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A build with no --out writes into the generated directory, which is
// gitignored and re-derivable, so building is never something the author has to
// remember not to commit.
func TestBuildDefaultsIntoTheGeneratedDirectory(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("build", "--kb", root)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "wrote") {
		t.Errorf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".stemma", "site", "index.html")); err != nil {
		t.Errorf("the site is not in the generated directory: %v", err)
	}
}

// A relative --out is relative to the KB, not to the working directory, so one
// invocation builds one directory no matter where it is typed.
func TestBuildOutIsRelativeToTheKB(t *testing.T) {
	root := freshKB(t)
	code, _, stderr := run("build", "--kb", root, "--out", "out")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "index.html")); err != nil {
		t.Errorf("--out out did not write into the KB: %v", err)
	}
}

func TestBuildJSON(t *testing.T) {
	root := freshKB(t)
	code, stdout, stderr := run("build", "--kb", root, "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var got struct {
		Dir     string   `json:"dir"`
		Written []string `json:"written"`
	}
	decodeData(t, stdout, &got)
	if got.Dir == "" {
		t.Error("the result names no directory")
	}
	for _, want := range []string{"index.html", "all.html", "assets/style.css"} {
		if !built(got.Written, want) {
			t.Errorf("written does not include %s: %v", want, got.Written)
		}
	}
}

func TestBuildTakesNoArguments(t *testing.T) {
	root := freshKB(t)
	code, _, stderr := run("build", "--kb", root, "extra")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "takes no arguments") {
		t.Errorf("stderr = %q", stderr)
	}
}

func built(urls []string, want string) bool {
	for _, u := range urls {
		if u == want {
			return true
		}
	}
	return false
}

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func page(title, extra, body string) string {
	s := "---\ntitle: " + title + "\n"
	if extra != "" {
		s += extra + "\n"
	}
	return s + "---\n" + body
}

// run runs one invocation and returns the code with both streams.
func run(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// envelope is the --json response shape. Tests decode the payload out of Data
// rather than reading the whole document, so the envelope is asserted in one
// place and a change to it is felt in one place.
type envelope struct {
	Command  string          `json:"command"`
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Findings []JSONFinding   `json:"findings"`
	Error    string          `json:"error"`
}

func decodeJSON(t *testing.T, out string) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return e
}

func decodeData(t *testing.T, out string, v any) envelope {
	t.Helper()
	e := decodeJSON(t, out)
	if len(e.Data) == 0 {
		t.Fatalf("the envelope carries no data:\n%s", out)
	}
	if err := json.Unmarshal(e.Data, v); err != nil {
		t.Fatalf("data is not %T: %v\n%s", v, err, out)
	}
	return e
}

func cleanKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", "start at [[A]]\n"),
		"pages/a.md":       page("A", "type: concept", "cites [@key]\n"),
		"bibliography.bib": "@article{key,}\n",
	})
}

func dirtyKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/a.md":     page("A", "type: nonsense", "broken [[Nothing]]\n"),
	})
}

func TestLintOnACleanKBExitsZero(t *testing.T) {
	code, stdout, stderr := run("lint", "--kb", cleanKB(t))
	if code != ExitOK {
		t.Errorf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, ExitOK, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "clean" {
		t.Errorf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
}

func TestLintWithFindingsExitsOne(t *testing.T) {
	code, stdout, stderr := run("lint", "--kb", dirtyKB(t))
	if code != ExitFindings {
		t.Errorf("exit = %d, want %d", code, ExitFindings)
	}
	if !strings.Contains(stdout, `pages/a.md:5: warning: no page is called "Nothing"`) {
		t.Errorf("stdout does not point at the link:\n%s", stdout)
	}
	if !strings.Contains(stderr, "findings") {
		t.Errorf("the summary is missing from stderr: %q", stderr)
	}
}

func TestLintJSONOutput(t *testing.T) {
	code, stdout, stderr := run("lint", "--kb", dirtyKB(t), "--json")
	if code != ExitFindings {
		t.Errorf("exit = %d, want %d", code, ExitFindings)
	}
	if stderr != "" {
		t.Errorf("stderr = %q: the JSON stream has to stay clean", stderr)
	}

	var summary struct {
		Strict   bool `json:"strict"`
		Clean    bool `json:"clean"`
		Warnings int  `json:"warnings"`
		Errors   int  `json:"errors"`
	}
	e := decodeData(t, stdout, &summary)
	if e.Command != "lint" {
		t.Errorf("command = %q", e.Command)
	}
	if e.OK {
		t.Error("a run with findings is marked ok")
	}
	if summary.Strict || summary.Clean {
		t.Errorf("summary = %+v", summary)
	}
	if summary.Warnings+summary.Errors != len(e.Findings) {
		t.Errorf("the counts do not match the findings: %+v", e)
	}
	if len(e.Findings) == 0 {
		t.Fatal("no findings")
	}
	for _, f := range e.Findings {
		if f.Code == "" || f.Path == "" || f.Message == "" {
			t.Errorf("a finding is missing a field: %+v", f)
		}
	}
}

// An empty result is an empty list, not null, so a consumer never has to
// special-case a clean KB.
func TestLintJSONOnACleanKB(t *testing.T) {
	code, stdout, _ := run("lint", "--kb", cleanKB(t), "--json")
	if code != ExitOK {
		t.Errorf("exit = %d, want %d", code, ExitOK)
	}
	var summary struct {
		Clean bool `json:"clean"`
	}
	e := decodeData(t, stdout, &summary)
	if !e.OK || !summary.Clean {
		t.Errorf("a clean run is not marked clean: %+v", e)
	}
	if len(e.Findings) != 0 {
		t.Errorf("a clean run reports findings: %+v", e.Findings)
	}
}

func TestStrictEscalatesWarnings(t *testing.T) {
	root := dirtyKB(t)

	_, lenientOut, _ := run("lint", "--kb", root, "--json")
	code, strictOut, _ := run("lint", "--kb", root, "--strict", "--json")
	if code != ExitFindings {
		t.Errorf("exit = %d, want %d", code, ExitFindings)
	}
	if !strings.Contains(lenientOut, `"warning"`) {
		t.Errorf("the lenient run has no warnings:\n%s", lenientOut)
	}
	if strings.Contains(strictOut, `"warning"`) {
		t.Errorf("the strict run still has warnings:\n%s", strictOut)
	}
	if !strings.Contains(strictOut, `"strict": true`) {
		t.Errorf("the strict run is not marked strict:\n%s", strictOut)
	}
}

func TestLintOnAnUnreadableKBExitsTwo(t *testing.T) {
	root := writeTree(t, map[string]string{"pages/a.md": "---\ntitle: A\n"})
	code, stdout, stderr := run("lint", "--kb", root)
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing: the run did not complete", stdout)
	}
	if !strings.Contains(stderr, "pages/a.md") {
		t.Errorf("stderr does not name the file: %q", stderr)
	}
}

func TestLintOnADirectoryThatIsNotAKBExitsTwo(t *testing.T) {
	code, _, stderr := run("lint", "--kb", t.TempDir())
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "not a KB root") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestLintRejectsExtraArguments(t *testing.T) {
	code, _, stderr := run("lint", "--kb", cleanKB(t), "surprise")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "takes no arguments") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestUnknownCommandExitsTwo(t *testing.T) {
	code, _, stderr := run("frobnicate")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestNoArgumentsShowsUsageAndExitsTwo(t *testing.T) {
	code, _, stderr := run()
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestHelpExitsZero(t *testing.T) {
	code, stdout, _ := run("help")
	if code != ExitOK {
		t.Errorf("exit = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout, "usage:") {
		t.Errorf("stdout = %q", stdout)
	}
}

// The KB root comes from the flag first, then the environment, then the
// working directory.
func TestDiscovery(t *testing.T) {
	root := cleanKB(t)

	t.Run("from the environment", func(t *testing.T) {
		t.Setenv(EnvKB, root)
		code, _, stderr := run("lint")
		if code != ExitOK {
			t.Errorf("exit = %d, want %d: %s", code, ExitOK, stderr)
		}
	})

	t.Run("by walking up", func(t *testing.T) {
		t.Setenv(EnvKB, "")
		t.Chdir(filepath.Join(root, "pages"))
		code, _, stderr := run("lint")
		if code != ExitOK {
			t.Errorf("exit = %d, want %d: %s", code, ExitOK, stderr)
		}
	})

	t.Run("a manifest beats a nearer pages directory", func(t *testing.T) {
		t.Setenv(EnvKB, "")
		// The nearer directory holds a pages/ of its own but no manifest, so
		// walking up has to keep going until it finds one. The page in the
		// nearer tree has no frontmatter, so choosing it would produce findings.
		root := writeTree(t, map[string]string{
			"stemma.toml":       "title = \"Root\"\n",
			"pages/index.md":    page("Index", "type: index", ""),
			"nested/pages/x.md": "no frontmatter here\n",
		})
		t.Chdir(filepath.Join(root, "nested"))
		code, stdout, stderr := run("lint")
		if code != ExitOK {
			t.Errorf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, ExitOK, stdout, stderr)
		}
	})

	t.Run("an explicit path beats the environment", func(t *testing.T) {
		t.Setenv(EnvKB, t.TempDir())
		code, _, stderr := run("lint", "--kb", root)
		if code != ExitOK {
			t.Errorf("exit = %d, want %d: %s", code, ExitOK, stderr)
		}
	})

	t.Run("nothing found", func(t *testing.T) {
		t.Setenv(EnvKB, "")
		t.Chdir(t.TempDir())
		code, _, stderr := run("lint")
		if code != ExitError {
			t.Errorf("exit = %d, want %d", code, ExitError)
		}
		if !strings.Contains(stderr, "no KB found") {
			t.Errorf("stderr = %q", stderr)
		}
	})

	t.Run("a path that is not a directory", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "thing")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, stderr := run("lint", "--kb", file)
		if code != ExitError {
			t.Errorf("exit = %d, want %d", code, ExitError)
		}
		if !strings.Contains(stderr, "not a directory") {
			t.Errorf("stderr = %q", stderr)
		}
	})
}

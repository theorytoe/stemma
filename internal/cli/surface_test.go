package cli

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopHelpListsEveryCommand(t *testing.T) {
	code, stdout, stderr := run("help")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, c := range commands {
		if !strings.Contains(stdout, c.name) {
			t.Errorf("help does not list %q:\n%s", c.name, stdout)
		}
	}
	if !strings.Contains(stdout, "usage:") {
		t.Errorf("help has no usage line:\n%s", stdout)
	}
}

func TestCommandHelpShowsItsFlags(t *testing.T) {
	code, stdout, stderr := run("help", "list")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"usage: stemma list", "--type", "--status", "--dir", "--tag", "--json", "--kb"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help for list is missing %q:\n%s", want, stdout)
		}
	}
}

// --help on a command is the same as asking help for it, and neither is an
// error.
func TestHelpFlagOnACommand(t *testing.T) {
	code, stdout, stderr := run("list", "--help")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, "usage: stemma list") {
		t.Errorf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q", stderr)
	}
}

// The reference is generated from the command table, so every command and the
// exit-code convention have to be in it without anyone maintaining a second
// list.
func TestReferenceMarkdownCoversTheSurface(t *testing.T) {
	code, stdout, stderr := run("help", "--markdown")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"## Exit codes", "## JSON output", "`--json`"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the reference is missing %q", want)
		}
	}
	for _, c := range commands {
		if !strings.Contains(stdout, "stemma "+c.name) {
			t.Errorf("the reference does not describe %q", c.name)
		}
	}
}

func TestNoCommandAndUnknownCommand(t *testing.T) {
	code, _, stderr := run()
	if code != ExitError || !strings.Contains(stderr, "no command") {
		t.Errorf("no arguments: exit %d, stderr %q", code, stderr)
	}
	code, _, stderr = run("frobnicate")
	if code != ExitError || !strings.Contains(stderr, "unknown command") {
		t.Errorf("unknown command: exit %d, stderr %q", code, stderr)
	}
}

// A noun family dispatches on its second argument and names a member without
// one. The real families arrive with later tasks; the mechanism is what this
// pins.
func TestNounFamilyDispatch(t *testing.T) {
	child := &command{
		name:    "json",
		summary: "dump the KB as JSON",
		setup: func(fs *flag.FlagSet, o *options) runFunc {
			return func(c *command, w *output, args []string) int {
				fmt.Fprintf(w.stdout, "ran %s with %v", c.name, args)
				return ExitOK
			}
		},
	}
	family := &command{name: "export", summary: "export the KB", sub: []*command{child}}
	set := []*command{family}

	c, name, rest, err := lookup(set, []string{"export", "json", "extra"})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := c.invoke(name, rest, &stdout, &stderr); code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr.String())
	}
	if stdout.String() != "ran json with [extra]" {
		t.Errorf("stdout = %q", stdout.String())
	}

	for _, args := range [][]string{{"export"}, {"export", "pdf"}} {
		if _, _, _, err := lookup(set, args); err == nil {
			t.Errorf("lookup(%q) was accepted", args)
		}
	}
}

// A --json failure still writes one parseable document, and nothing to stderr.
func TestJSONErrorEnvelope(t *testing.T) {
	code, stdout, stderr := run("show", "--kb", cleanKB(t), "nothing", "--json")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	e := decodeJSON(t, stdout)
	if e.OK || e.Error == "" {
		t.Errorf("envelope = %+v", e)
	}
	if !strings.Contains(e.Error, "no page") {
		t.Errorf("error = %q", e.Error)
	}
}

// Even a flag the command does not define produces the JSON envelope when
// --json was asked for, so a script never has to parse a human message.
// Every command writes the same envelope, with its own name in it, so a client
// can read one shape for the whole surface.
func TestEveryCommandEmitsAnEnvelope(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", "see [[Alpha]]\n"),
		"pages/alpha.md":   page("Alpha", "type: concept", ""),
		"inbox/draft.md":   "---\ntitle: A Draft\ntype: note\n---\n",
		"bibliography.bib": "@article{k,}\n",
	})

	// The mutations run in order, so each one acts on what the last one left.
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"new", []string{"--kb", root, "Beta", "--type", "note"}},
		{"list", []string{"--kb", root}},
		{"show", []string{"--kb", root, "Alpha"}},
		{"move", []string{"--kb", root, "Beta", "pages/sub"}},
		{"rename", []string{"--kb", root, "Alpha", "Renamed"}},
		{"archive", []string{"--kb", root, "Renamed", "--reason", "done"}},
		{"promote", []string{"--kb", root, "draft"}},
		{"status", []string{"--kb", root}},
		{"lint", []string{"--kb", root}},
		{"env", nil},
		{"help", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{tc.name, "--json"}, tc.args...)
			_, stdout, stderr := run(args...)
			if stderr != "" {
				t.Errorf("stderr = %q, want nothing under --json", stderr)
			}
			if e := decodeJSON(t, stdout); e.Command != tc.name {
				t.Errorf("command = %q, want %q", e.Command, tc.name)
			}
		})
	}

	// init makes its own root, so it does not fit the sequence above.
	initRoot := filepath.Join(t.TempDir(), "kb")
	_, stdout, _ := run("init", initRoot, "--title", "X", "--json")
	if e := decodeJSON(t, stdout); e.Command != "init" {
		t.Errorf("command = %q, want init", e.Command)
	}
}

func TestBadFlagUnderJSON(t *testing.T) {
	code, stdout, stderr := run("lint", "--kb", cleanKB(t), "--bogus", "--json")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	if e := decodeJSON(t, stdout); e.Error == "" {
		t.Errorf("envelope = %+v", e)
	}
}

func listTestKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"pages/a.md":       page("Alpha", "type: concept\nstatus: archived\ntags: [Machine-Learning]", ""),
		"pages/b.md":       page("Beta", "type: note", ""),
		"pages/deep/c.md":  page("Gamma", "type: concept\ntags: [machine learning]", ""),
		"bibliography.bib": "@article{k,}\n",
	})
}

func TestListFilters(t *testing.T) {
	root := listTestKB(t)
	for _, tc := range []struct {
		name string
		args []string
		want []string
	}{
		{"by type", []string{"--type", "concept"}, []string{"pages/a.md", "pages/deep/c.md"}},
		{"by status", []string{"--status", "archived"}, []string{"pages/a.md"}},
		{"by directory, relative to pages", []string{"--dir", "deep"}, []string{"pages/deep/c.md"}},
		{"by directory, named in full", []string{"--dir", "pages/deep"}, []string{"pages/deep/c.md"}},
		{"by tag, matched by its normalised form", []string{"--tag", "machine-learning"}, []string{"pages/a.md", "pages/deep/c.md"}},
		{"two tags are required together", []string{"--tag", "machine-learning", "--tag", "nope"}, nil},
		{"filters combine", []string{"--type", "note", "--status", "active"}, []string{"pages/b.md"}},
		{"nothing matches", []string{"--type", "wizard"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"list", "--kb", root, "--json"}, tc.args...)
			code, stdout, stderr := run(args...)
			if code != ExitOK {
				t.Fatalf("exit = %d: %s", code, stderr)
			}
			var got struct {
				Pages []struct {
					Path string `json:"path"`
				} `json:"pages"`
			}
			decodeData(t, stdout, &got)
			var paths []string
			for _, p := range got.Pages {
				paths = append(paths, p.Path)
			}
			if strings.Join(paths, ",") != strings.Join(tc.want, ",") {
				t.Errorf("paths = %q, want %q", paths, tc.want)
			}
		})
	}
}

func TestListRejectsAnImpossibleFilter(t *testing.T) {
	root := listTestKB(t)
	code, _, stderr := run("list", "--kb", root, "--status", "draft")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "active") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestStatusReport(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", "[[A]]\n"),
		"pages/a.md":       page("A", "type: concept", "[@k]\n"),
		"pages/orphan.md":  page("Orphan", "type: note", ""),
		"bibliography.bib": "@article{k,}\n@article{unused,}\n",
		"inbox/draft.md":   "---\ntitle: Draft\n---\n",
	})

	code, stdout, stderr := run("status", "--kb", root, "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var got struct {
		Pages   int            `json:"pages"`
		ByType  map[string]int `json:"by_type"`
		Orphans []string       `json:"orphans"`
		Sources int            `json:"sources"`
		Uncited []string       `json:"uncited_sources"`
		Drafts  int            `json:"drafts"`
		Index   struct {
			Present bool `json:"present"`
		} `json:"index"`
		Ambiguous []string `json:"ambiguous"`
	}
	decodeData(t, stdout, &got)
	if got.Pages != 3 {
		t.Errorf("pages = %d", got.Pages)
	}
	if got.ByType["concept"] != 1 || got.ByType["index"] != 1 || got.ByType["note"] != 1 {
		t.Errorf("by_type = %v", got.ByType)
	}
	if len(got.Orphans) != 1 || got.Orphans[0] != "pages/orphan.md" {
		t.Errorf("orphans = %v", got.Orphans)
	}
	if got.Sources != 2 || len(got.Uncited) != 1 || got.Uncited[0] != "unused" {
		t.Errorf("sources = %d, uncited = %v", got.Sources, got.Uncited)
	}
	if got.Drafts != 1 {
		t.Errorf("drafts = %d", got.Drafts)
	}
	if got.Index.Present {
		t.Error("an index that does not exist was reported present")
	}
	if len(got.Ambiguous) != 0 {
		t.Errorf("ambiguous = %v", got.Ambiguous)
	}
}

func TestPromoteMovesADraftAndResolvesLinks(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":          page("Index", "type: index", "see [[Half A Thought]]\n"),
		"inbox/half-a-thought.md": "---\ntitle: Half A Thought\ntype: concept\n---\nbody\n",
	})

	code, stdout, stderr := run("promote", "--kb", root, "half a thought", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var got struct {
		From          string `json:"from"`
		To            string `json:"to"`
		Title         string `json:"title"`
		Type          string `json:"type"`
		LinksResolved int    `json:"links_resolved"`
	}
	decodeData(t, stdout, &got)
	if got.From != "inbox/half-a-thought.md" || got.To != "pages/half-a-thought.md" {
		t.Errorf("got %+v", got)
	}
	if got.Title != "Half A Thought" || got.Type != "concept" || got.LinksResolved != 1 {
		t.Errorf("got %+v", got)
	}
	if _, err := os.Lstat(filepath.Join(root, "inbox", "half-a-thought.md")); !os.IsNotExist(err) {
		t.Error("the draft is still in the inbox")
	}
	// The link that pointed at the draft resolves now, so the KB is clean.
	if code, out, err := run("lint", "--kb", root, "--strict"); code != ExitOK {
		t.Errorf("the KB is not clean after promotion: exit %d\n%s\n%s", code, out, err)
	}
}

// Everything the inbox permitted is an error at promotion, and nothing is
// written when that happens.
func TestPromoteRefusesAnInvalidDraft(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"inbox/no-type.md": "---\ntitle: No Type\n---\n",
		"inbox/wizard.md":  "---\ntitle: Wizard\ntype: wizard\n---\n",
	})

	code, stdout, _ := run("promote", "--kb", root, "no type")
	if code != ExitFindings {
		t.Errorf("a draft with no type: exit = %d, want %d", code, ExitFindings)
	}
	if !strings.Contains(stdout, "type") {
		t.Errorf("the finding does not mention the type:\n%s", stdout)
	}

	code, stdout, _ = run("promote", "--kb", root, "wizard")
	if code != ExitFindings {
		t.Errorf("a draft with an unknown type: exit = %d, want %d", code, ExitFindings)
	}
	if !strings.Contains(stdout, "wizard") {
		t.Errorf("the finding does not mention the type:\n%s", stdout)
	}

	// Neither draft moved.
	for _, name := range []string{"inbox/no-type.md", "inbox/wizard.md"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Errorf("%s went missing: %v", name, err)
		}
	}
}

// --type is the explicit way to supply what a draft is missing.
func TestPromoteSuppliesAType(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":   page("Index", "type: index", ""),
		"inbox/thought.md": "---\ntitle: A Thought\n---\n",
	})
	code, _, stderr := run("promote", "--kb", root, "thought", "--type", "note")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	body, err := os.ReadFile(filepath.Join(root, "pages", "a-thought.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "type: note") {
		t.Errorf("the type was not written:\n%s", body)
	}
}

// Promoting a draft whose title another page already holds would make every
// link to that name ambiguous, so it is refused.
func TestPromoteRefusesANameCollision(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/alpha.md": page("Alpha", "type: concept", ""),
		"inbox/alpha.md": "---\ntitle: Alpha\ntype: concept\n---\n",
	})
	code, _, stderr := run("promote", "--kb", root, "inbox/alpha.md")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "already the name of") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestPromoteNeedsADraft(t *testing.T) {
	root := freshKB(t)
	code, _, stderr := run("promote", "--kb", root, "nothing")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "no draft") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestEnvReportsTheEnvironment(t *testing.T) {
	code, stdout, stderr := run("env")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"go", "python", "shim", "reads", "sqlite"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("env is missing %q:\n%s", want, stdout)
		}
	}

	code, stdout, _ = run("env", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	var got struct {
		Go struct {
			Version string `json:"version"`
			OS      string `json:"os"`
		} `json:"go"`
	}
	decodeData(t, stdout, &got)
	if got.Go.Version == "" || got.Go.OS == "" {
		t.Errorf("got %+v", got)
	}
}

func TestStatusText(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/a.md":     page("A", "type: concept", ""),
		"inbox/draft.md": "---\ntitle: Draft\n---\n",
	})
	code, stdout, stderr := run("status", "--kb", root)
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"pages", "orphans", "sources", "drafts", "index", "absent"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status text is missing %q:\n%s", want, stdout)
		}
	}
}

// A rename that cannot finish has to say how far it got. This is the failure
// mode of the most dangerous command, and the one a caller has to be able to
// recover from by running it again.
func TestRenameReportsPartialFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	root := writeTree(t, map[string]string{
		"pages/index.md":    page("Index", "type: index", "[[Alpha]]\n"),
		"pages/alpha.md":    page("Alpha", "type: concept", ""),
		"pages/sub/beta.md": page("Beta", "type: concept", "[[Alpha]]\n"),
	})
	sub := filepath.Join(root, "pages", "sub")
	if err := os.Chmod(sub, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(sub, 0o755) })

	code, _, stderr := run("rename", "--kb", root, "Alpha", "New Title")
	if code != ExitError {
		t.Fatalf("exit = %d, want %d", code, ExitError)
	}
	if !strings.Contains(stderr, "run the same command again") {
		t.Errorf("the failure does not say how to finish:\n%s", stderr)
	}
}

// P5: a plain directory works. No manifest, no git, no index, no setup.
func TestAPlainDirectoryWithNoManifestOrGit(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", "see [[Alpha]]\n"),
		"pages/alpha.md": page("Alpha", "type: concept", ""),
	})
	if _, err := os.Lstat(filepath.Join(root, "stemma.toml")); !os.IsNotExist(err) {
		t.Fatal("the fixture has a manifest")
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatal("the fixture is a git repository")
	}

	if code, out, err := run("lint", "--kb", root, "--strict"); code != ExitOK {
		t.Errorf("lint: exit %d\n%s\n%s", code, out, err)
	}
	if code, _, err := run("new", "--kb", root, "Beta", "--type", "note", "--json"); code != ExitOK {
		t.Errorf("new: exit %d: %s", code, err)
	}
	if code, _, err := run("status", "--kb", root, "--json"); code != ExitOK {
		t.Errorf("status: exit %d: %s", code, err)
	}
}

// A KB root that cannot be written still reads. A read-only root is a normal
// thing to find in CI, and every read command has to work there.
func TestAReadOnlyKBStillReads(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	root := cleanKB(t)
	pages := filepath.Join(root, "pages")
	if err := os.Chmod(pages, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(pages, 0o755) })

	if code, out, err := run("lint", "--kb", root); code != ExitOK {
		t.Errorf("lint on a read-only root: exit %d\n%s\n%s", code, out, err)
	}
	if code, _, _ := run("status", "--kb", root, "--json"); code != ExitOK {
		t.Error("status failed on a read-only root")
	}
	code, _, stderr := run("new", "--kb", root, "Blocked", "--type", "note")
	if code != ExitError {
		t.Errorf("new on a read-only root: exit = %d, want %d", code, ExitError)
	}
	if stderr == "" {
		t.Error("new failed without saying why")
	}
}

package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// bibDirKB is a KB whose bibliography uses the directory form: one file per
// entry, one of them cited by a page.
func bibDirKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", ""),
		"pages/a.md":     page("A", "type: concept", "see [@bush1945]\n"),
		"bibliography/bush1945.bib": "@article{bush1945,\n" +
			"  title = {As We May Think},\n" +
			"  author = {Bush, Vannevar},\n" +
			"  year = {1945},\n" +
			"}\n",
		"bibliography/cormen2009.bib": "@book{cormen2009,\n" +
			"  title = {Introduction to Algorithms},\n" +
			"  author = {Cormen, Thomas H.},\n" +
			"  year = {2009},\n" +
			"}\n",
	})
}

func dirFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func TestCiteListReadsTheDirectoryForm(t *testing.T) {
	code, stdout, stderr := run("cite", "list", "--kb", bibDirKB(t))
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"bush1945", "cormen2009"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}
}

func TestCiteShowNamesTheDefiningFile(t *testing.T) {
	code, stdout, stderr := run("cite", "show", "--kb", bibDirKB(t), "bush1945")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "bibliography/bush1945.bib") {
		t.Errorf("stdout does not name the entry's own file:\n%s", stdout)
	}
}

func TestCiteAddWritesANewFileInTheDirectoryForm(t *testing.T) {
	root := bibDirKB(t)
	cormenBefore, err := os.ReadFile(filepath.Join(root, "bibliography", "cormen2009.bib"))
	if err != nil {
		t.Fatal(err)
	}

	code, _, stderr := run("cite", "add", "--kb", root,
		"--title", "A New Work", "--author", "Roe, John", "--year", "2020")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	files := dirFiles(t, filepath.Join(root, "bibliography"))
	want := []string{"bush1945.bib", "cormen2009.bib", "roe2020new.bib"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Errorf("files = %q, want %q", files, want)
	}
	if f := readBib(t, root, "bibliography/roe2020new.bib"); !f.Has("roe2020new") {
		t.Errorf("the new entry is not in its file: %v", f.Entries())
	}
	cormenAfter, err := os.ReadFile(filepath.Join(root, "bibliography", "cormen2009.bib"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cormenBefore) != string(cormenAfter) {
		t.Error("adding an entry changed another entry's file")
	}
}

func TestCiteAddUpdatesAnEntryInItsOwnFile(t *testing.T) {
	root := bibDirKB(t)
	code, _, stderr := run("cite", "add", "--kb", root, "--key", "bush1945", "--title", "Changed")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	files := dirFiles(t, filepath.Join(root, "bibliography"))
	if len(files) != 2 {
		t.Errorf("files = %q, want the original two", files)
	}
	e, ok := readBib(t, root, "bibliography/bush1945.bib").Entry("bush1945")
	if !ok {
		t.Fatal("the entry is missing from its file")
	}
	if got, _ := e.Value("title"); got != "Changed" {
		t.Errorf("title = %q", got)
	}
}

func TestCiteAddWhenBothFormsExistWritesTheSingleFile(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":            page("Index", "type: index", ""),
		"bibliography.bib":          "@article{old,}\n",
		"bibliography/existing.bib": "@article{existing,}\n",
	})
	code, _, stderr := run("cite", "add", "--kb", root,
		"--title", "A New Work", "--author", "Roe, John", "--year", "2020")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}

	single := readBib(t, root, "bibliography.bib")
	if !single.Has("old") || !single.Has("roe2020new") {
		t.Errorf("the single file = %v", single.Entries())
	}
	if files := dirFiles(t, filepath.Join(root, "bibliography")); len(files) != 1 {
		t.Errorf("the directory changed: %q", files)
	}
}

func TestCiteCheckFindsADuplicateAcrossFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md":       page("Index", "type: index", ""),
		"bibliography/one.bib": "@article{dup,}\n",
		"bibliography/two.bib": "@book{dup,}\n",
	})
	code, stdout, stderr := run("cite", "check", "--kb", root)
	if code != ExitFindings {
		t.Fatalf("exit = %d, want %d\n%s", code, ExitFindings, stderr)
	}
	if !strings.Contains(stdout, "defined more than once") {
		t.Errorf("stdout = %q", stdout)
	}
}

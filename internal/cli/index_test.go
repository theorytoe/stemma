package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// indexStatus is the `status --json` view of the cache.
type indexStatus struct {
	Present bool `json:"present"`
	Fresh   bool `json:"fresh"`
}

func statusIndex(t *testing.T, root string) indexStatus {
	t.Helper()
	code, stdout, stderr := run("status", "--kb", root, "--json")
	if code != ExitOK {
		t.Fatalf("status: exit %d: %s", code, stderr)
	}
	var got struct {
		Index indexStatus `json:"index"`
	}
	decodeData(t, stdout, &got)
	return got.Index
}

// The whole loop: no index, build it, edit the KB, refresh, rebuild.
func TestIndexCommandBuildsRefreshesAndRebuilds(t *testing.T) {
	root := writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", "[[A]]\n"),
		"pages/a.md":     page("A", "type: concept", "alpha\n"),
	})

	if ind := statusIndex(t, root); ind.Present {
		t.Fatalf("an index that was never built is reported present: %+v", ind)
	}

	code, stdout, stderr := run("index", "--kb", root, "--json")
	if code != ExitOK {
		t.Fatalf("index: exit %d: %s", code, stderr)
	}
	var built struct {
		Mode  string `json:"mode"`
		Pages int    `json:"pages"`
		Added int    `json:"added"`
	}
	decodeData(t, stdout, &built)
	if built.Mode != "refresh" || built.Pages != 2 || built.Added != 2 {
		t.Errorf("the first build = %+v, want a refresh of 2 pages", built)
	}
	if ind := statusIndex(t, root); !ind.Present || !ind.Fresh {
		t.Errorf("after building: %+v, want present and fresh", ind)
	}

	// Edit a page: the index no longer matches, and status says so.
	edited := filepath.Join(root, "pages", "a.md")
	if err := os.WriteFile(edited, []byte(page("A", "type: concept", "beta\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if ind := statusIndex(t, root); !ind.Present || ind.Fresh {
		t.Errorf("after an edit: %+v, want present but stale", ind)
	}

	// A refresh rewrites the one page that changed.
	code, stdout, stderr = run("index", "--kb", root, "--json")
	if code != ExitOK {
		t.Fatalf("index: exit %d: %s", code, stderr)
	}
	var refreshed struct {
		Added   int `json:"added"`
		Updated int `json:"updated"`
		Removed int `json:"removed"`
	}
	decodeData(t, stdout, &refreshed)
	if refreshed.Updated != 1 || refreshed.Added != 0 || refreshed.Removed != 0 {
		t.Errorf("the refresh = %+v, want 1 updated", refreshed)
	}
	if ind := statusIndex(t, root); !ind.Fresh {
		t.Errorf("after refreshing: %+v, want fresh", ind)
	}

	// --rebuild discards and rebuilds, and says which it did.
	code, stdout, stderr = run("index", "--kb", root, "--rebuild", "--json")
	if code != ExitOK {
		t.Fatalf("index --rebuild: exit %d: %s", code, stderr)
	}
	var rebuilt struct {
		Mode    string `json:"mode"`
		Rebuilt bool   `json:"rebuilt"`
		Pages   int    `json:"pages"`
	}
	decodeData(t, stdout, &rebuilt)
	if rebuilt.Mode != "rebuild" || !rebuilt.Rebuilt || rebuilt.Pages != 2 {
		t.Errorf("the rebuild = %+v", rebuilt)
	}
}

func TestIndexRejectsArguments(t *testing.T) {
	code, _, stderr := run("index", "--kb", cleanKB(t), "extra")
	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if stderr == "" {
		t.Error("index accepted an argument without a word")
	}
}

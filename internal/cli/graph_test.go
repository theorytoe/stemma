package cli

import (
	"reflect"
	"strings"
	"testing"
)

// graphTestKB builds index -> alpha -> beta, with gamma isolated.
func graphTestKB(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"pages/index.md": page("Index", "type: index", "see [[Alpha]]\n"),
		"pages/alpha.md": page("Alpha", "type: concept", "then [[Beta]]\n"),
		"pages/beta.md":  page("Beta", "type: note", "the end\n"),
		"pages/gamma.md": page("Gamma", "type: concept", "alone\n"),
	})
}

type graphReport struct {
	Start     string `json:"start"`
	Depth     int    `json:"depth"`
	Tier      string `json:"tier"`
	Neighbors []struct {
		Path  string `json:"path"`
		Title string `json:"title"`
		Depth int    `json:"depth"`
	} `json:"neighbors"`
	Orphans  []string `json:"orphans"`
	DeadEnds []string `json:"dead_ends"`
}

func runGraph(t *testing.T, root string, args ...string) graphReport {
	t.Helper()
	full := append([]string{"graph", "--kb", root, "--json"}, args...)
	code, stdout, stderr := run(full...)
	if code != ExitOK {
		t.Fatalf("graph %v: exit %d: %s", args, code, stderr)
	}
	var got graphReport
	decodeData(t, stdout, &got)
	return got
}

func TestGraphWalksOutAndIn(t *testing.T) {
	root := graphTestKB(t)

	out := runGraph(t, root, "Alpha", "--out")
	if out.Start != "pages/alpha.md" || len(out.Neighbors) != 1 || out.Neighbors[0].Path != "pages/beta.md" {
		t.Errorf("out = %+v", out)
	}
	in := runGraph(t, root, "Beta", "--in")
	if len(in.Neighbors) != 1 || in.Neighbors[0].Path != "pages/alpha.md" {
		t.Errorf("in = %+v", in)
	}
	both := runGraph(t, root, "Alpha")
	if len(both.Neighbors) != 2 {
		t.Errorf("both directions = %+v", both)
	}
}

func TestGraphListsOrphansAndDeadEnds(t *testing.T) {
	root := graphTestKB(t)

	orphans := runGraph(t, root, "--orphans")
	if !reflect.DeepEqual(orphans.Orphans, []string{"pages/gamma.md"}) {
		t.Errorf("orphans = %v", orphans.Orphans)
	}
	dead := runGraph(t, root, "--dead-ends")
	if !reflect.DeepEqual(dead.DeadEnds, []string{"pages/beta.md", "pages/gamma.md"}) {
		t.Errorf("dead ends = %v", dead.DeadEnds)
	}
}

func TestGraphTextAndTier(t *testing.T) {
	root := graphTestKB(t)
	code, stdout, stderr := run("graph", "--kb", root, "Alpha", "--out")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "1\tpages/beta.md") {
		t.Errorf("text = %q", stdout)
	}

	// A fresh index changes the tier, not the walk.
	if code, _, stderr := run("index", "--kb", root, "--json"); code != ExitOK {
		t.Fatalf("index: exit %d: %s", code, stderr)
	}
	got := runGraph(t, root, "Alpha", "--out")
	if got.Tier != "index" || len(got.Neighbors) != 1 || got.Neighbors[0].Path != "pages/beta.md" {
		t.Errorf("indexed walk = %+v", got)
	}
}

func TestGraphRejectsBadInput(t *testing.T) {
	root := graphTestKB(t)
	for _, args := range [][]string{
		{"graph", "--kb", root},
		{"graph", "--kb", root, "Alpha", "Beta"},
		{"graph", "--kb", root, "--orphans", "Alpha"},
		{"graph", "--kb", root, "Nothing", "--out"},
		{"graph", "--kb", root, "Alpha", "--depth", "-1"},
	} {
		if code, _, _ := run(args...); code != ExitError {
			t.Errorf("%v: exit %d, want %d", args, code, ExitError)
		}
	}
}

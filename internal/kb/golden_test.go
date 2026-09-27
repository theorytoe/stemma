package kb

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGolden rewrites the golden files from what the tool currently does. It
// is how a deliberate change to serialisation is recorded, and it is the only
// way the files change: a test that rewrote its own expectations would prove
// nothing.
var updateGolden = flag.Bool("update", false, "rewrite the golden files from the current behaviour")

// repoRoot walks up from the working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test's working directory")
		}
		dir = parent
	}
}

// corpus is every page the preservation guarantee is checked against.
//
// It is two things on purpose. The first is a set of files written to be
// awkward: nested values, block scalars, comments in the middle of the block, a
// byte-order-mark-free CRLF file, a file with no final newline, a file with no
// frontmatter at all. The second is the project's own documentation, which is a
// KB written in the format the tool defines. If the format is bad, the
// documentation degrades first and degrades loudly, which is the point of
// pointing the harness at it rather than at a fixture alone.
func corpus(t *testing.T) []string {
	t.Helper()
	var paths []string

	awkward, err := filepath.Glob("testdata/roundtrip/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(awkward) == 0 {
		t.Fatal("the round-trip corpus is empty")
	}
	paths = append(paths, awkward...)

	wiki := filepath.Join(repoRoot(t), "wiki", "pages")
	err = filepath.WalkDir(wiki, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the example wiki: %v", err)
	}
	return paths
}

// TestGoldenRoundTrip is the preservation guarantee applied to a corpus: a page
// the tool has no reason to change comes back byte for byte, including its key
// order, its comments and its quoting style.
//
// This is why the page model keeps bytes and splices edits into them rather
// than re-encoding the block: re-encoding cannot promise this, and this is the
// promise the format is built on.
func TestGoldenRoundTrip(t *testing.T) {
	for _, path := range corpus(t) {
		name := strings.TrimPrefix(path, repoRoot(t)+string(filepath.Separator))
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			page, err := ParsePage(raw)
			if err != nil {
				t.Fatalf("the corpus does not parse: %v", err)
			}
			if got := page.Bytes(); !bytes.Equal(got, raw) {
				t.Errorf("round trip changed the page.\n--- got ---\n%q\n--- want ---\n%q", got, raw)
			}
		})
	}
}

// TestGoldenEdit is the other half: what the tool writes has to be recorded, so
// that a change in how a field is written is a visible diff rather than a
// surprise. Each input is edited the same way and compared against the file
// committed beside it.
func TestGoldenEdit(t *testing.T) {
	inputs, err := filepath.Glob("testdata/edit/*.md")
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no edit corpus: %v", err)
	}
	for _, input := range inputs {
		t.Run(filepath.Base(input), func(t *testing.T) {
			raw, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			page, err := ParsePage(raw)
			if err != nil {
				t.Fatalf("the input does not parse: %v", err)
			}
			hadFrontmatter := page.HasFrontmatter()
			if err := page.Set(FieldStatus, StatusArchived); err != nil {
				t.Fatalf("Set: %v", err)
			}
			if err := page.Set(FieldArchiveReason, "superseded"); err != nil {
				t.Fatalf("Set: %v", err)
			}
			got := page.Bytes()

			golden := strings.TrimSuffix(input, ".md") + ".golden"
			if *updateGolden {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("no golden file; run `go test ./internal/kb -update`: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output changed.\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}

			// And the edit stayed where it was put: every other line of the page
			// is the line that was there before. Writing two fields can append two
			// lines, or four when the page had no frontmatter block to append to.
			limit := 2
			if !hadFrontmatter {
				limit = 4
			}
			assertOnlyChangedLines(t, raw, got, limit)
		})
	}
}

// assertOnlyChangedLines checks that an edit touched one contiguous run of
// lines and left the rest alone. A golden file records what was written; this
// records that nothing else was.
func assertOnlyChangedLines(t *testing.T, before, after []byte, limit int) {
	t.Helper()
	oldLines := strings.Split(string(before), "\n")
	newLines := strings.Split(string(after), "\n")

	// Everything before the first difference.
	head := 0
	for head < len(oldLines) && head < len(newLines) && oldLines[head] == newLines[head] {
		head++
	}
	// Everything after the last difference.
	tail := 0
	for tail < len(oldLines)-head && tail < len(newLines)-head &&
		oldLines[len(oldLines)-1-tail] == newLines[len(newLines)-1-tail] {
		tail++
	}

	changed := (len(newLines) - head - tail) - (len(oldLines) - head - tail)
	if changed < 0 {
		changed = -changed
	}
	if changed > limit {
		t.Errorf("the edit changed %d lines, which is more than %d:\n%s", changed, limit, after)
	}
	if head == 0 && tail == 0 && changed == 0 && !bytes.Equal(before, after) {
		t.Error("the page changed without any line differing")
	}
}

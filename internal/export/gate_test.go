package export

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// exampleWiki loads the project's own documentation, which is the KB the
// definition of done is written about: a person clones the repository, runs the
// tool against this wiki, and hands a piece of it to someone else. It is read
// where it lies — an extraction reads and never writes — so the repository is
// left alone without being copied first.
func exampleWiki(t *testing.T) *kb.KB {
	t.Helper()
	dir := filepath.Join("..", "..", "wiki")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("the example wiki is not present: %v", err)
	}
	k, err := kb.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// TestEveryExtractStandsAlone is the `D30` gate.
//
// An extract has to be a KB root that lints clean under `--strict`, which is
// what makes it something a person can hand over rather than something that
// merely looks right. Every page at both depths is checked, not a sample: the
// case a sample would miss is exactly the interesting one — a page whose
// closure prunes a link, or materialises no sources, or holds the source's own
// entry document.
func TestEveryExtractStandsAlone(t *testing.T) {
	source := exampleWiki(t)
	paths := source.Graph.Paths()
	if len(paths) == 0 {
		t.Fatal("the example wiki has no pages")
	}

	for _, root := range paths {
		for _, depth := range []int{1, 0} {
			dir := filepath.Join(t.TempDir(), "extract")
			x, err := Scoped(source, root, depth)
			if err != nil {
				t.Fatalf("%s at depth %d: %v", root, depth, err)
			}
			if err := x.Write(dir); err != nil {
				t.Fatalf("%s at depth %d: %v", root, depth, err)
			}
			got := load(t, dir)

			if findings := got.Lint(kb.Strict); len(findings) != 0 {
				t.Errorf("the extract of %s at depth %d does not stand alone:", root, depth)
				for _, f := range findings {
					t.Errorf("  %s:%d %s: %s", f.Path, f.Line, f.Code, f.Message)
				}
				continue
			}

			// Said in the tool's own words as well, so that the gate states what
			// it means rather than only what lint happens to check today: every
			// link it kept resolves to exactly one page, and every citation it
			// kept names an entry the extract carries.
			for _, p := range got.Graph.Paths() {
				for _, l := range got.Graph.Links(p) {
					if res := got.Graph.Resolve(l.Name); res.Kind != kb.Resolved {
						t.Errorf("%s depth %d: %s:%d kept %q, which resolves to %d pages",
							root, depth, p, l.Line, l.Target, len(res.Matches))
					}
				}
				for _, c := range got.Graph.Citations(p) {
					if got.Bibliography == nil || !got.Bibliography.Has(c.Key) {
						t.Errorf("%s depth %d: %s:%d kept [@%s], which the extract does not define",
							root, depth, p, c.Line, c.Key)
					}
				}
			}
		}
	}
}

// TestAnExtractOfAnExtractIsItself is the round trip. An extract is a KB, so it
// can be extracted from again, and that second pass must reproduce it exactly.
// It is the strongest form of "stands alone" available here: the artifact is a
// fixed point of the operation that made it, so nothing about it depends on the
// tree it came from still being there.
func TestAnExtractOfAnExtractIsItself(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first")
	one, err := Scoped(exampleWiki(t), "pages/index.md", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := one.Write(first); err != nil {
		t.Fatal(err)
	}

	second := filepath.Join(t.TempDir(), "second")
	two, err := Scoped(load(t, first), "pages/index.md", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := two.Write(second); err != nil {
		t.Fatal(err)
	}

	want, got := content(t, first), content(t, second)
	if len(got) != len(want) {
		t.Fatalf("the second pass holds %v, the first %v", sortedNames(got), sortedNames(want))
	}
	for _, name := range sortedNames(want) {
		if !bytes.Equal(got[name], want[name]) {
			t.Errorf("%s changed on the second pass:\n--- first ---\n%s\n--- second ---\n%s", name, want[name], got[name])
		}
	}
}

// content is everything an extract holds that a reader reads. The manifest and
// the record in .stemma/ are left out on purpose: they say where the extract
// came from, and an extract of an extract came from somewhere else.
func content(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == kb.ManifestName || strings.HasPrefix(rel, kb.GeneratedDir+"/") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sortedNames(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

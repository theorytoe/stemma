package index

import (
	"reflect"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// graphKB builds a small link graph:
//
//	index -> alpha -> beta
//	gamma (isolated)
func graphKB(t *testing.T) (*kb.KB, *Store) {
	t.Helper()
	k := buildKB(t, map[string]string{
		"pages/index.md": pageFile("Index", "type: index", "see [[Alpha]]\n"),
		"pages/alpha.md": pageFile("Alpha", "type: concept\naliases: [First]", "then [[Beta]]\n"),
		"pages/beta.md":  pageFile("Beta", "type: note", "the end\n"),
		"pages/gamma.md": pageFile("Gamma", "type: concept", "alone\n"),
	})
	store := open(t, k.Root)
	if _, err := store.Populate(k); err != nil {
		t.Fatal(err)
	}
	return k, store
}

func neighborPaths(ns []Neighbor) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Path
	}
	return out
}

func TestNeighborsWalksBothWays(t *testing.T) {
	k, store := graphKB(t)
	for _, src := range []Source{newKBSource(k), newIndexSource(store)} {
		for _, tc := range []struct {
			name  string
			start string
			dirs  []Direction
			depth int
			want  []string
		}{
			{"out one hop", "pages/alpha.md", []Direction{Out}, 1, []string{"pages/beta.md"}},
			{"out two hops", "pages/index.md", []Direction{Out}, 2, []string{"pages/alpha.md", "pages/beta.md"}},
			{"in one hop", "pages/beta.md", []Direction{In}, 1, []string{"pages/alpha.md"}},
			{"in two hops", "pages/beta.md", []Direction{In}, 2, []string{"pages/alpha.md", "pages/index.md"}},
			{"both", "pages/alpha.md", []Direction{In, Out}, 1, []string{"pages/beta.md", "pages/index.md"}},
			{"unlimited", "pages/beta.md", []Direction{In}, 0, []string{"pages/alpha.md", "pages/index.md"}},
			{"isolated", "pages/gamma.md", []Direction{In, Out}, 3, nil},
		} {
			got, err := Neighbors(src, tc.start, tc.dirs, tc.depth)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(neighborPaths(got), nonNilStrings(tc.want)) {
				t.Errorf("tier %v, %s: got %v, want %v", src.Tier(), tc.name, neighborPaths(got), tc.want)
			}
		}
	}
}

func TestResolvePage(t *testing.T) {
	k, store := graphKB(t)
	for _, src := range []Source{newKBSource(k), newIndexSource(store)} {
		for _, tc := range []struct {
			name    string
			want    string
			wantErr bool
		}{
			{"Alpha", "pages/alpha.md", false},
			{"alpha", "pages/alpha.md", false},
			{"First", "pages/alpha.md", false},
			{"pages/alpha.md", "pages/alpha.md", false},
			{"nothing", "", true},
		} {
			got, err := ResolvePage(src, tc.name)
			if tc.wantErr {
				if err == nil {
					t.Errorf("tier %v, %q: no error", src.Tier(), tc.name)
				}
				continue
			}
			if err != nil || got != tc.want {
				t.Errorf("tier %v, %q = %q (%v), want %q", src.Tier(), tc.name, got, err, tc.want)
			}
		}
	}
}

func TestOrphansAndDeadEnds(t *testing.T) {
	k, store := graphKB(t)
	for _, src := range []Source{newKBSource(k), newIndexSource(store)} {
		orphans, err := Orphans(src)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(orphans, []string{"pages/gamma.md"}) {
			t.Errorf("tier %v: orphans = %v, want [pages/gamma.md]", src.Tier(), orphans)
		}
		dead, err := DeadEnds(src)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(dead, []string{"pages/beta.md", "pages/gamma.md"}) {
			t.Errorf("tier %v: dead ends = %v", src.Tier(), dead)
		}
	}
}

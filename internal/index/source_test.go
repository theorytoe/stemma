package index

import (
	"math"
	"reflect"
	"testing"
)

// The two tiers must answer every retrieval question the same way. This is the
// guard that keeps the index from quietly becoming the authority: if it ever
// disagrees with the KB, a query would depend on whether a cache happened to
// exist.
func TestSourcesAgreeOnTheSameCorpus(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/index.md":   pageFile("Index", "type: index", "see [[Alpha]] and cite [@key]\n"),
		"pages/alpha.md":   pageFile("Alpha", "type: concept\ntags: [Machine-Learning, Graphs]\naliases: [A]", "alpha body cites [@key] and links [[Beta]]\n"),
		"pages/beta.md":    pageFile("Beta", "type: note", "beta body, plus a naïve café\n"),
		"bibliography.bib": "@article{key,}\n",
	})
	store := open(t, k.Root)
	if _, err := store.Populate(k); err != nil {
		t.Fatal(err)
	}
	kbSrc := newKBSource(k)
	idxSrc := newIndexSource(store)

	pa, err := kbSrc.Pages()
	if err != nil {
		t.Fatal(err)
	}
	pb, err := idxSrc.Pages()
	if err != nil {
		t.Fatal(err)
	}
	if !sameSlice(pa, pb) {
		t.Errorf("Pages differ:\n kb  = %+v\n idx = %+v", pa, pb)
	}

	for _, path := range []string{"pages/index.md", "pages/alpha.md", "pages/beta.md"} {
		d0, err := kbSrc.Docs([]string{path})
		if err != nil {
			t.Fatal(err)
		}
		d1, err := idxSrc.Docs([]string{path})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(d0, d1) {
			t.Errorf("Docs(%s) differs:\n kb  = %+v\n idx = %+v", path, d0, d1)
		}

		l0, _ := kbSrc.Links(path)
		l1, err := idxSrc.Links(path)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(l0, l1) {
			t.Errorf("Links(%s) differ: %+v vs %+v", path, l0, l1)
		}

		b0, _ := kbSrc.Backlinks(path)
		b1, err := idxSrc.Backlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(b0, b1) {
			t.Errorf("Backlinks(%s) differ: %v vs %v", path, b0, b1)
		}

		c0, _ := kbSrc.Citations(path)
		c1, err := idxSrc.Citations(path)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(c0, c1) {
			t.Errorf("Citations(%s) differ: %+v vs %+v", path, c0, c1)
		}
	}

	// The batch forms are what a rule over the whole corpus reads, so they have to
	// agree too: a page with no links is absent from both, not empty in one.
	la, _ := kbSrc.LinksAll()
	lb, err := idxSrc.LinksAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(la, lb) {
		t.Errorf("LinksAll differs:\n kb  = %+v\n idx = %+v", la, lb)
	}
	ba, _ := kbSrc.BacklinksAll()
	bb, err := idxSrc.BacklinksAll()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ba, bb) {
		t.Errorf("BacklinksAll differs:\n kb  = %+v\n idx = %+v", ba, bb)
	}

	// A folded spelling is a hit on both tiers, because both tokenize in Go.
	for _, term := range []string{"alpha", "beta", "body", "machine", "cafe", "naive", "key", "absent"} {
		m0, _ := kbSrc.Match([]string{term})
		m1, err := idxSrc.Match([]string{term})
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(m0, m1) {
			t.Errorf("Match(%q) differs: %v vs %v", term, m0, m1)
		}
		df0, _ := kbSrc.DF(term)
		df1, err := idxSrc.DF(term)
		if err != nil {
			t.Fatal(err)
		}
		if df0 != df1 {
			t.Errorf("DF(%q) differs: %d vs %d", term, df0, df1)
		}
	}

	m0, _ := kbSrc.Match([]string{"alpha", "beta"})
	m1, err := idxSrc.Match([]string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if !sameSlice(m0, m1) {
		t.Errorf("a multi-term match differs: %v vs %v", m0, m1)
	}

	for _, name := range []string{"a", "alpha", "index", "beta", "nobody", ""} {
		c0, _ := kbSrc.Claimants(name)
		c1, err := idxSrc.Claimants(name)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(c0, c1) {
			t.Errorf("Claimants(%q) differs: %v vs %v", name, c0, c1)
		}
	}

	for _, key := range []string{"key", "absent"} {
		c0, _ := kbSrc.CitedBy(key)
		c1, err := idxSrc.CitedBy(key)
		if err != nil {
			t.Fatal(err)
		}
		if !sameSlice(c0, c1) {
			t.Errorf("CitedBy(%q) differs: %v vs %v", key, c0, c1)
		}
	}

	a0, _ := kbSrc.AvgLength()
	a1, err := idxSrc.AvgLength()
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(a0-a1) > 1e-9 {
		t.Errorf("average length differs: %v vs %v", a0, a1)
	}
}

// sameSlice compares two slices treating nil and empty as equal, which is the
// only way the two tiers differ: SQL and Go disagree about the zero value, not
// about the answer.
func sameSlice[T any](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

// A missing or stale index is not an error: the KB answers, once, without noise.
func TestNewSourceFallsBackSilentlyAndOnce(t *testing.T) {
	k := buildKB(t, map[string]string{
		"pages/a.md": pageFile("A", "type: concept", "alpha\n"),
	})

	var reasons []string
	note := func(reason string) { reasons = append(reasons, reason) }

	// No index yet: Tier 0, one line naming the reason.
	if src := NewSource(k, note); src.Tier() != TierKB {
		t.Errorf("tier = %v, want kb", src.Tier())
	}
	if !reflect.DeepEqual(reasons, []string{"absent"}) {
		t.Errorf("reasons = %v, want [absent]", reasons)
	}

	// A fresh index answers, and says nothing.
	store := open(t, k.Root)
	if _, err := store.Populate(k); err != nil {
		t.Fatal(err)
	}
	reasons = nil
	src := NewSource(k, note)
	if src.Tier() != TierIndex {
		t.Errorf("tier = %v, want index", src.Tier())
	}
	if len(reasons) != 0 {
		t.Errorf("a fresh index still logged %v", reasons)
	}
	if err := src.Close(); err != nil {
		t.Errorf("close: %v", err)
	}

	// A stale index: Tier 0 again, once.
	writeFile(t, k.Root, "pages/a.md", pageFile("A", "type: concept", "beta\n"))
	k2 := reload(t, k.Root)
	reasons = nil
	if src := NewSource(k2, note); src.Tier() != TierKB {
		t.Errorf("tier = %v, want kb", src.Tier())
	}
	if !reflect.DeepEqual(reasons, []string{"stale"}) {
		t.Errorf("reasons = %v, want [stale]", reasons)
	}
}

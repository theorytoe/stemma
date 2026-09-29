package index

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// synthKB writes a KB of n pages into dir and loads it.
//
// The shape is the one that costs: every page has prose, a tag, a citation and
// a link to the hub, and the hub therefore has a backlink from every page. It is
// deterministic, so two runs at the same size measure the same corpus.
func synthKB(tb testing.TB, dir string, n int) *kb.KB {
	tb.Helper()
	write := func(name, body string) {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	page := func(title, extra, body string) string {
		s := "---\ntitle: " + title + "\n"
		if extra != "" {
			s += extra + "\n"
		}
		return s + "---\n" + body
	}

	write("stemma.toml", "title = \"Synthetic\"\n")
	write("bibliography.bib", "@article{key,}\n")
	write("pages/hub.md", page("Hub", "type: index", "the hub page\n"))
	for i := 0; i < n; i++ {
		body := fmt.Sprintf(
			"page %d discusses retrieval and indexing and the graph of knowledge; it cites [@key] and links to [[Hub]] and [[Page %d]].\n",
			i, (i+1)%n)
		write(fmt.Sprintf("pages/page-%d.md", i),
			page(fmt.Sprintf("Page %d", i), "type: concept\ntags: [machine-learning]", body))
	}
	k, err := kb.Load(dir)
	if err != nil {
		tb.Fatalf("load synthetic KB: %v", err)
	}
	return k
}

// BenchmarkTiers measures the operations that matter at increasing page counts,
// on both tiers. It is a measurement, not a test: it passes whatever the
// numbers are. Run it with
//
//	make bench
//
// which uses one iteration per size, so the timings are one-shot wall times
// rather than averages.
func BenchmarkTiers(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("pages=%d", n), func(b *testing.B) {
			dir := b.TempDir()
			k := synthKB(b, dir, n)
			tier0 := newKBSource(k)

			store, err := Open(dir)
			if err != nil {
				b.Fatal(err)
			}
			defer store.Close()
			if _, err := store.Populate(k); err != nil {
				b.Fatal(err)
			}
			tier1 := newIndexSource(store)

			b.ReportAllocs()
			b.Run("tier0_load", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := kb.Load(dir); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("tier0_tokenize", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_ = newKBSource(k)
				}
			})
			b.Run("tier0_hashes", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := kb.Hashes(dir); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("tier1_check", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := Check(dir); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("tier0_command_match", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					fresh, err := kb.Load(dir)
					if err != nil {
						b.Fatal(err)
					}
					src := newKBSource(fresh)
					_, _ = src.Match([]string{"retrieval"})
				}
			})
			b.Run("tier1_command_match", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					if _, err := kb.Hashes(dir); err != nil {
						b.Fatal(err)
					}
					_, _ = tier1.Match([]string{"retrieval"})
				}
			})
			b.Run("tier0_lint", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_ = k.Lint(kb.Lenient)
				}
			})
			b.Run("tier0_resolve", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_ = k.Graph.Resolve("Page 500")
				}
			})
			b.Run("tier0_match", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = tier0.Match([]string{"retrieval"})
				}
			})
			b.Run("tier0_backlinks", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = tier0.Backlinks("pages/hub.md")
				}
			})
			b.Run("tier1_build", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = store.Populate(k)
				}
			})
			b.Run("tier1_refresh_noop", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = store.Refresh(k)
				}
			})
			b.Run("tier1_match", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = tier1.Match([]string{"retrieval"})
				}
			})
			b.Run("tier1_backlinks", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = tier1.Backlinks("pages/hub.md")
				}
			})
		})
	}
}

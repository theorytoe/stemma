package cli

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/theorytoe/stemma/internal/index"
	"github.com/theorytoe/stemma/internal/kb"
)

// StatusReport is the health summary. It is computed from what the KB holds
// rather than from anything generated, so it is correct on a fresh clone with
// no index at all.
type StatusReport struct {
	Root           string         `json:"root"`
	Title          string         `json:"title"`
	Pages          int            `json:"pages"`
	ByType         map[string]int `json:"by_type"`
	Orphans        []string       `json:"orphans"`
	Ambiguous      []string       `json:"ambiguous"`
	Sources        int            `json:"sources"`
	UncitedSources []string       `json:"uncited_sources"`
	Drafts         int            `json:"drafts"`
	Index          IndexState     `json:"index"`
}

// IndexState says whether the Tier-1 cache is there, and whether it is newer
// than the pages it was built from. It is deliberately name-agnostic: the index
// is whatever the tool put in the generated directory, and the question worth
// answering is whether anything has changed since.
type IndexState struct {
	Present bool `json:"present"`
	Fresh   bool `json:"fresh"`
}

// statusCommand implements `stemma status`.
//
// It answers "how is this KB doing" in one read-only pass, which is what an
// agent wants before deciding what to do next.
var statusCommand = &command{
	name:    "status",
	summary: "summarise the health of the KB",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("status takes no arguments, got %q", args[0]))
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}
			report, err := GatherStatus(k)
			if err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(report)
			}
			fmt.Fprint(w.stdout, report.text())
			return ExitOK
		}
	},
}

// GatherStatus surveys what the KB holds in one read-only pass: how many
// pages, of what types, what is orphaned, ambiguous, or uncited, and whether
// the Tier-1 index is there and current. It is computed from the KB rather
// than from anything generated, so it is correct on a fresh clone with no
// index at all. Nothing writes, and nothing requires an index.
func GatherStatus(k *kb.KB) (StatusReport, error) {
	drafts, err := k.DraftPaths()
	if err != nil {
		return StatusReport{}, err
	}

	report := StatusReport{
		Root:           k.Root,
		Title:          k.Manifest.Title,
		Pages:          k.Graph.Len(),
		ByType:         map[string]int{},
		Orphans:        NonNil(k.Graph.Orphans()),
		Ambiguous:      NonNil(ambiguousNames(k)),
		Drafts:         len(drafts),
		Index:          readIndexState(k),
		UncitedSources: []string{},
	}
	for _, p := range k.Graph.Paths() {
		if page, ok := k.Graph.Page(p); ok {
			report.ByType[page.Type()]++
		}
	}
	for _, key := range k.Bibliography.Keys() {
		report.Sources++
		if len(k.Graph.CitedBy(key)) == 0 {
			report.UncitedSources = append(report.UncitedSources, key)
		}
	}
	return report, nil
}

// ambiguousNames returns the names more than one page claims, sorted. Each is a
// latent name collision: every link to one is a hard error until it is
// repaired.
func ambiguousNames(k *kb.KB) []string {
	var out []string
	for _, name := range k.Graph.Names() {
		if len(k.Graph.Claimants(name)) > 1 {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (r StatusReport) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-10s%s\n", "kb", r.Title)
	fmt.Fprintf(&b, "%-10s%s\n", "root", r.Root)
	fmt.Fprintf(&b, "%-10s%d\n", "pages", r.Pages)
	types := make([]string, 0, len(r.ByType))
	for t := range r.ByType {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		fmt.Fprintf(&b, "%-10s%d %s\n", "", r.ByType[t], t)
	}
	fmt.Fprintf(&b, "%-10s%d\n", "orphans", len(r.Orphans))
	fmt.Fprintf(&b, "%-10s%d\n", "ambiguous", len(r.Ambiguous))
	fmt.Fprintf(&b, "%-10s%d (%d uncited)\n", "sources", r.Sources, len(r.UncitedSources))
	fmt.Fprintf(&b, "%-10s%d\n", "drafts", r.Drafts)
	switch {
	case !r.Index.Present:
		fmt.Fprintf(&b, "%-10s%s\n", "index", "absent")
	case r.Index.Fresh:
		fmt.Fprintf(&b, "%-10s%s\n", "index", "fresh")
	default:
		fmt.Fprintf(&b, "%-10s%s\n", "index", "stale")
	}
	return b.String()
}

// readIndexState reports whether this KB has an index, and whether the index
// still matches the KB it describes.
//
// The check compares the index's content stamps with the pages as loaded, so it
// answers the question a cache is for and never trusts a timestamp: a file
// touched without changing is not a change. A check that cannot run is not a
// freshness claim, so the index is reported absent rather than guessed at.
func readIndexState(k *kb.KB) IndexState {
	switch st, err := index.CheckKB(k); {
	case err != nil:
		return IndexState{}
	case st == index.Fresh:
		return IndexState{Present: true, Fresh: true}
	case st == index.Stale:
		return IndexState{Present: true}
	default:
		return IndexState{}
	}
}

// NonNil turns a nil slice into an empty one, so that --json always reports a
// list rather than null and a consumer never special-cases an empty KB.
func NonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

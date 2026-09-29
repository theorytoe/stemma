package cli

import (
	"flag"
	"fmt"

	"github.com/theorytoe/stemma/internal/index"
	"github.com/theorytoe/stemma/internal/kb"
)

// searchCommand implements `stemma search`.
//
// It answers from the index when one is fresh and from the KB otherwise, and a
// caller gets the same results either way. The tier is reported in --json and
// not announced in text, because falling back is normal rather than a problem.
var searchCommand = &command{
	name:    "search",
	summary: "search the pages, ranked",
	args:    "[QUERY]",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		typ := fs.String("type", "", "search only pages of this type")
		status := fs.String("status", "", "search only pages with this status")
		dir := fs.String("dir", "", "search only pages under this directory")
		tags := listFlag(fs, "tag", "search only pages carrying this tag; repeat to require several")
		inbox := fs.Bool("include-inbox", false, "also search inbox drafts")
		limit := fs.Int("limit", 0, "return at most this many results; 0 means no limit")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 1 {
				return w.fail(fmt.Errorf("search takes one query, got %d arguments", len(args)))
			}
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			if *status != "" && *status != kb.StatusActive && *status != kb.StatusArchived {
				return w.fail(fmt.Errorf("status is %q; it is %s or %s",
					*status, kb.StatusActive, kb.StatusArchived))
			}
			if *limit < 0 {
				return w.fail(fmt.Errorf("limit is %d; it is zero or more", *limit))
			}
			under, err := underDir(*dir)
			if err != nil {
				return w.fail(err)
			}

			root, err := discover(o.kb)
			if err != nil {
				return w.fail(err)
			}
			src, extra, err := searchSource(root, *inbox)
			if err != nil {
				return w.fail(err)
			}
			defer src.Close()

			hits, err := index.Search(src, extra, index.SearchRequest{
				Query:  query,
				Type:   *typ,
				Status: *status,
				Tags:   *tags,
				Dir:    under,
				Limit:  *limit,
			})
			if err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(map[string]any{
					"query":   query,
					"tier":    src.Tier().String(),
					"count":   len(hits),
					"results": hits,
				})
			}
			for _, h := range hits {
				fmt.Fprintf(w.stdout, "%s\t%.3f", h.Path, h.Score)
				switch {
				case h.Snippet != "":
					fmt.Fprintf(w.stdout, "\t%s", h.Snippet)
				case h.Title != "":
					fmt.Fprintf(w.stdout, "\t%s", h.Title)
				}
				fmt.Fprintln(w.stdout)
			}
			return ExitOK
		}
	},
}

// searchSource returns a source for the KB, plus the drafts to search alongside
// it when the caller asked for the inbox.
//
// The inbox path loads the KB, because reading drafts is a KB operation; the
// source may still be the index, since the load is only for the drafts. The
// plain path loads nothing when the index is fresh.
func searchSource(root string, includeInbox bool) (index.Source, []index.Document, error) {
	if !includeInbox {
		src, err := index.NewSourceAt(root, nil)
		return src, nil, err
	}

	k, err := kb.Load(root)
	if err != nil {
		return nil, nil, err
	}
	paths, err := k.DraftPaths()
	if err != nil {
		return nil, nil, err
	}
	extra := make([]index.Document, 0, len(paths))
	for _, p := range paths {
		page, err := k.ReadDraft(p)
		if err != nil {
			return nil, nil, err
		}
		extra = append(extra, index.NewDocument(
			p, page.Title(), page.Type(), page.Status(), page.Tags(), string(page.Body())))
	}
	return index.NewSource(k, nil), extra, nil
}

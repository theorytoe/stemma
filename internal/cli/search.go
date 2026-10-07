package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

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
			under, err := UnderDir(*dir)
			if err != nil {
				return w.fail(err)
			}

			root, err := discover(o.kb)
			if err != nil {
				return w.fail(err)
			}
			src, extra, err := SearchSource(root, *inbox)
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
			writeSearchResults(w.stdout, hits, index.Tokenize(query),
				colorEnabled(w.stdout), terminalWidth(w.stdout))
			return ExitOK
		}
	},
}

// The escapes a search row uses. They are the eight-colour set, which every
// terminal that claims to be one can render, and each is paired with a reset so
// the color cannot leak into the rest of the line.
const (
	ansiReset = "\x1b[0m"
	ansiPath  = "\x1b[36m"   // cyan
	ansiMatch = "\x1b[1;32m" // bold green
)

// minSnippetWidth is the least room a clipped snippet keeps when the terminal
// is too narrow for the columns before it. A few words of context beat none,
// even when the line then runs past the edge.
const minSnippetWidth = 20

// writeSearchResults prints one hit per line: the page path, its score, and the
// context around the match.
//
// The snippet arrives already collapsed to one line, because a result that
// wraps is a result a reader has to reassemble; only the path and score columns
// are padded, so the fields line up without a tab that a path of the wrong
// length would knock out of place.
//
// color and width describe the terminal the lines are going to. width is 0 off
// a terminal, and then nothing is clipped and no color is written, so a pipe
// receives the plain text it can split on whitespace, with the snippet's
// "[term]" marks left in place. On a terminal, lines are clipped so they cannot
// wrap, and color may be on or off: NO_COLOR turns off the color without giving
// up the clipping.
func writeSearchResults(out io.Writer, hits []index.Hit, terms []string, color bool, width int) {
	pathWidth, scoreWidth := 0, 0
	scores := make([]string, len(hits))
	for i, h := range hits {
		if n := utf8.RuneCountInString(h.Path); n > pathWidth {
			pathWidth = n
		}
		scores[i] = strconv.FormatFloat(h.Score, 'f', 3, 64)
		if n := len(scores[i]); n > scoreWidth {
			scoreWidth = n
		}
	}

	for i, h := range hits {
		text := h.Snippet
		if text == "" {
			text = h.Title
		}
		pad := strings.Repeat(" ", pathWidth-utf8.RuneCountInString(h.Path))

		// The two spaces between fields, plus the path and score as rendered,
		// come out of the terminal before the snippet gets the rest.
		budget := 0
		if width > 0 {
			budget = width - pathWidth - scoreWidth - 4
			if budget < minSnippetWidth {
				budget = minSnippetWidth
			}
		}
		snippet := renderSnippet(index.SplitSnippet(text, terms), color, budget)

		if color {
			fmt.Fprintf(out, "%s%s%s%s  %*s  %s\n",
				ansiPath, h.Path, ansiReset, pad, scoreWidth, scores[i], snippet)
			continue
		}
		fmt.Fprintf(out, "%s%s  %*s  %s\n", h.Path, pad, scoreWidth, scores[i], snippet)
	}
}

// renderSnippet renders a snippet's parts as one line.
//
// With color, a marked term is written in color and its marking brackets are
// dropped; without it, the brackets stay, because they are the plain-text
// convention a pipe and the site both read. A positive width clips the line to
// that many visible runes with a trailing ellipsis; the count is of what a
// reader sees, so the escapes do not eat into the budget and a bracket pair
// counts as the two columns it occupies.
func renderSnippet(parts []index.SnippetPart, color bool, width int) string {
	const ellipsis = "..."
	limit := -1
	if width > 0 {
		if limit = width - len(ellipsis); limit < 0 {
			limit = 0
		}
	}

	var b strings.Builder
	used, clipped := 0, false
	for _, part := range parts {
		visible := utf8.RuneCountInString(part.Text)
		display := part.Text
		switch {
		case part.Match && color:
			display = ansiMatch + part.Text + ansiReset
		case part.Match:
			display = "[" + part.Text + "]"
			visible += 2
		}

		if limit >= 0 && used+visible > limit {
			// A text run is cut to the room left; a marked term is kept or
			// dropped whole, because half a highlight is worse than none.
			if !part.Match {
				if room := limit - used; room > 0 {
					b.WriteString(runePrefix(part.Text, room))
				}
			}
			clipped = true
			break
		}
		b.WriteString(display)
		used += visible
	}
	if clipped {
		b.WriteString(ellipsis)
	}
	return b.String()
}

// runePrefix returns the first room runes of s, or s when it is no longer.
func runePrefix(s string, room int) string {
	rs := []rune(s)
	if len(rs) <= room {
		return s
	}
	return string(rs[:room])
}

// SearchSource returns a source for the KB, plus the drafts to search alongside
// it when the caller asked for the inbox.
//
// The inbox path loads the KB, because reading drafts is a KB operation; the
// source may still be the index, since the load is only for the drafts. The
// plain path loads nothing when the index is fresh.
func SearchSource(root string, includeInbox bool) (index.Source, []index.Document, error) {
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

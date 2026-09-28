package cli

import (
	"flag"
	"fmt"
	"path"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// listEntry is one page as --json reports it. The text form is path and title
// only, because that is what a person reads and a script splits; the JSON form
// carries everything the KB knows.
type listEntry struct {
	Path   string   `json:"path"`
	Title  string   `json:"title"`
	Type   string   `json:"type"`
	Status string   `json:"status"`
	Tags   []string `json:"tags,omitempty"`
}

// listCommand implements `stemma list`.
//
// The filters are orthogonal and combine: each one narrows the result, and a
// page has to satisfy all of them. A tag is compared by its normalised form, so
// the spelling an author used and the spelling a caller typed do not have to
// agree.
var listCommand = &command{
	name:    "list",
	summary: "list the pages",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		typ := fs.String("type", "", "list only pages of this type")
		status := fs.String("status", "", "list only pages with this status")
		dir := fs.String("dir", "", "list only pages under this directory")
		tags := listFlag(fs, "tag", "list only pages carrying this tag; repeat to require several")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("list takes no arguments, got %q", args[0]))
			}
			if *status != "" && *status != kb.StatusActive && *status != kb.StatusArchived {
				return w.fail(fmt.Errorf("status is %q; it is %s or %s",
					*status, kb.StatusActive, kb.StatusArchived))
			}
			under, err := underDir(*dir)
			if err != nil {
				return w.fail(err)
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}

			var entries []listEntry
			for _, p := range k.Graph.Paths() {
				page, ok := k.Graph.Page(p)
				if !ok {
					continue
				}
				if *typ != "" && page.Type() != *typ {
					continue
				}
				if *status != "" && page.Status() != *status {
					continue
				}
				if under != "" && p != under && !strings.HasPrefix(p, under+"/") {
					continue
				}
				if !carriesEvery(page, *tags) {
					continue
				}
				entries = append(entries, listEntry{
					Path:   p,
					Title:  page.Title(),
					Type:   page.Type(),
					Status: page.Status(),
					Tags:   page.Tags(),
				})
			}

			if w.json {
				if entries == nil {
					entries = []listEntry{}
				}
				return w.emit(map[string]any{
					"pages": entries,
					"count": len(entries),
				})
			}
			for _, e := range entries {
				fmt.Fprintf(w.stdout, "%s\t%s\n", e.Path, e.Title)
			}
			return ExitOK
		}
	},
}

// carriesEvery reports whether a page carries all the wanted tags.
func carriesEvery(page *kb.Page, wanted []string) bool {
	have := page.Tags()
	for _, want := range wanted {
		n := kb.Normalize(want)
		found := false
		for _, tag := range have {
			if kb.Normalize(tag) == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// underDir turns a --dir value into a KB-relative directory.
//
// A value is read relative to pages/ unless it already names pages/, so both
// "papers" and "pages/papers" mean the same thing and neither can walk outside
// the tree.
func underDir(destination string) (string, error) {
	cleaned := path.Clean(strings.TrimSpace(destination))
	switch {
	case cleaned == "" || cleaned == ".":
		return "", nil
	case path.IsAbs(cleaned), cleaned == "..", strings.HasPrefix(cleaned, "../"):
		return "", fmt.Errorf("%q is outside the KB", destination)
	case cleaned == kb.PagesDir:
		return cleaned, nil
	case strings.HasPrefix(cleaned, kb.PagesDir+"/"):
		return cleaned, nil
	default:
		return path.Join(kb.PagesDir, cleaned), nil
	}
}

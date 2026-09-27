package cli

import (
	"flag"
	"fmt"
	"io"
)

// listEntry is one page as --json reports it. The text form is path and title
// only, because that is what a person reads and a script splits; the JSON form
// carries everything the KB knows.
type listEntry struct {
	Path   string `json:"path"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// runList implements `stemma list`.
func runList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stemma list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	opts.register(fs)
	only := fs.String("type", "", "list only pages of this type")
	if err := parse(fs, args); err != nil {
		return ExitError
	}
	if fs.NArg() > 0 {
		return fail(stderr, fmt.Errorf("list takes no arguments, got %q", fs.Arg(0)))
	}

	k, code := opts.load(stderr)
	if k == nil {
		return code
	}

	var entries []listEntry
	for _, path := range k.Graph.Paths() {
		page, ok := k.Graph.Page(path)
		if !ok {
			continue
		}
		if *only != "" && page.Type() != *only {
			continue
		}
		entries = append(entries, listEntry{
			Path:   path,
			Title:  page.Title(),
			Type:   page.Type(),
			Status: page.Status(),
		})
	}

	if opts.json {
		if entries == nil {
			entries = []listEntry{}
		}
		return writeJSON(stdout, stderr, map[string]any{
			"pages": entries,
			"count": len(entries),
		})
	}
	for _, e := range entries {
		fmt.Fprintf(stdout, "%s\t%s\n", e.Path, e.Title)
	}
	return ExitOK
}

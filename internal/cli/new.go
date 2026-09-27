package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// runNew implements `stemma new`.
func runNew(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stemma new", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	opts.register(fs)
	typ := fs.String("type", "", "the page's type; the KB's default when not given")
	draft := fs.Bool("draft", false, "create a draft in the inbox instead of a page")
	if err := parse(fs, args); err != nil {
		return ExitError
	}
	if fs.NArg() != 1 {
		return fail(stderr, fmt.Errorf("new takes one title"))
	}
	title := strings.TrimSpace(fs.Arg(0))

	k, code := opts.load(stderr)
	if k == nil {
		return code
	}

	// A title that normalises to nothing cannot be linked to, so it cannot be a
	// page: identity is the title, and an empty name names everything.
	if kb.Normalize(title) == "" {
		return fail(stderr, fmt.Errorf("%q has no letters or digits in it, so nothing could link to it", title))
	}

	if !*draft {
		// Creating a second page with a name another page already answers to
		// would make every link to that name ambiguous, which the format calls
		// a hard error. Better to refuse now than to break the KB.
		if claimants := k.Graph.Claimants(title); len(claimants) > 0 {
			return fail(stderr, fmt.Errorf("%q is already the name of %s", title, strings.Join(claimants, " and ")))
		}
	}

	pageType := *typ
	if pageType == "" {
		pageType = k.Manifest.DefaultType
	}
	if !k.Vocabulary.Assignable(pageType) {
		if kb.IsReservedType(pageType) {
			return fail(stderr, fmt.Errorf("%q is a type the tool owns and does not assign", pageType))
		}
		return fail(stderr, fmt.Errorf("%q is not a type this KB knows; add it to %s",
			pageType, kb.ManifestName))
	}

	dir := kb.PagesDir
	if *draft {
		dir = kb.InboxDir
	}
	name := kb.PagePath(dir, title)

	page, err := kb.NewPage(title, pageType)
	if err != nil {
		return fail(stderr, err)
	}
	if err := k.CreatePage(name, page); err != nil {
		return fail(stderr, err)
	}

	if opts.json {
		return writeJSON(stdout, stderr, map[string]string{
			"path":  name,
			"title": title,
			"type":  pageType,
		})
	}
	fmt.Fprintln(stdout, name)
	return ExitOK
}

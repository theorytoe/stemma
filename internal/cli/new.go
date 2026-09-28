package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// newCommand implements `stemma new`.
var newCommand = &command{
	name:    "new",
	summary: "create a page, or a draft in the inbox",
	args:    "TITLE",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		typ := fs.String("type", "", "the page's type; the KB's default when not given")
		draft := fs.Bool("draft", false, "create a draft in the inbox instead of a page")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("new takes one title"))
			}
			title := strings.TrimSpace(args[0])

			k, code := o.load(w)
			if k == nil {
				return code
			}

			// A title that normalises to nothing cannot be linked to, so it
			// cannot be a page: identity is the title, and an empty name names
			// everything.
			if kb.Normalize(title) == "" {
				return w.fail(fmt.Errorf("%q has no letters or digits in it, so nothing could link to it", title))
			}

			if !*draft {
				// Creating a second page with a name another page already
				// answers to would make every link to that name ambiguous,
				// which the format calls a hard error. Better to refuse now
				// than to break the KB.
				if claimants := k.Graph.Claimants(title); len(claimants) > 0 {
					return w.fail(fmt.Errorf("%q is already the name of %s", title, strings.Join(claimants, " and ")))
				}
			}

			pageType := *typ
			if pageType == "" {
				pageType = k.Manifest.DefaultType
			}
			if !k.Vocabulary.Assignable(pageType) {
				if kb.IsReservedType(pageType) {
					return w.fail(fmt.Errorf("%q is a type the tool owns and does not assign", pageType))
				}
				return w.fail(fmt.Errorf("%q is not a type this KB knows; add it to %s",
					pageType, kb.ManifestName))
			}

			dir := kb.PagesDir
			if *draft {
				dir = kb.InboxDir
			}
			name := kb.PagePath(dir, title)

			page, err := kb.NewPage(title, pageType)
			if err != nil {
				return w.fail(err)
			}
			if err := k.CreatePage(name, page); err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(map[string]string{
					"path":  name,
					"title": title,
					"type":  pageType,
				})
			}
			fmt.Fprintln(w.stdout, name)
			return ExitOK
		}
	},
}

package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// NewReport is the payload of a --json new: where the page lives, and the
// title and type it carries. Under --strict with findings it names the page
// that was not written.
type NewReport struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

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
			// The reserved types are the tool's own and are never assigned, so
			// choosing one is a usage error rather than a fact about the page.
			if kb.IsReservedType(pageType) {
				return w.fail(fmt.Errorf("%q is a type the tool owns and does not assign", pageType))
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

			// A type the vocabulary does not know is a finding, not a usage
			// error: the format warns by default and errors under --strict
			// (D55). Under --strict the page is not written; by default it is,
			// with a warning, so the author can fix the type or add it to the
			// manifest. A draft is left alone, because the inbox is lenient and
			// promotion is where its type is held to the format.
			var findings []kb.Finding
			if !*draft {
				findings = page.Validate(k.Vocabulary, o.mode())
				// Validation leaves Path empty because a caller showing one page
				// already knows which it asked about. Here the page does not
				// exist yet, so name the file the finding is about.
				for i := range findings {
					findings[i].Path = name
				}
			}
			data := NewReport{Path: name, Title: title, Type: pageType}
			if o.strict && len(findings) > 0 {
				if w.json {
					return w.report(data, findings)
				}
				return reportText(w.stdout, w.stderr, findings)
			}

			if err := k.CreatePage(name, page); err != nil {
				return w.fail(err)
			}
			if w.json {
				return w.report(data, findings)
			}
			fmt.Fprintln(w.stdout, name)
			if len(findings) > 0 {
				return reportText(w.stdout, w.stderr, findings)
			}
			return ExitOK
		}
	},
}

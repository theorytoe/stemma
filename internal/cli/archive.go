package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// archiveCommand implements `stemma archive`.
//
// It records why. A reason is required rather than optional because the document
// is where the reasoning lives: history, where there is any, records that a page
// changed and when, and nothing else records why it was retired.
//
// The tool never deletes a page, so archiving is the closest thing to removal
// that exists. The reason is written to `archive_reason`, which is where the
// format keeps it; nothing is appended to the body, because the body is the
// author's.
var archiveCommand = &command{
	name:    "archive",
	summary: "archive a page, recording why",
	args:    "PAGE",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		reason := fs.String("reason", "", "why the page is being archived")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("archive takes one page name"))
			}
			if strings.TrimSpace(*reason) == "" {
				return w.fail(fmt.Errorf("archive records a reason: pass --reason"))
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}
			p, err := Resolve(k, args[0])
			if err != nil {
				return w.fail(err)
			}
			page, ok := k.Graph.Page(p)
			if !ok {
				return w.fail(fmt.Errorf("%s is not a page", p))
			}

			already := page.Status() == kb.StatusArchived
			if err := page.Set(kb.FieldStatus, kb.StatusArchived); err != nil {
				return w.fail(err)
			}
			if err := page.Set(kb.FieldArchiveReason, *reason); err != nil {
				return w.fail(err)
			}
			if err := k.WritePage(p, page); err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(map[string]any{
					"path":    p,
					"status":  kb.StatusArchived,
					"reason":  *reason,
					"changed": !already,
				})
			}
			if already {
				fmt.Fprintf(w.stdout, "%s was already archived; the reason is now %q\n", p, *reason)
				return ExitOK
			}
			fmt.Fprintf(w.stdout, "%s is archived: %s\n", p, *reason)
			return ExitOK
		}
	},
}

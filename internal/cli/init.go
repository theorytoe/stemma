package cli

import (
	"flag"
	"fmt"

	"github.com/theorytoe/stemma/internal/extract"
	"github.com/theorytoe/stemma/internal/kb"
)

// initCommand implements `stemma init`.
//
// It does not look for a KB, because it is creating one. The directory is given
// rather than discovered, and defaults to the working directory.
var initCommand = &command{
	name:    "init",
	summary: "create a KB root",
	args:    "[DIRECTORY]",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		title := fs.String("title", "", "the KB's title; its directory name when not given")
		return func(c *command, w *output, args []string) int {
			root := "."
			if len(args) > 1 {
				return w.fail(fmt.Errorf("init takes one directory, got %d", len(args)))
			}
			if len(args) == 1 {
				root = args[0]
			}

			if err := kb.Init(root, *title); err != nil {
				return w.fail(err)
			}

			// The extraction script is generated rather than authored, and a new KB
			// should have its whole shape at once, so init writes it too. It is
			// re-derived on use, so nothing downstream has to care whether this
			// particular copy survived.
			if _, err := extract.Materialize(root); err != nil {
				return w.fail(err)
			}

			// Read it back, so that what is reported is what was actually
			// written rather than what was intended.
			k, err := kb.Load(root)
			if err != nil {
				return w.fail(err)
			}
			if w.json {
				return w.emit(map[string]string{
					"root":  k.Root,
					"title": k.Manifest.Title,
				})
			}
			fmt.Fprintf(w.stdout, "%s: created a KB root titled %q\n", k.Root, k.Manifest.Title)
			return ExitOK
		}
	},
}

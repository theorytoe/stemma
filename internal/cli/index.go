package cli

import (
	"flag"
	"fmt"

	"github.com/theorytoe/stemma/internal/index"
)

// indexReport is the payload of `stemma index`. It embeds the build report so
// the counts keep the names they have in the library, and adds which run it was
// and where the cache landed.
type indexReport struct {
	index.Report
	Mode string `json:"mode"`
	Path string `json:"path"`
}

// indexCommand implements `stemma index`.
//
// It writes the Tier-1 cache. Nothing else requires it: every read command
// works without an index and falls back to the KB itself (D23). Building one is
// a deliberate act, which is why it is a command rather than something that
// happens as a side effect of a query.
var indexCommand = &command{
	name:    "index",
	summary: "build or refresh the Tier-1 search index",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		rebuild := fs.Bool("rebuild", false, "discard the index and build it from scratch")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("index takes no arguments, got %q", args[0]))
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}

			s, err := index.Open(k.Root)
			if err != nil {
				return w.fail(err)
			}
			defer s.Close()

			mode := "refresh"
			var report index.Report
			if *rebuild {
				mode = "rebuild"
				report, err = s.Populate(k)
			} else {
				report, err = s.Refresh(k)
			}
			if err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(indexReport{Report: report, Mode: mode, Path: s.Path()})
			}
			fmt.Fprintf(w.stdout, "%s: %s, %d pages (%d added, %d updated, %d removed)\n",
				s.Path(), mode, report.Pages, report.Added, report.Updated, report.Removed)
			return ExitOK
		}
	},
}

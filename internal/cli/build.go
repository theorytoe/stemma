package cli

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/theorytoe/stemma/internal/build"
	"github.com/theorytoe/stemma/internal/kb"
)

// buildCommand implements `stemma build`.
//
// It is the offline half of the renderer. The documents serve answers from are
// the ones written out, less the search page and the reload helper, which only
// a running server can provide. What is left is a directory of files that needs
// no server at all: it can be opened from disk, copied to a host, or handed to
// someone else, and its links still resolve because the renderer writes every
// one of them relative to the document that holds it.
var buildCommand = &command{
	name:    "build",
	summary: "write the KB as a static site",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		out := fs.String("out", "", "output directory, relative to the KB root (default .stemma/site)")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("build takes no arguments, got %q", args[0]))
			}
			root, err := discover(o.kb)
			if err != nil {
				return w.fail(err)
			}

			// A relative output is relative to the KB, not to wherever the
			// command was run, so one invocation builds one directory no matter
			// which directory it was typed in. The default is inside the
			// generated directory, which is gitignored and re-derivable, so a
			// build is never something the author has to remember not to commit.
			dir := *out
			switch {
			case dir == "":
				dir = filepath.Join(root, kb.GeneratedDir, "site")
			case !filepath.IsAbs(dir):
				dir = filepath.Join(root, dir)
			}

			res, err := build.Write(root, dir)
			if err != nil {
				return w.fail(err)
			}
			if w.json {
				return w.emit(res)
			}
			fmt.Fprintf(w.stdout, "wrote %d files to %s\n", len(res.Written), res.Dir)
			for _, u := range res.Removed {
				fmt.Fprintf(w.stdout, "removed %s\n", u)
			}
			return ExitOK
		}
	},
}

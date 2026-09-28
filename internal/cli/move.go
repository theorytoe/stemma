package cli

import (
	"flag"
	"fmt"
	"path"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// moveCommand implements `stemma move`.
//
// Directories inside pages/ are organisational and carry no meaning, which is
// the whole point of the layout decision: a move changes no link anywhere in
// the KB, so nothing has to be rewritten and nothing can be broken by moving a
// page. The filename does not change either, because it is not what a page is.
var moveCommand = &command{
	name:    "move",
	summary: "move a page to another directory",
	args:    "PAGE DIRECTORY",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 2 {
				return w.fail(fmt.Errorf("move takes a page and a destination directory"))
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}
			from, err := resolve(k, args[0])
			if err != nil {
				return w.fail(err)
			}

			dir, err := underPages(args[1])
			if err != nil {
				return w.fail(err)
			}
			to := path.Join(dir, path.Base(from))

			if to == from {
				// Nothing to do, and saying so is not the same as failing.
				if w.json {
					return w.emit(map[string]string{"from": from, "to": to})
				}
				fmt.Fprintf(w.stdout, "%s is already there\n", from)
				return ExitOK
			}
			if err := k.MovePage(from, to); err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(map[string]string{"from": from, "to": to})
			}
			fmt.Fprintf(w.stdout, "%s -> %s\n", from, to)
			return ExitOK
		}
	},
}

// underPages checks that a destination is inside pages/ and returns it cleaned.
//
// The check is the point: a destination is a path in the KB rather than a
// string to be joined onto one, so ".." cannot walk a page out of the tree it
// belongs to.
func underPages(destination string) (string, error) {
	cleaned := path.Clean(strings.TrimSpace(destination))
	if cleaned == "" || cleaned == "." {
		return kb.PagesDir, nil
	}
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%q is outside the KB", destination)
	}
	if cleaned == kb.PagesDir {
		return cleaned, nil
	}
	if !strings.HasPrefix(cleaned, kb.PagesDir+"/") {
		return "", fmt.Errorf("%q is not under %s/", destination, kb.PagesDir)
	}
	return cleaned, nil
}

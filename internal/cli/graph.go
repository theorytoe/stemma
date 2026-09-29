package cli

import (
	"flag"
	"fmt"

	"github.com/theorytoe/stemma/internal/index"
)

// graphCommand implements `stemma graph`.
//
// It is the traversal view of what lint reports as facts: backlinks are folded
// in as --in rather than getting a verb of their own, and orphans and dead ends
// are the same findings seen from the graph rather than from the linter.
var graphCommand = &command{
	name:    "graph",
	summary: "walk the link graph",
	args:    "[PAGE]",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		in := fs.Bool("in", false, "follow backlinks")
		out := fs.Bool("out", false, "follow forward links")
		depth := fs.Int("depth", 1, "how many hops to walk; 0 means the whole reachable set")
		orphans := fs.Bool("orphans", false, "list pages nothing links to")
		deadEnds := fs.Bool("dead-ends", false, "list pages with no outgoing links")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			listing := *orphans || *deadEnds
			switch {
			case listing && len(args) > 0:
				return w.fail(fmt.Errorf("graph --orphans and --dead-ends take no page, got %q", args[0]))
			case !listing && len(args) == 0:
				return w.fail(fmt.Errorf("graph needs a page, or --orphans or --dead-ends"))
			case len(args) > 1:
				return w.fail(fmt.Errorf("graph takes one page, got %d", len(args)))
			case *depth < 0:
				return w.fail(fmt.Errorf("depth is %d; it is zero or more", *depth))
			}

			root, err := discover(o.kb)
			if err != nil {
				return w.fail(err)
			}
			src, err := index.NewSourceAt(root, nil)
			if err != nil {
				return w.fail(err)
			}
			defer src.Close()

			if listing {
				return graphListings(src, w, *orphans, *deadEnds)
			}

			dirs := make([]index.Direction, 0, 2)
			if *in {
				dirs = append(dirs, index.In)
			}
			if *out {
				dirs = append(dirs, index.Out)
			}
			if len(dirs) == 0 {
				dirs = []index.Direction{index.In, index.Out}
			}

			start, err := index.ResolvePage(src, args[0])
			if err != nil {
				return w.fail(err)
			}
			neighbors, err := index.Neighbors(src, start, dirs, *depth)
			if err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(map[string]any{
					"start":     start,
					"depth":     *depth,
					"tier":      src.Tier().String(),
					"neighbors": nonNil(neighbors),
				})
			}
			for _, n := range neighbors {
				fmt.Fprintf(w.stdout, "%d\t%s\n", n.Depth, n.Path)
			}
			return ExitOK
		}
	},
}

// graphListings prints the pages that are orphans, dead ends, or both.
func graphListings(src index.Source, w *output, orphans, deadEnds bool) int {
	var o, d []string
	var err error
	if orphans {
		if o, err = index.Orphans(src); err != nil {
			return w.fail(err)
		}
	}
	if deadEnds {
		if d, err = index.DeadEnds(src); err != nil {
			return w.fail(err)
		}
	}

	if w.json {
		data := map[string]any{"tier": src.Tier().String()}
		if orphans {
			data["orphans"] = nonNil(o)
		}
		if deadEnds {
			data["dead_ends"] = nonNil(d)
		}
		return w.emit(data)
	}
	for _, p := range o {
		fmt.Fprintln(w.stdout, p)
	}
	for _, p := range d {
		fmt.Fprintln(w.stdout, p)
	}
	return ExitOK
}

package cli

import (
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/theorytoe/stemma/internal/export"
)

// exportCommand is the family for handing the KB to something that is not this
// tool. It is a family rather than one verb because its members answer different
// questions: `json` is a dump for a program, and `page` will be a KB for another
// person.
var exportCommand = &command{
	name:    "export",
	summary: "hand the KB to another tool",
	sub: []*command{
		exportJSONCommand,
		exportPageCommand,
	},
}

// exportJSONCommand implements `stemma export json`.
//
// It is the machine-readable half of the export family (`D33`): one JSON
// document holding the pages, the links, the citations and the bibliography,
// which is enough to rebuild the graph without the KB tree.
//
// The document is the artifact, so it goes to a file and the command reports
// what it wrote. "--out -" puts it on standard output instead, and there is then
// no room for a report beside it, which is why that form writes nothing else.
var exportJSONCommand = &command{
	name:    "json",
	summary: "dump pages, links, citations and the bibliography as JSON",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		out := fs.String("out", "", "write here, relative to the KB (default .stemma/export.json); - for standard output")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("export json takes no arguments, got %q", args[0]))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}

			if *out == "-" {
				b, err := export.Assemble(k).Bytes()
				if err != nil {
					return w.fail(err)
				}
				if _, err := w.stdout.Write(b); err != nil {
					return w.fail(err)
				}
				return ExitOK
			}

			res, err := export.Write(k, outputPath(k.Root, *out, "export.json"))
			if err != nil {
				return w.fail(err)
			}
			if w.json {
				return w.emit(res)
			}
			fmt.Fprintf(w.stdout, "wrote %d pages, %d links, %d citations and %d entries to %s\n",
				res.Pages, res.Links, res.Citations, res.Bibliography, res.Out)
			return ExitOK
		}
	},
}

// exportPageCommand implements `stemma export page`.
//
// It is the half of the export family that produces a KB rather than a dump
// (`D27`): one page, the pages it links to, and the sources they cite, written
// as a KB root someone else can read, lint, search and build. The shape is in
// FORMAT.md under "The scoped extract".
var exportPageCommand = &command{
	name:    "page",
	summary: "extract one page and the pages it links to, as a KB",
	args:    "PAGE",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		depth := fs.String("depth", "", "how many hops to follow; 0 or \"all\" for the whole reachable set (default: the manifest)")
		out := fs.String("out", "", "write the extract here, relative to the KB (default .stemma/extract/<page>)")
		report := fs.Bool("report-pruned", false, "list every pruned link and citation, not only the counts")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("export page needs one page, got %d", len(args)))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}
			root, err := resolve(k, args[0])
			if err != nil {
				return w.fail(err)
			}
			hops, err := parseDepth(*depth, k.Manifest.Export.DefaultDepth)
			if err != nil {
				return w.fail(err)
			}
			x, err := export.Scoped(k, root, hops)
			if err != nil {
				return w.fail(err)
			}

			name := filepath.Join("extract", strings.TrimSuffix(path.Base(root), ".md"))
			if err := x.Write(outputPath(k.Root, *out, name)); err != nil {
				return w.fail(err)
			}
			if w.json {
				return w.emit(x)
			}
			printExtract(w, x, *report)
			return ExitOK
		}
	},
}

// parseDepth reads the --depth flag: a number of hops, or "all" for the whole
// reachable set. Nothing at all means the manifest's default (`D29`), and 0 and
// "all" are the same request, because the manifest already spells "no limit" as
// 0.
func parseDepth(flag string, def int) (int, error) {
	switch s := strings.TrimSpace(flag); s {
	case "":
		return def, nil
	case "all":
		return 0, nil
	default:
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("depth is %q; it is a number of hops, 0, or \"all\"", flag)
		}
		return n, nil
	}
}

// printExtract is what a person reads: what was written, where to start, and how
// much had to be left behind. --report-pruned lists the constructs one by one,
// which is the detail `D51` says is worth asking for and not worth printing
// every time.
func printExtract(w *output, x *export.Extract, report bool) {
	links, cites := 0, 0
	for _, p := range x.Pruned {
		if p.Kind == "link" {
			links++
		} else {
			cites++
		}
	}
	fmt.Fprintf(w.stdout, "wrote %d pages and %d sources to %s\n", len(x.Pages), len(x.Sources), x.Out)
	fmt.Fprintf(w.stdout, "start at %s\n", filepath.Join(x.Out, filepath.FromSlash(x.Entry)))
	if len(x.Pruned) == 0 {
		return
	}
	fmt.Fprintf(w.stdout, "pruned %d links and %d citations that could not come along\n", links, cites)
	if !report {
		fmt.Fprintf(w.stdout, "  pass --report-pruned to list them\n")
		return
	}
	for _, p := range x.Pruned {
		fmt.Fprintf(w.stdout, "  %s:%d %s %q (%s) became %q\n", p.Page, p.Line, p.Kind, p.Target, p.Reason, p.Became)
	}
}

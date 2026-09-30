package cli

import (
	"flag"
	"fmt"

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

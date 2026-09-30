package cli

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/theorytoe/stemma/internal/serve"
)

// serveCommand implements `stemma serve`.
//
// It is the local entry point of the renderer: the same documents `build`
// writes, plus server-side search and a reload helper. It runs until stopped,
// so it returns only on failure.
var serveCommand = &command{
	name:    "serve",
	summary: "serve the KB as a local site",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		addr := fs.String("addr", "127.0.0.1:8080", "address to listen on")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("serve takes no arguments, got %q", args[0]))
			}
			root, err := discover(o.kb)
			if err != nil {
				return w.fail(err)
			}
			srv, err := serve.New(root)
			if err != nil {
				return w.fail(err)
			}
			fmt.Fprintf(w.stdout, "serving %s at http://%s\n", root, *addr)
			if err := http.ListenAndServe(*addr, srv); err != nil {
				return w.fail(err)
			}
			return ExitOK
		}
	},
}

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/theorytoe/stemma/internal/kb"
)

// runInit implements `stemma init`.
//
// It does not look for a KB, because it is creating one. The directory is given
// rather than discovered, and defaults to the working directory.
func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stemma init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	title := fs.String("title", "", "the KB's title; its directory name when not given")
	asJSON := fs.Bool("json", false, "write output as JSON")
	if err := parse(fs, args); err != nil {
		return ExitError
	}

	root := "."
	if fs.NArg() > 1 {
		return fail(stderr, fmt.Errorf("init takes one directory, got %d", fs.NArg()))
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	if err := kb.Init(root, *title); err != nil {
		return fail(stderr, err)
	}

	// Read it back, so that what is reported is what was actually written
	// rather than what was intended.
	k, err := kb.Load(root)
	if err != nil {
		return fail(stderr, err)
	}
	if *asJSON {
		return writeJSON(stdout, stderr, map[string]string{
			"root":  k.Root,
			"title": k.Manifest.Title,
		})
	}
	fmt.Fprintf(stdout, "%s: created a KB root titled %q\n", k.Root, k.Manifest.Title)
	return ExitOK
}

// writeJSON is how every command writes --json output: indented, on stdout, and
// with nothing else mixed in so that the stream stays parseable.
func writeJSON(stdout, stderr io.Writer, value any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return fail(stderr, err)
	}
	return ExitOK
}

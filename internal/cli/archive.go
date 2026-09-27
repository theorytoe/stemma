package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// runArchive implements `stemma archive`.
//
// It records why. A reason is required rather than optional because the document
// is where the reasoning lives: history, where there is any, records that a page
// changed and when, and nothing else records why it was retired.
//
// The tool never deletes a page, so archiving is the closest thing to removal
// that exists.
func runArchive(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stemma archive", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	opts.register(fs)
	reason := fs.String("reason", "", "why the page is being archived")
	if err := parse(fs, args); err != nil {
		return ExitError
	}
	if fs.NArg() != 1 {
		return fail(stderr, fmt.Errorf("archive takes one page name"))
	}
	if strings.TrimSpace(*reason) == "" {
		return fail(stderr, fmt.Errorf("archive records a reason: pass --reason"))
	}

	k, code := opts.load(stderr)
	if k == nil {
		return code
	}
	path, err := resolve(k, fs.Arg(0))
	if err != nil {
		return fail(stderr, err)
	}
	page, ok := k.Graph.Page(path)
	if !ok {
		return fail(stderr, fmt.Errorf("%s is not a page", path))
	}

	if err := page.Set(kb.FieldStatus, kb.StatusArchived); err != nil {
		return fail(stderr, err)
	}
	if err := page.Set(kb.FieldArchiveReason, *reason); err != nil {
		return fail(stderr, err)
	}
	if err := k.WritePage(path, page); err != nil {
		return fail(stderr, err)
	}

	if opts.json {
		return writeJSON(stdout, stderr, map[string]string{
			"path":   path,
			"status": kb.StatusArchived,
			"reason": *reason,
		})
	}
	fmt.Fprintf(stdout, "%s is archived\n", path)
	return ExitOK
}

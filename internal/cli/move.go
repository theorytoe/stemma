package cli

import (
	"flag"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// runMove implements `stemma move`.
//
// Directories inside pages/ are organisational and carry no meaning, which is
// the whole point of the layout decision: a move changes no link anywhere in
// the KB, so nothing has to be rewritten and nothing can be broken by moving a
// page. The filename does not change either, because it is not what a page is.
func runMove(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stemma move", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	opts.register(fs)
	if err := parse(fs, args); err != nil {
		return ExitError
	}
	if fs.NArg() != 2 {
		return fail(stderr, fmt.Errorf("move takes a page and a destination directory"))
	}
	name, destination := fs.Arg(0), fs.Arg(1)

	k, code := opts.load(stderr)
	if k == nil {
		return code
	}
	from, err := resolve(k, name)
	if err != nil {
		return fail(stderr, err)
	}

	dir, err := underPages(destination)
	if err != nil {
		return fail(stderr, err)
	}
	to := path.Join(dir, path.Base(from))

	if to == from {
		// Nothing to do, and saying so is not the same as failing.
		if opts.json {
			return writeJSON(stdout, stderr, map[string]string{"from": from, "to": to})
		}
		fmt.Fprintf(stdout, "%s is already there\n", from)
		return ExitOK
	}
	if err := k.MovePage(from, to); err != nil {
		return fail(stderr, err)
	}

	if opts.json {
		return writeJSON(stdout, stderr, map[string]string{"from": from, "to": to})
	}
	fmt.Fprintf(stdout, "%s -> %s\n", from, to)
	return ExitOK
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

package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/theorytoe/stemma/internal/extract"
	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/source"
)

// pdfHeader is what a PDF starts with. It is checked here as well as in the shim
// because the two are answering different questions: this one decides which
// subcommand to run, and the shim decides whether the document is acceptable.
const pdfHeader = "%PDF-"

// newRunner builds what `fetch` reads through. It is a variable so that a test can
// point extraction at a stand-in script, which is how every path through this
// command stays testable on a machine with no Python.
var newRunner = func(root string) (*extract.Runner, error) {
	// The script is generated, so it is re-established rather than assumed: a
	// cleared .stemma/, an upgrade, and a KB received by clone all have to leave
	// this working.
	if _, err := extract.Materialize(root); err != nil {
		return nil, err
	}
	python, err := extract.FindPython()
	if err != nil {
		return nil, err
	}
	return &extract.Runner{Python: python, Script: extract.Path(root)}, nil
}

// fetchReport is what `fetch` reports under --json. It carries the text as well as
// naming the file, so a consumer does not have to read one to use the other.
type fetchReport struct {
	Pointer   string   `json:"pointer"`
	Path      string   `json:"path"`
	Kind      string   `json:"kind"`
	Extractor string   `json:"extractor"`
	Pages     int      `json:"pages,omitempty"`
	Truncated bool     `json:"truncated"`
	Notes     []string `json:"notes,omitempty"`
	Text      string   `json:"text"`
}

// clearReport is the other thing this command can do, and it has nothing to say
// about a document, so it reports separately rather than half-empty.
type clearReport struct {
	Directory string `json:"directory"`
	Cleared   int    `json:"cleared"`
}

// fetchCommand implements `stemma fetch`.
//
// It reads a document and leaves its text where an agent can find it again. It
// does not touch the bibliography: `cite add` records what a work is and this
// records what it says, so a changed .bib always has one author, and reading a
// local file that no record points at is still something the tool can do.
var fetchCommand = &command{
	name:    "fetch",
	summary: "read a document's text into the scratch area",
	args:    "PATH-OR-URL",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		clear := fs.Bool("clear", false, "empty the scratch area and stop")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			k, code := o.load(w)
			if k == nil {
				return code
			}

			if *clear {
				if len(args) > 0 {
					return w.fail(fmt.Errorf("fetch --clear takes no arguments, got %q", args[0]))
				}
				files, err := extract.Clear(k.Root)
				if err != nil {
					return w.fail(err)
				}
				if w.json {
					return w.emit(clearReport{Directory: extract.ScratchDir(k.Root), Cleared: files})
				}
				noun := "files"
				if files == 1 {
					noun = "file"
				}
				fmt.Fprintf(w.stdout, "cleared %d %s from %s\n", files, noun, extract.ScratchDir(k.Root))
				return ExitOK
			}

			if len(args) != 1 {
				return w.fail(fmt.Errorf("fetch takes one path or URL, got %d", len(args)))
			}
			pointer := args[0]

			runner, err := newRunner(k.Root)
			if err != nil {
				return w.fail(err)
			}
			result, err := readPointer(runner, pointer)
			if err != nil {
				return w.fail(err)
			}

			path := extract.TextPath(k.Root, pointer)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return w.fail(err)
			}
			if err := kb.WriteFileAtomic(path, []byte(result.Text)); err != nil {
				return w.fail(err)
			}

			if w.json {
				return w.emit(fetchReport{
					Pointer:   pointer,
					Path:      path,
					Kind:      result.Kind,
					Extractor: result.Extractor,
					Pages:     result.Pages,
					Truncated: result.Truncated,
					Notes:     result.Notes,
					Text:      result.Text,
				})
			}
			// The text is the answer and the path is a notice, so the answer is
			// what a pipe carries and the notice is what a terminal shows.
			fmt.Fprintf(w.stderr, "stemma: %s\n", path)
			fmt.Fprint(w.stdout, result.Text)
			return ExitOK
		}
	},
}

// readPointer decides which subcommand reads a pointer, and runs it.
//
// An identifier names a work rather than a document — resolving one produces a
// record, not text — so it is refused with the command that does own it instead of
// being attempted as a filename.
func readPointer(runner *extract.Runner, pointer string) (*extract.Result, error) {
	ctx := context.Background()

	kind := source.Detect(pointer)
	switch {
	case kind == source.KindURL:
		return runner.URL(ctx, pointer)
	case kind != source.KindUnknown:
		return nil, fmt.Errorf(
			"%q names a work (%s) rather than a document; `stemma cite add` records what a work is, "+
				"and fetch reads what it says, which is a file or a URL", pointer, kind)
	}

	family, err := localFamily(pointer)
	if err != nil {
		return nil, err
	}
	if family == "pdf" {
		return runner.PDF(ctx, pointer)
	}
	return runner.Text(ctx, pointer)
}

// localFamily decides which subcommand reads a local file.
//
// The bytes decide rather than the name, because a PDF is a PDF whatever it is
// called: a paper downloaded without its extension would otherwise be read as text
// and fail on its encoding, which is a confusing way to learn that it was a PDF.
func localFamily(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, and fetch reads one document", path)
	}

	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()

	head := make([]byte, len(pdfHeader))
	read, err := io.ReadFull(handle, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	if bytes.Contains(head[:read], []byte(pdfHeader)) {
		return "pdf", nil
	}
	return "text", nil
}

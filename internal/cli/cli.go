// Package cli is the command-line surface: the one interface every other
// surface falls back to.
//
// It is deliberately thin. Parsing, resolution and the invariants all live in
// the core library, and a command here does little more than find a KB, call
// the library, and decide an exit code. Run returns that code rather than
// calling os.Exit, so the whole surface is testable without a subprocess.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// Exit codes. Every command returns one of exactly these.
const (
	// ExitOK is a clean run.
	ExitOK = 0
	// ExitFindings is a run that found something to report.
	ExitFindings = 1
	// ExitError is a run that could not be completed.
	ExitError = 2
)

// EnvKB names the environment variable that points at a KB root, used when no
// path is given on the command line.
const EnvKB = "STEMMA_KB"

const usage = `stemma is a tool for authoring and maintaining a knowledge base.

usage: stemma <command> [flags]

commands:
  init      create a KB root
  new       create a page, or a draft in the inbox
  list      list the pages
  show      show one page with its links and citations resolved
  move      move a page to another directory
  rename    retitle a page and rewrite every link that named it
  archive   archive a page, recording why
  lint      report everything wrong with the KB
  help      show this message

Every command accepts:
  --json        write output as JSON.

Commands that work on an existing KB also accept:
  --kb <path>   the KB root. Discovered when not given.
  --strict      treat warnings as errors.

The KB root is found from --kb, then $` + EnvKB + `, then by walking up from the
working directory looking for a ` + kb.ManifestName + `, and failing that for a
directory holding ` + kb.PagesDir + `/.
`

// Run executes one invocation and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ExitError
	}
	command, rest := args[0], args[1:]
	switch command {
	case "init":
		return runInit(rest, stdout, stderr)
	case "new":
		return runNew(rest, stdout, stderr)
	case "list":
		return runList(rest, stdout, stderr)
	case "show":
		return runShow(rest, stdout, stderr)
	case "move":
		return runMove(rest, stdout, stderr)
	case "rename":
		return runRename(rest, stdout, stderr)
	case "archive":
		return runArchive(rest, stdout, stderr)
	case "lint":
		return runLint(rest, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return ExitOK
	default:
		fmt.Fprintf(stderr, "stemma: unknown command %q\n\n", command)
		fmt.Fprint(stderr, usage)
		return ExitError
	}
}

// resolve turns a page name given on the command line into a path in the KB.
//
// A name that matches nothing, or matches more than one page, is a failure of
// the command rather than a finding about the KB: the tool was asked for one
// page and cannot produce it.
//
// A path is accepted as well as a name, and takes priority. That matters when
// two pages claim one name: every link to it is then a hard error, and without
// this there would be no way to say which page a command meant, so the tool
// would be unable to repair a collision it had found. A name never contains a
// slash, so there is no doubt about which of the two was given.
func resolve(k *kb.KB, name string) (string, error) {
	if _, ok := k.Graph.Page(name); ok {
		return name, nil
	}
	switch r := k.Graph.Resolve(name); r.Kind {
	case kb.Resolved:
		return r.Path, nil
	case kb.Ambiguous:
		return "", fmt.Errorf("%q could be %s", name, strings.Join(r.Matches, " or "))
	default:
		return "", fmt.Errorf("no page is called %q", name)
	}
}

// fail writes a message the way every command writes one, so that a failure
// always looks the same whatever it was that failed.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "stemma: %v\n", err)
	return ExitError
}

// parse parses a command's arguments, letting flags and positional arguments
// appear in any order.
func parse(fs *flag.FlagSet, args []string) error {
	return fs.Parse(permute(fs, args))
}

// permute moves flags ahead of positional arguments.
//
// The standard library stops at the first argument that is not a flag, which
// would make `stemma new "A Title" --draft` read --draft as part of a title
// and quietly create a page rather than a draft. Nothing the project allows
// itself to depend on parses interspersed flags, so the arguments are
// rearranged before the parser sees them.
//
// Whether a flag takes a value is asked of the flag itself rather than guessed,
// because guessing wrong would either swallow a title or leave a value behind.
func permute(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return append(append(flags, positional...), args[i+1:]...)
		}
		if len(arg) < 2 || arg[0] != '-' {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.ContainsRune(name, '=') {
			continue
		}
		if f := fs.Lookup(name); f != nil && takesValue(f) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

// takesValue reports whether a flag consumes the argument after it. A boolean
// flag does not, which is what the standard library asks of a flag's value.
func takesValue(f *flag.Flag) bool {
	type boolFlag interface{ IsBoolFlag() bool }
	bf, ok := f.Value.(boolFlag)
	return !ok || !bf.IsBoolFlag()
}

// options are the flags every command accepts. Keeping them in one place is
// what makes --json and --strict uniform across the surface instead of a
// property of whichever command remembered to add them.
type options struct {
	kb     string
	json   bool
	strict bool
}

func (o *options) register(fs *flag.FlagSet) {
	fs.StringVar(&o.kb, "kb", "", "the KB root; discovered when not given")
	fs.BoolVar(&o.json, "json", false, "write output as JSON")
	fs.BoolVar(&o.strict, "strict", false, "treat warnings as errors")
}

// mode is the leniency the flags ask for.
func (o *options) mode() kb.Mode {
	if o.strict {
		return kb.Strict
	}
	return kb.Lenient
}

// load finds and reads the KB, reporting the failure itself. A nil KB means the
// caller should return the code alongside it.
func (o *options) load(stderr io.Writer) (*kb.KB, int) {
	root, err := discover(o.kb)
	if err != nil {
		fmt.Fprintf(stderr, "stemma: %v\n", err)
		return nil, ExitError
	}
	k, err := kb.Load(root)
	if err != nil {
		fmt.Fprintf(stderr, "stemma: %v\n", err)
		return nil, ExitError
	}
	return k, ExitOK
}

// discover finds the KB root.
//
// The order is fixed: an explicit path, then the environment, then walking up
// from the working directory. Walking up prefers a directory holding a
// manifest, and falls back to the nearest one holding pages/, so a KB
// configured by hand beats a directory that merely looks like one.
func discover(explicit string) (string, error) {
	if explicit != "" {
		return asDirectory(explicit, "")
	}
	if env := os.Getenv(EnvKB); env != "" {
		return asDirectory(env, EnvKB+" is set to ")
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	fallback := ""
	for {
		if _, err := os.Stat(filepath.Join(dir, kb.ManifestName)); err == nil {
			return dir, nil
		}
		if fallback == "" {
			if info, err := os.Stat(filepath.Join(dir, kb.PagesDir)); err == nil && info.IsDir() {
				fallback = dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no KB found: pass --kb, set %s, or run inside one", EnvKB)
}

// asDirectory checks that a named KB root is a directory, and gives the failure
// a message that says where the name came from.
func asDirectory(name, prefix string) (string, error) {
	info, err := os.Stat(name)
	if err != nil {
		return "", fmt.Errorf("%s%s: %w", prefix, name, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s%s is not a directory", prefix, name)
	}
	return name, nil
}

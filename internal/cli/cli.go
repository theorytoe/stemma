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
  lint    report everything wrong with the KB
  help    show this message

Every command accepts:
  --kb <path>   the KB root. Discovered when not given.
  --json        write output as JSON.
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

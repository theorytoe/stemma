// Package cli is the command-line surface: the one interface every other
// surface falls back to.
//
// It is deliberately thin. Parsing, resolution and the invariants all live in
// the core library, and a command here does little more than find a KB, call
// the library, and decide an exit code. Run returns that code rather than
// calling os.Exit, so the whole surface is testable without a subprocess.
//
// The surface is described once, in the command table below, and everything
// else about it is derived from that description: dispatch, the help text, and
// the generated reference. A verb that is not in the table does not exist.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
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

// EnvOffline, when set to a true value, is the same as passing --offline to a
// command that can use the network.
const EnvOffline = "STEMMA_OFFLINE"

// command is one verb on the surface.
//
// A command either runs (setup and its returned closure) or is a noun family
// with members (sub). The two are exclusive: `export` and `cite` are families
// whose members are the actual verbs, and a family itself does nothing.
type command struct {
	// name is how the verb is invoked.
	name string

	// summary is one line for the command list, with no trailing period.
	summary string

	// args names the positional arguments, for the usage line. Empty means the
	// command takes none.
	args string

	// setup registers the command's own flags and returns the closure that runs
	// it. The universal --json flag is registered by the dispatcher, and a
	// command that works on a KB asks for --kb and --strict by calling
	// registerKB. Keeping the flags here is what lets help and the reference be
	// generated rather than written by hand.
	setup func(fs *flag.FlagSet, o *options) runFunc

	// sub holds the members of a noun family, and is empty for everything else.
	sub []*command
}

// runFunc runs one command once its arguments have been parsed. args holds the
// positional arguments with all flags removed.
type runFunc func(c *command, w *output, args []string) int

// commands is the whole surface, in the order help lists it. It is filled in
// init so that the help command, which reads this table to build its own help,
// does not form an initialization cycle with it.
var commands []*command

func init() {
	commands = []*command{
		initCommand,
		newCommand,
		listCommand,
		showCommand,
		moveCommand,
		renameCommand,
		archiveCommand,
		promoteCommand,
		statusCommand,
		indexCommand,
		searchCommand,
		graphCommand,
		citeCommand,
		fetchCommand,
		lintCommand,
		envCommand,
		helpCommand,
	}
}

// Run executes one invocation and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	c, name, rest, err := lookup(commands, args)
	if err != nil {
		fmt.Fprintf(stderr, "stemma: %v\n\n", err)
		fmt.Fprint(stderr, topUsage())
		return ExitError
	}
	return c.invoke(name, rest, stdout, stderr)
}

// lookup finds the command an argument list names, and the full name it was
// invoked by.
//
// A noun family is matched by its first two arguments: `stemma export json`
// runs the `json` member of the `export` family. Naming a family without one of
// its members is a usage error rather than a guess. The full name is returned
// because the envelope has to say "cite add" rather than "add", and the member
// alone does not know its family.
func lookup(cmds []*command, args []string) (*command, string, []string, error) {
	if len(args) == 0 {
		return nil, "", nil, errors.New("no command given")
	}
	for _, c := range cmds {
		if c.name != args[0] {
			continue
		}
		if len(c.sub) == 0 {
			return c, c.name, args[1:], nil
		}
		if len(args) < 2 {
			return nil, "", nil, fmt.Errorf("%s needs one of: %s", c.name, subNames(c))
		}
		for _, s := range c.sub {
			if s.name == args[1] {
				return s, c.name + " " + s.name, args[2:], nil
			}
		}
		return nil, "", nil, fmt.Errorf("%s has no %q; it has %s", c.name, args[1], subNames(c))
	}
	return nil, "", nil, fmt.Errorf("unknown command %q", args[0])
}

// invoke parses a command's arguments and runs it. name is how the run is
// named in output, which is the full "family member" name.
func (c *command) invoke(name string, args []string, stdout, stderr io.Writer) int {
	if wantsHelp(args) {
		fmt.Fprint(stdout, commandHelp(c))
		return ExitOK
	}

	fs := flag.NewFlagSet("stemma "+c.name, flag.ContinueOnError)
	// The errors flag returns are turned into one uniform message by output.fail
	// rather than printed here, so that a --json run stays parseable.
	fs.SetOutput(io.Discard)

	var o options
	o.registerJSON(fs)
	run := c.setup(fs, &o)
	if err := parse(fs, args); err != nil {
		w := &output{cmd: name, json: o.json || wantsJSON(args), stdout: stdout, stderr: stderr}
		return w.fail(err)
	}
	w := &output{cmd: name, json: o.json, stdout: stdout, stderr: stderr}
	return run(c, w, fs.Args())
}

// find returns the command with a name, or nil.
func find(name string) *command {
	for _, c := range commands {
		if c.name == name {
			return c
		}
	}
	return nil
}

// options are the flags every command accepts. Keeping them in one place is
// what makes --json and --strict uniform across the surface instead of a
// property of whichever command remembered to add them.
type options struct {
	kb     string
	json   bool
	strict bool
}

// registerJSON is the one flag no command has to ask for.
func (o *options) registerJSON(fs *flag.FlagSet) {
	fs.BoolVar(&o.json, "json", false, "write output as JSON")
}

// registerKB adds the flags a command needs to find and read a KB.
func (o *options) registerKB(fs *flag.FlagSet) {
	fs.StringVar(&o.kb, "kb", "", "the KB root; discovered when not given")
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
func (o *options) load(w *output) (*kb.KB, int) {
	root, err := discover(o.kb)
	if err != nil {
		return nil, w.fail(err)
	}
	k, err := kb.Load(root)
	if err != nil {
		return nil, w.fail(err)
	}
	return k, ExitOK
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

// wantsHelp reports whether an argument list asks for help rather than a run.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "-h", "-help", "--help":
			return true
		}
	}
	return false
}

// wantsJSON reports whether an argument list asks for JSON output. It is used
// only when parsing failed, because after a successful parse the flag itself
// knows the answer.
func wantsJSON(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "-json" || a == "--json" {
			return true
		}
	}
	return false
}

// stringList collects a repeatable flag. Each use appends one value.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Get lets the help generator name the flag's type.
func (s *stringList) Get() any { return []string(*s) }

// listFlag registers a repeatable string flag.
func listFlag(fs *flag.FlagSet, name, usage string) *stringList {
	var out stringList
	fs.Var(&out, name, usage)
	return &out
}

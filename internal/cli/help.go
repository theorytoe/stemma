package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// helpCommand implements `stemma help`.
//
// Help is generated from the command table rather than written beside it, so a
// verb or a flag cannot be added without appearing here. --markdown emits the
// whole reference in one document, which is what the skill suite consumes. A
// noun family's member is named in two words, so `help cite add` and
// `stemma cite add` resolve alike.
var helpCommand = &command{
	name:    "help",
	summary: "show help for a command",
	args:    "[COMMAND [MEMBER]]",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		markdown := fs.Bool("markdown", false, "write the whole command reference as markdown")
		return func(c *command, w *output, args []string) int {
			if *markdown {
				if w.json {
					return w.emit(map[string]string{"markdown": referenceMarkdown()})
				}
				fmt.Fprint(w.stdout, referenceMarkdown())
				return ExitOK
			}
			target, name, err := helpTarget(args)
			if err != nil {
				return w.fail(err)
			}
			if target == nil {
				if w.json {
					return w.emit(helpReport{Commands: surface()})
				}
				fmt.Fprint(w.stdout, topUsage())
				return ExitOK
			}
			if w.json {
				info := describeCommand(target)
				info.Name = name
				return w.emit(info)
			}
			fmt.Fprint(w.stdout, commandHelp(name, target))
			return ExitOK
		}
	},
}

// helpTarget resolves the positional arguments after `help` to the command they
// name and the full name it is invoked by. A noun family is named by its first
// two arguments, so the same resolution the dispatcher uses applies here, and
// nil with no error means the arguments named nothing and the whole surface is
// wanted. Naming a family without a member is not an error: the family is a
// page of its own, listing what its members are.
func helpTarget(args []string) (*command, string, error) {
	switch len(args) {
	case 0:
		return nil, "", nil
	case 1:
		c := find(args[0])
		if c == nil {
			return nil, "", fmt.Errorf("no command is called %q", args[0])
		}
		return c, c.name, nil
	case 2:
		c := find(args[0])
		if c == nil {
			return nil, "", fmt.Errorf("no command is called %q", args[0])
		}
		if len(c.sub) == 0 {
			return nil, "", fmt.Errorf("%s takes no member %q", c.name, args[1])
		}
		for _, s := range c.sub {
			if s.name == args[1] {
				return s, c.name + " " + s.name, nil
			}
		}
		return nil, "", fmt.Errorf("%s has no %q; it has %s", c.name, args[1], subNames(c))
	default:
		return nil, "", fmt.Errorf("help takes at most one command, got %d", len(args))
	}
}

// topUsage is the index of the surface.
func topUsage() string {
	var b strings.Builder
	b.WriteString("stemma is a tool for authoring and maintaining a knowledge base.\n\n")
	b.WriteString("usage: stemma <command> [flags]\n\ncommands:\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "  %-9s %s\n", c.name, c.summary)
	}
	b.WriteString("\nEvery command accepts --json. Commands that work on an existing KB\n")
	b.WriteString("also accept --kb and --strict. When --kb is not given, the KB root comes\n")
	b.WriteString("from the STEMMA_KB environment variable, else from a walk up from the\n")
	b.WriteString("working directory. Run \"stemma help <command>\" for one command.\n")
	return b.String()
}

// commandHelp is the help for one verb or family member. name is how the command
// was invoked, which for a family member includes the family, so the usage line
// reads "stemma cite add" rather than "stemma add".
func commandHelp(name string, c *command) string {
	var b strings.Builder
	fmt.Fprintf(&b, "usage: stemma %s", name)
	if c.args != "" {
		fmt.Fprintf(&b, " %s", c.args)
	}
	fmt.Fprintf(&b, " [flags]\n\n%s\n", c.summary)

	if len(c.sub) > 0 {
		b.WriteString("\ncommands:\n")
		for _, s := range c.sub {
			fmt.Fprintf(&b, "  %-9s %s\n", s.name, s.summary)
		}
	}

	b.WriteString("\nflags:\n")
	for _, f := range flagsOf(c) {
		fmt.Fprintf(&b, "  --%s", f.Name)
		if f.Type != "bool" {
			fmt.Fprintf(&b, " %s", f.Type)
		}
		fmt.Fprintf(&b, "\n      %s", f.Usage)
		if f.Default != "" {
			fmt.Fprintf(&b, " (default %s)", f.Default)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// subNames lists a family's members for a usage error.
func subNames(c *command) string {
	names := make([]string, len(c.sub))
	for i, s := range c.sub {
		names[i] = s.name
	}
	return strings.Join(names, ", ")
}

// flagDoc is one documented flag, derived from the flag set the command builds
// so that a flag and its documentation cannot drift apart.
type flagDoc struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default,omitempty"`
	Usage   string `json:"usage"`
}

// helpReport is the surface as data, for the agent that would rather read JSON
// than parse the help text.
type helpReport struct {
	Commands []commandInfo `json:"commands"`
}

// commandInfo describes one verb.
type commandInfo struct {
	Name    string    `json:"name"`
	Summary string    `json:"summary"`
	Args    string    `json:"args,omitempty"`
	Members []string  `json:"members,omitempty"`
	Flags   []flagDoc `json:"flags,omitempty"`
}

func surface() []commandInfo {
	out := make([]commandInfo, 0, len(commands))
	for _, c := range commands {
		out = append(out, describeCommand(c))
	}
	return out
}

func describeCommand(c *command) commandInfo {
	info := commandInfo{Name: c.name, Summary: c.summary, Args: c.args}
	if len(c.sub) > 0 {
		for _, s := range c.sub {
			info.Members = append(info.Members, s.name)
		}
	}
	info.Flags = flagsOf(c)
	return info
}

// flagsOf builds a command's flag set and reads back what it defined.
func flagsOf(c *command) []flagDoc {
	fs := flag.NewFlagSet("stemma "+c.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o options
	o.registerJSON(fs)
	if c.setup != nil {
		c.setup(fs, &o)
	}
	var out []flagDoc
	fs.VisitAll(func(f *flag.Flag) {
		out = append(out, flagDoc{
			Name:    f.Name,
			Type:    flagType(f),
			Default: flagDefault(f),
			Usage:   f.Usage,
		})
	})
	return out
}

// flagType names a flag's value for the help text.
func flagType(f *flag.Flag) string {
	g, ok := f.Value.(flag.Getter)
	if !ok {
		return "value"
	}
	switch g.Get().(type) {
	case bool:
		return "bool"
	case int:
		return "int"
	case []string:
		return "list"
	case string:
		return "string"
	}
	return "value"
}

// flagDefault is a flag's default as a person should read it. A zero value is
// left out rather than printed, because "default false" is noise.
func flagDefault(f *flag.Flag) string {
	g, ok := f.Value.(flag.Getter)
	if !ok {
		return ""
	}
	switch v := g.Get().(type) {
	case bool:
		return ""
	case int:
		if v == 0 {
			return ""
		}
		return fmt.Sprint(v)
	case []string:
		if len(v) == 0 {
			return ""
		}
		return strings.Join(v, ", ")
	case string:
		return v
	}
	return ""
}

// referenceMarkdown is the generated CLI reference: one document describing
// every verb, flag, exit code and JSON envelope, made from the command table so
// it cannot drift from it.
func referenceMarkdown() string {
	var b strings.Builder
	b.WriteString("# The stemma command reference\n\n")
	b.WriteString("This document is generated from the command definitions in the tool's source\n")
	b.WriteString("and describes the whole command surface. Every command supports `--json`.\n\n")

	b.WriteString("## Exit codes\n\n")
	b.WriteString("| Code | Meaning |\n| ---- | ------- |\n")
	b.WriteString("| `0` | the run was clean |\n")
	b.WriteString("| `1` | the run found something to report |\n")
	b.WriteString("| `2` | the run could not be completed |\n\n")
	b.WriteString("A code of `1` means the KB has problems; `2` means the tool broke. They are\n")
	b.WriteString("deliberately different, because a lint failure and a missing file call for\n")
	b.WriteString("different responses.\n\n")

	b.WriteString("## JSON output\n\n")
	b.WriteString("Every command accepts `--json`. The output is one envelope, always indented,\n")
	b.WriteString("always on stdout, and nothing else is written to either stream:\n\n")
	b.WriteString("```json\n")
	b.WriteString("{\n  \"command\": \"list\",\n  \"ok\": true,\n  \"data\": { \"count\": 2, \"pages\": [] }\n}\n")
	b.WriteString("```\n\n")
	b.WriteString("- `command` names the verb that ran.\n")
	b.WriteString("- `ok` is true only for a clean run.\n")
	b.WriteString("- `data` is the command's own payload.\n")
	b.WriteString("- `findings` carries validation findings, and is present when there are any.\n")
	b.WriteString("- `error` carries an operational failure, and is present when the run failed.\n\n")
	b.WriteString("`ok` is false with `findings` when the KB has problems (exit `1`), and false\n")
	b.WriteString("with `error` when the run could not be completed (exit `2`).\n\n")

	b.WriteString("## Commands\n")
	for _, c := range commands {
		b.WriteString("\n### `stemma " + c.name)
		if c.args != "" {
			b.WriteString(" " + c.args)
		}
		b.WriteString("`\n\n")
		b.WriteString(c.summary + ".\n")
		if len(c.sub) > 0 {
			b.WriteString("\nMembers: ")
			for i, s := range c.sub {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString("`" + s.name + "`")
			}
			b.WriteString(".\n")
			// A family's own flags are only --json, so the flags that matter
			// belong to its members and are documented with each one.
			for _, s := range c.sub {
				b.WriteString("\n#### `stemma " + c.name + " " + s.name)
				if s.args != "" {
					b.WriteString(" " + s.args)
				}
				b.WriteString("`\n\n")
				b.WriteString(s.summary + ".\n")
				b.WriteString(flagTable(s))
			}
			continue
		}
		b.WriteString(flagTable(c))
	}
	return b.String()
}

// flagTable renders one command's flags as the markdown table the reference
// uses, derived from the flag set the command builds.
func flagTable(c *command) string {
	var b strings.Builder
	b.WriteString("\n| Flag | Type | Default | Meaning |\n")
	b.WriteString("| ---- | ---- | ------- | ------- |\n")
	for _, f := range flagsOf(c) {
		def := f.Default
		if def == "" {
			def = "—"
		} else {
			def = "`" + def + "`"
		}
		fmt.Fprintf(&b, "| `--%s` | %s | %s | %s |\n", f.Name, f.Type, def, f.Usage)
	}
	return b.String()
}

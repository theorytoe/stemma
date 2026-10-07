package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/theorytoe/stemma/internal/version"
)

// Manual pages are the third rendering of the command table, beside the help
// text and the markdown reference, and they are derived from the same
// description for the same reason: a verb or a flag that is added to the table
// gets a page without anyone maintaining a second list. The table is the one
// source (D10) and a generated page is a derivative that is never committed
// (P8, D23), so the pages are written into a directory and installed from there.
//
// A manual is a set of files rather than one document, because `man stemma` and
// `man stemma-lint` are separate lookups. `--out` therefore names a directory
// and the whole set is written into it; rendering a single page to standard
// output is the same work with the directory left out.

// manTarget is one page's place in the surface: the name it is installed under,
// how the command is spelled after the program name, and the command itself.
type manTarget struct {
	// page is the page name without its section — "stemma", "stemma-lint",
	// "stemma-cite-add" — and is also the file's base name.
	page string

	// name is how a command is invoked after `stemma`: "lint", "cite add". It
	// is empty for the index page, which describes no single command.
	name string

	// family is the noun family a member belongs to: "cite" for "cite add".
	// Empty when the page documents a top-level verb or the index.
	family string

	// c is the command the page describes, nil for the index page.
	c *command
}

// manTargets is the whole set of pages: the index first, then the command table
// in its own order, so a family's members follow the family and the pages read
// in the order help lists commands in.
func manTargets() []manTarget {
	out := []manTarget{{page: "stemma"}}
	for _, c := range commands {
		out = append(out, targetFor(c, c.name))
		for _, s := range c.sub {
			out = append(out, targetFor(s, c.name+" "+s.name))
		}
	}
	return out
}

// targetFor names the page a command is documented on. A family member carries
// its family in the page name, so `cite add` becomes stemma-cite-add.1, the way
// `git commit` becomes git-commit.1.
func targetFor(c *command, name string) manTarget {
	if c == nil {
		return manTarget{page: "stemma"}
	}
	t := manTarget{page: "stemma-" + strings.ReplaceAll(name, " ", "-"), name: name, c: c}
	if i := strings.IndexByte(name, ' '); i >= 0 {
		t.family = name[:i]
	}
	return t
}

// manTargetFor resolves the positional arguments after `help` to the page they
// name, using the same resolution the dispatcher uses so that `help cite add`
// and `stemma cite add` agree on what a name means.
func manTargetFor(args []string) (manTarget, error) {
	c, name, err := helpTarget(args)
	if err != nil {
		return manTarget{}, err
	}
	return targetFor(c, name), nil
}

// manFileName is the file a page is written to. Section 1 is user commands.
func manFileName(page string) string { return page + ".1" }

// manFile is one written page, for the report a directory write returns.
type manFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// manReport is what `help --man --out` returns under --json: where the pages
// went, so a caller does not have to guess the names.
type manReport struct {
	Dir   string    `json:"dir"`
	Pages []manFile `json:"pages"`
}

// runMan implements `help --man`. With a directory it writes the page the
// arguments name, or the whole set when they name nothing; without one it
// writes a single page to standard output.
func runMan(w *output, args []string, dir string) int {
	target, err := manTargetFor(args)
	if err != nil {
		return w.fail(err)
	}
	if dir == "" {
		body := manPage(target)
		if w.json {
			return w.emit(map[string]string{"man": body})
		}
		fmt.Fprint(w.stdout, body)
		return ExitOK
	}

	targets := []manTarget{target}
	if len(args) == 0 {
		targets = manTargets()
	}
	files, err := writeManPages(dir, targets)
	if err != nil {
		return w.fail(err)
	}
	if w.json {
		return w.emit(manReport{Dir: dir, Pages: files})
	}
	for _, f := range files {
		fmt.Fprintln(w.stdout, f.Path)
	}
	return ExitOK
}

// writeManPages writes one file per page into dir, creating it when missing, and
// returns what it wrote in the order the pages were given.
func writeManPages(dir string, targets []manTarget) ([]manFile, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}
	out := make([]manFile, 0, len(targets))
	for _, t := range targets {
		path := filepath.Join(dir, manFileName(t.page))
		if err := os.WriteFile(path, []byte(manPage(t)), 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", path, err)
		}
		out = append(out, manFile{Name: t.page, Path: path})
	}
	return out, nil
}

// manPage renders one page as roff for section 1. The sections are written in
// the order a person reads them: what it is, how to run it, what it does, then
// what it takes and what it returns.
func manPage(t manTarget) string {
	var b strings.Builder
	var flags []flagDoc
	if t.c != nil {
		flags = flagsOf(t.c)
	}
	manHeader(&b, t)
	manName(&b, t)
	manSynopsis(&b, t)
	manDescription(&b, t)
	manCommands(&b, t)
	manOptions(&b, flags)
	manEnvironment(&b, flags)
	manExitStatus(&b)
	manSeeAlso(&b, t)
	return b.String()
}

// manHeader writes the .TH line. The title is the page name uppercased, as the
// markup convention wants, and the source names the build, so `man stemma` says
// which stemma it is describing. The date is the day the page was generated,
// which is what it is: nobody edited the page, the tool wrote it. A missing
// date is legal and is what an unedited draft has, but mandoc reports it, so the
// page carries the only true date it can have.
func manHeader(b *strings.Builder, t manTarget) {
	fmt.Fprintf(b, ".TH %s 1 %q \"stemma %s\" \"stemma Manual\"\n",
		strings.ToUpper(t.page), time.Now().Format("2006-01-02"), manText(version.Version))
}

// manName is the NAME section, the one line `man -k` and `apropos` index.
func manName(b *strings.Builder, t manTarget) {
	b.WriteString(".SH NAME\n")
	if t.c == nil {
		b.WriteString(manText("stemma") + ` \- ` +
			manText("author and maintain a citation-bearing knowledge base") + "\n")
		return
	}
	b.WriteString(manText("stemma "+t.name) + ` \- ` + manText(t.c.summary) + "\n")
}

// manSynopsis spells the invocation. The program name and the verb are fixed; a
// command's positional arguments come from the table, so the synopsis cannot
// promise an argument the command does not take.
func manSynopsis(b *strings.Builder, t manTarget) {
	b.WriteString(".SH SYNOPSIS\n")
	if t.c == nil {
		b.WriteString(`\fBstemma\fR \fIcommand\fR [\fIflags\fR]` + "\n")
		return
	}
	fmt.Fprintf(b, `\fBstemma %s\fR`, manText(t.name))
	if t.c.args != "" {
		fmt.Fprintf(b, ` \fI%s\fR`, manText(t.c.args))
	}
	b.WriteString(" [\\fIflags\\fR]\n")
}

// manDescription is the command's own summary, or for the index a short account
// of what the tool is and how flags are found. The flags themselves are not
// repeated here: every command page carries the authoritative list, and a
// second copy is a second thing to keep true.
func manDescription(b *strings.Builder, t manTarget) {
	b.WriteString(".SH DESCRIPTION\n")
	if t.c != nil {
		manParagraph(b, t.c.summary+".")
		return
	}
	manParagraph(b, "stemma is a tool for authoring and maintaining a citation-bearing "+
		"knowledge base. A knowledge base is a directory of markdown pages linked with "+
		"[[wikilinks]] that cite sources from a BibTeX bibliography.")
	manParagraph(b, "Every command accepts --json, and a command that works on an existing "+
		"knowledge base also accepts --kb and --strict. The flags of one command are in that "+
		"command's own page.")
}

// manCommands lists what may be invoked: the whole surface for the index, and a
// family's members on the family's own page. The family page is where a person
// lands after reading that `cite` is a noun, so it is the page that has to say
// what the nouns are.
func manCommands(b *strings.Builder, t manTarget) {
	var members []*command
	var parent string
	switch {
	case t.c == nil:
		members, parent = commands, "stemma"
	case len(t.c.sub) > 0:
		members, parent = t.c.sub, "stemma-"+t.c.name
	default:
		return
	}
	b.WriteString(".SH COMMANDS\n")
	for _, m := range members {
		b.WriteString(".TP\n")
		fmt.Fprintf(b, ".B %s\n", manText(m.name))
		manRoffParagraph(b, manText(m.summary)+". See "+manRef(parent+"-"+m.name)+".")
	}
}

// manOptions documents one command's flags, derived from the flag set the
// command builds, so a flag and its documentation cannot drift apart.
func manOptions(b *strings.Builder, flags []flagDoc) {
	if len(flags) == 0 {
		return
	}
	b.WriteString(".SH OPTIONS\n")
	for _, f := range flags {
		b.WriteString(".TP\n")
		fmt.Fprintf(b, ".B \\-\\-%s", manText(f.Name))
		if f.Type != "bool" {
			name := "value"
			if f.Type == "int" {
				name = "n"
			}
			fmt.Fprintf(b, " \\fI%s\\fR", name)
		}
		b.WriteString("\n")
		usage := f.Usage
		if f.Default != "" {
			usage += fmt.Sprintf(" (default %s)", f.Default)
		}
		manParagraph(b, usage)
	}
}

// manEnvironment documents the variables a command reads. It is driven by the
// flags, so a command that takes no KB is not told about STEMMA_KB, and a
// command that never reaches the network is not told about STEMMA_OFFLINE.
func manEnvironment(b *strings.Builder, flags []flagDoc) {
	type variable struct{ name, text string }
	var vars []variable
	if hasFlag(flags, "kb") {
		vars = append(vars, variable{EnvKB,
			"the knowledge base root, used when --kb is not given; otherwise the KB is " +
				"discovered by walking up from the working directory"})
	}
	if hasFlag(flags, "offline") {
		vars = append(vars, variable{EnvOffline,
			"a true value is the same as --offline, so no request is made"})
	}
	if len(vars) == 0 {
		return
	}
	b.WriteString(".SH ENVIRONMENT\n")
	for _, v := range vars {
		fmt.Fprintf(b, ".TP\n.B %s\n", v.name)
		manParagraph(b, v.text)
	}
}

// manExitStatus is the same three codes on every page, because they are a
// property of the surface rather than of one command.
func manExitStatus(b *strings.Builder) {
	b.WriteString(".SH EXIT STATUS\n")
	for _, e := range [][2]string{
		{"0", "the run was clean"},
		{"1", "the run found something to report"},
		{"2", "the run could not be completed"},
	} {
		fmt.Fprintf(b, ".TP\n.B %s\n%s\n", e[0], manText(e[1]))
	}
}

// manSeeAlso links a page to its neighbours: the index always, the family for a
// member and the members for a family, so the set is navigable from any page.
func manSeeAlso(b *strings.Builder, t manTarget) {
	b.WriteString(".SH SEE ALSO\n")
	var refs []string
	if t.c != nil {
		refs = append(refs, manRef("stemma"))
		if t.family != "" {
			refs = append(refs, manRef("stemma-"+t.family))
		}
		for _, s := range t.c.sub {
			refs = append(refs, manRef("stemma-"+t.name+"-"+s.name))
		}
		// A command that is neither a family nor a member is the end of the
		// line, so it offers the one page a person can ask for anywhere. The
		// help page is not sent to itself.
		if len(t.c.sub) == 0 && t.family == "" && t.page != "stemma-help" {
			refs = append(refs, manRef("stemma-help"))
		}
	}
	if len(refs) == 0 {
		refs = append(refs, manRef("stemma-help"))
	}
	b.WriteString(strings.Join(refs, ", ") + "\n")
}

// manRef is a cross-reference as `man` prints one: the page name in bold, then
// the section in parentheses.
func manRef(page string) string {
	return "\\fB" + manText(page) + "\\fR(1)"
}

// manText escapes one run of prose for roff. A backslash would begin an escape,
// and a hyphen is written \- so that a name like --dry-run is a name and not a
// range. Nothing else in prose needs escaping; a line that would begin with a
// dot or an apostrophe is guarded by manParagraph, which is where the line
// beginning is known.
func manText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\e`)
		case '-':
			b.WriteString(`\-`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// manParagraph writes prose as a filled roff paragraph, escaping it first.
func manParagraph(b *strings.Builder, text string) {
	manRoffParagraph(b, manText(text))
}

// manRoffParagraph writes text that already carries roff markup as a filled
// paragraph, so a caller that builds a cross-reference can pass it through
// without the markup being escaped a second time. Any prose in it must have
// been through manText already.
//
// roff joins consecutive input lines of a paragraph, so the text is wrapped and
// each line is written separately. It reads a line beginning with a dot or an
// apostrophe as a request, so a wrapped line that would is given a zero-width \&
// to lead with.
func manRoffParagraph(b *strings.Builder, roff string) {
	for _, line := range wrapWords(roff, 70) {
		if line == "" {
			continue
		}
		if line[0] == '.' || line[0] == '\'' {
			b.WriteString(`\&`)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

// wrapWords breaks text into lines of at most width columns, on spaces.
func wrapWords(text string, width int) []string {
	var out []string
	var line string
	for _, w := range strings.Fields(text) {
		switch {
		case line == "":
			line = w
		case len(line)+1+len(w) <= width:
			line += " " + w
		default:
			out = append(out, line)
			line = w
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

// hasFlag reports whether a command's flag set defines a flag.
func hasFlag(flags []flagDoc, name string) bool {
	for _, f := range flags {
		if f.Name == name {
			return true
		}
	}
	return false
}

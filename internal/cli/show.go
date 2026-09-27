package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// showLink is one resolved link, and showCitation one resolved citation.
type showLink struct {
	Target     string   `json:"target"`
	Resolved   string   `json:"resolved,omitempty"`
	Matches    []string `json:"matches,omitempty"`
	Unresolved bool     `json:"unresolved,omitempty"`
}

type showCitation struct {
	Key       string `json:"key"`
	Defined   bool   `json:"defined"`
	DefinedIn string `json:"defined_in,omitempty"`
}

type showReport struct {
	Path          string         `json:"path"`
	Title         string         `json:"title"`
	Type          string         `json:"type"`
	Status        string         `json:"status"`
	Aliases       []string       `json:"aliases,omitempty"`
	ArchiveReason string         `json:"archive_reason,omitempty"`
	Body          string         `json:"body"`
	Links         []showLink     `json:"links"`
	Citations     []showCitation `json:"citations"`
}

// runShow implements `stemma show`.
//
// The page is named the way a link names it, so show is also the way to ask
// where a link goes. --path and --raw exist for scripts: each prints one thing
// and nothing else, so neither needs parsing.
func runShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stemma show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	opts.register(fs)
	asPath := fs.Bool("path", false, "print only the page's path")
	raw := fs.Bool("raw", false, "print the page exactly as it is on disk")
	if err := parse(fs, args); err != nil {
		return ExitError
	}
	if fs.NArg() != 1 {
		return fail(stderr, fmt.Errorf("show takes one page name"))
	}
	if *asPath && *raw {
		return fail(stderr, fmt.Errorf("--path and --raw print different things; ask for one"))
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

	if *asPath {
		fmt.Fprintln(stdout, path)
		return ExitOK
	}
	if *raw {
		if _, err := stdout.Write(page.Bytes()); err != nil {
			return fail(stderr, err)
		}
		return ExitOK
	}

	report := describe(k, path, page)
	if opts.json {
		return writeJSON(stdout, stderr, report)
	}
	fmt.Fprint(stdout, report.text())
	return ExitOK
}

// describe gathers everything the tool knows about a page, with its links and
// citations resolved.
func describe(k *kb.KB, path string, page *kb.Page) showReport {
	r := showReport{
		Path:          path,
		Title:         page.Title(),
		Type:          page.Type(),
		Status:        page.Status(),
		Aliases:       page.Aliases(),
		ArchiveReason: page.ArchiveReason(),
		Body:          string(page.Body()),
		Links:         []showLink{},
		Citations:     []showCitation{},
	}
	for _, l := range k.Graph.Links(path) {
		res := k.Graph.Resolve(l.Name)
		link := showLink{Target: l.Target}
		switch res.Kind {
		case kb.Resolved:
			link.Resolved = res.Path
		case kb.Ambiguous:
			link.Matches = res.Matches
		default:
			link.Unresolved = true
		}
		r.Links = append(r.Links, link)
	}
	for _, c := range k.Graph.Citations(path) {
		cite := showCitation{Key: c.Key, Defined: k.Bibliography.Has(c.Key)}
		if cite.Defined {
			cite.DefinedIn = k.Bibliography.PathOf(c.Key)
		}
		r.Citations = append(r.Citations, cite)
	}
	return r
}

// text renders a report for a person: what the page says it is, what it says,
// and where the things it names actually go.
func (r showReport) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", r.Path)
	field := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%-8s%s\n", name, value)
		}
	}
	field("title", r.Title)
	field("type", r.Type)
	field("status", r.Status)
	field("aliases", strings.Join(r.Aliases, ", "))
	field("reason", r.ArchiveReason)

	fmt.Fprintf(&b, "\n%s", r.Body)
	if len(r.Body) > 0 && !strings.HasSuffix(r.Body, "\n") {
		b.WriteByte('\n')
	}

	if len(r.Links) > 0 {
		b.WriteString("\nlinks\n")
		for _, l := range r.Links {
			fmt.Fprintf(&b, "  [[%s]] -> %s\n", l.Target, l.where())
		}
	}
	if len(r.Citations) > 0 {
		b.WriteString("\ncitations\n")
		for _, c := range r.Citations {
			where := "not in the bibliography"
			if c.Defined {
				where = c.DefinedIn
			}
			fmt.Fprintf(&b, "  [@%s] -> %s\n", c.Key, where)
		}
	}
	return b.String()
}

func (l showLink) where() string {
	switch {
	case l.Resolved != "":
		return l.Resolved
	case len(l.Matches) > 0:
		return "ambiguous: " + strings.Join(l.Matches, ", ")
	default:
		return "unresolved"
	}
}

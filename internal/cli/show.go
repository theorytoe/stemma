package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// ShowLink is one resolved link, and ShowCitation one resolved citation.
type ShowLink struct {
	Target     string   `json:"target"`
	Resolved   string   `json:"resolved,omitempty"`
	Matches    []string `json:"matches,omitempty"`
	Unresolved bool     `json:"unresolved,omitempty"`
}

type ShowCitation struct {
	Key       string `json:"key"`
	Defined   bool   `json:"defined"`
	DefinedIn string `json:"defined_in,omitempty"`
}

type ShowReport struct {
	Path          string         `json:"path"`
	Title         string         `json:"title"`
	Type          string         `json:"type"`
	Status        string         `json:"status"`
	Aliases       []string       `json:"aliases,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	ArchiveReason string         `json:"archive_reason,omitempty"`
	Body          string         `json:"body"`
	Links         []ShowLink     `json:"links"`
	Citations     []ShowCitation `json:"citations"`
}

// showCommand implements `stemma show`.
//
// The page is named the way a link names it, so show is also the way to ask
// where a link goes. --path and --raw exist for scripts: each prints one thing
// and nothing else, so neither needs parsing. Under --json each of them emits
// the one thing as a value instead.
var showCommand = &command{
	name:    "show",
	summary: "show one page with its links and citations resolved",
	args:    "PAGE",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		asPath := fs.Bool("path", false, "print only the page's path")
		raw := fs.Bool("raw", false, "print the page exactly as it is on disk")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("show takes one page name"))
			}
			if *asPath && *raw {
				return w.fail(fmt.Errorf("--path and --raw print different things; ask for one"))
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}
			p, err := Resolve(k, args[0])
			if err != nil {
				return w.fail(err)
			}
			page, ok := k.Graph.Page(p)
			if !ok {
				return w.fail(fmt.Errorf("%s is not a page", p))
			}

			switch {
			case *asPath:
				if w.json {
					return w.emit(map[string]string{"path": p})
				}
				fmt.Fprintln(w.stdout, p)
				return ExitOK
			case *raw:
				if w.json {
					return w.emit(map[string]string{"path": p, "raw": string(page.Bytes())})
				}
				if _, err := w.stdout.Write(page.Bytes()); err != nil {
					return w.fail(err)
				}
				return ExitOK
			}

			report := Describe(k, p, page)
			if w.json {
				return w.emit(report)
			}
			fmt.Fprint(w.stdout, report.text())
			return ExitOK
		}
	},
}

// Describe gathers everything the tool knows about a page, with its links and
// citations resolved.
func Describe(k *kb.KB, p string, page *kb.Page) ShowReport {
	r := ShowReport{
		Path:          p,
		Title:         page.Title(),
		Type:          page.Type(),
		Status:        page.Status(),
		Aliases:       page.Aliases(),
		Tags:          page.Tags(),
		ArchiveReason: page.ArchiveReason(),
		Body:          string(page.Body()),
		Links:         []ShowLink{},
		Citations:     []ShowCitation{},
	}
	for _, l := range k.Graph.Links(p) {
		res := k.Graph.Resolve(l.Name)
		link := ShowLink{Target: l.Target}
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
	for _, cite := range k.Graph.Citations(p) {
		c := ShowCitation{Key: cite.Key, Defined: k.Bibliography.Has(cite.Key)}
		if c.Defined {
			c.DefinedIn = k.Bibliography.PathOf(cite.Key)
		}
		r.Citations = append(r.Citations, c)
	}
	return r
}

// text renders a report for a person: what the page says it is, what it says,
// and where the things it names actually go.
func (r ShowReport) text() string {
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
	field("tags", strings.Join(r.Tags, ", "))
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

func (l ShowLink) where() string {
	switch {
	case l.Resolved != "":
		return l.Resolved
	case len(l.Matches) > 0:
		return "ambiguous: " + strings.Join(l.Matches, ", ")
	default:
		return "unresolved"
	}
}

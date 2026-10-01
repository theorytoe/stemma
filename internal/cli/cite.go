package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/source"
)

// newResolver is what `cite add` resolves through. It is a variable so that a
// test can point it at a local server instead of the network.
var newResolver = source.New

// CodeCiteUnresolved is the finding code for an identifier that resolved to
// nothing, or that was malformed. It is a finding about what was asked for
// rather than a failure to ask, which is why it exits 1 and not 2.
const CodeCiteUnresolved = "cite-unresolved"

// citeCommand is the bibliography family.
var citeCommand = &command{
	name:    "cite",
	summary: "work with the bibliography",
	sub: []*command{
		citeAddCommand,
		citeListCommand,
		citeShowCommand,
		citeCitedByCommand,
		citeVendorCommand,
		citeExportCommand,
		citeCheckCommand,
	},
}

// citeAddCommand implements `stemma cite add`.
//
// It is the one command that writes to the bibliography, and it is the seam
// where the two halves of the source subsystem meet: an identifier goes out to
// an authoritative resolver, a person's fields stay at home, and both produce
// the same kind of entry and go through the same duplicate check.
//
// The check is the point. A work reaches a bibliography more than once under
// more than one key — a DOI export mints keys differently from an arXiv export,
// and a hand-entered record mints none — and two copies of one work defeat the
// reverse lookup the bibliography exists to provide. So a match updates the
// entry already there rather than appending a second one, and --force is the
// deliberate way to say "yes, I meant a separate copy".
var citeAddCommand = &command{
	name:    "add",
	summary: "add a source, resolving an identifier or taking fields by hand",
	args:    "[IDENTIFIER]",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		typ := fs.String("type", "", "the entry type; inferred when not given")
		key := fs.String("key", "", "the citation key; generated when not given")
		offline := fs.Bool("offline", envBool(EnvOffline), "do not use the network")
		dryRun := fs.Bool("dry-run", false, "report what would be written without writing it")
		force := fs.Bool("force", false, "append a second copy even when the work is already there")

		title := fs.String("title", "", "the title, for a record entered by hand")
		authors := listFlag(fs, "author", "an author, repeatable, for a record entered by hand")
		year := fs.String("year", "", "the year, for a record entered by hand")
		container := fs.String("container", "", "the journal, or the book a chapter is in")
		publisher := fs.String("publisher", "", "the publisher")
		volume := fs.String("volume", "", "the volume")
		issue := fs.String("issue", "", "the issue or number")
		pages := fs.String("pages", "", "the page range")
		edition := fs.String("edition", "", "the edition")
		doi := fs.String("doi", "", "a DOI")
		arxiv := fs.String("arxiv", "", "an arXiv identifier")
		isbn := fs.String("isbn", "", "an ISBN")
		url := fs.String("url", "", "a URL")
		path := fs.String("path", "", "a path to a local file")
		o.registerKB(fs)

		return func(c *command, w *output, args []string) int {
			if len(args) > 1 {
				return w.fail(fmt.Errorf("cite add takes at most one identifier"))
			}

			// A positional argument that names a file on this machine is a local
			// source, not an identifier: there is no resolver for a path, so it
			// fills the same field --path does and the record is built by hand.
			if len(args) == 1 && *path == "" {
				if info, err := os.Stat(args[0]); err == nil && !info.IsDir() {
					*path = args[0]
					args = nil
				}
			}

			fields := source.Fields{
				Type: *typ, Key: *key, Title: *title, Authors: *authors,
				Year: *year, Container: *container, Publisher: *publisher,
				Volume: *volume, Issue: *issue, Pages: *pages, Edition: *edition,
				DOI: *doi, ArXiv: *arxiv, ISBN: *isbn, URL: *url, Path: *path,
			}
			handEntered := *title != "" || len(*authors) > 0 || *year != "" ||
				*container != "" || *publisher != "" || *volume != "" || *issue != "" ||
				*pages != "" || *edition != "" || *doi != "" || *arxiv != "" || *isbn != "" ||
				*url != "" || *path != ""

			k, code := o.load(w)
			if k == nil {
				return code
			}

			res := newResolver()
			res.Offline = *offline

			var entry *kb.BibEntry
			var record []byte

			switch {
			case len(args) == 1 && handEntered:
				return w.fail(fmt.Errorf(
					"an identifier carries its own metadata; drop the field flags and use them only for a record entered by hand"))
			case len(args) == 1:
				result, err := res.ResolveResult(context.Background(), args[0])
				if err != nil {
					return failResolution(w, args[0], err)
				}
				entry, record = result.Entry, result.Record
			default:
				if !handEntered {
					return w.fail(fmt.Errorf(
						"cite add needs an identifier, or the fields of a record to add by hand"))
				}
				var err error
				entry, err = source.Manual(fields)
				if err != nil {
					return w.fail(err)
				}
			}

			// --key and --type apply to either path: they say how the record
			// should be filed, which is the author's choice and not a fact a
			// resolver owns.
			if *key != "" {
				entry.SetKey(*key)
			}
			if *typ != "" {
				entry.SetType(*typ)
			}
			if len(record) > 0 {
				entry.Set(kb.FieldContentHash, "{"+kb.HashOf(record)+"}")
			}

			return writeEntry(w, k, entry, *force, *dryRun)
		}
	},
}

// citeAddReport is the payload of `cite add --json`.
type citeAddReport struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	Entry  string `json:"entry,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// writeEntry finds the entry's work in the bibliography and updates it, or
// appends the entry when it is new or --force says so.
func writeEntry(w *output, k *kb.KB, entry *kb.BibEntry, force, dryRun bool) int {
	sources, err := k.BibSources()
	if err != nil {
		return w.fail(err)
	}
	type loaded struct {
		name string
		file *kb.BibFile
	}
	var files []loaded
	for _, name := range sources {
		f, err := k.ReadBibliography(name)
		if err != nil {
			return w.fail(err)
		}
		files = append(files, loaded{name, f})
	}

	var (
		target    string
		written   *kb.BibFile
		out       *kb.BibEntry
		action    = "added"
		match     *kb.BibEntry
		matchFile = -1
	)
	if !force {
		for i := range files {
			for _, e := range files[i].file.Entries() {
				if kb.SameWork(e, entry) {
					match, matchFile = e, i
					break
				}
			}
			if match != nil {
				break
			}
		}
	}

	if match != nil {
		match.MergeFrom(entry)
		target, written, out, action = files[matchFile].name, files[matchFile].file, match, "updated"
	} else {
		f, name, err := k.BibliographyFor(entry.Key())
		if err != nil {
			return w.fail(err)
		}
		f.AddEntry(entry)
		target, written, out = name, f, entry
	}

	if !dryRun {
		if err := k.WriteBibliography(target, written); err != nil {
			return w.fail(err)
		}
	}

	report := citeAddReport{
		Action: action,
		Key:    out.Key(),
		Type:   out.Type(),
		Path:   target,
		Entry:  string(out.Bytes()),
		DryRun: dryRun,
	}
	if w.json {
		return w.emit(report)
	}
	verb := action
	if dryRun {
		verb = "would " + action
	}
	fmt.Fprintf(w.stdout, "%s %s in %s\n", verb, out.Key(), target)
	if dryRun {
		fmt.Fprint(w.stdout, report.Entry)
	}
	return ExitOK
}

// failResolution turns a resolution failure into an exit code. A network
// failure is one the run could not complete; an identifier that names nothing,
// or is malformed, is a finding about what was asked for.
func failResolution(w *output, id string, err error) int {
	if source.Operational(err) {
		return w.fail(err)
	}
	if w.json {
		return w.report(nil, []kb.Finding{{
			Severity: kb.Warning,
			Code:     CodeCiteUnresolved,
			Message:  err.Error(),
		}})
	}
	fmt.Fprintf(w.stderr, "stemma: %v\n", err)
	return ExitFindings
}

// envBool reads an environment variable as a flag would.
func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "", "0", "false", "no", "off":
		return false
	}
	return true
}

// citeListCommand implements `stemma cite list`.
//
// It lists records, not citations, and it filters rather than searches: every
// filter is a property of a record already in memory. `--uncited` is the one
// that earns its place, because a bibliography's quiet problem is the source
// nothing points at.
var citeListCommand = &command{
	name:    "list",
	summary: "list the bibliography, with filters",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		typ := fs.String("type", "", "only entries of this type")
		author := fs.String("author", "", "only entries with an author matching this text")
		year := fs.String("year", "", "only entries from this year")
		uncited := fs.Bool("uncited", false, "only entries no page cites")
		cited := fs.Bool("cited", false, "only entries some page cites")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("cite list takes no arguments, got %q", args[0]))
			}
			if *uncited && *cited {
				return w.fail(fmt.Errorf("--uncited and --cited ask for opposite things"))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}

			rows := citeRows(k, *typ, *author, *year, *uncited, *cited)
			if w.json {
				return w.emit(rows)
			}
			printCiteRows(w.stdout, rows)
			return ExitOK
		}
	},
}

// citeRow is one line of `cite list`.
type citeRow struct {
	Key    string `json:"key"`
	Type   string `json:"type"`
	Year   string `json:"year,omitempty"`
	Author string `json:"author,omitempty"`
	Title  string `json:"title,omitempty"`
	Cited  bool   `json:"cited"`
}

func citeRows(k *kb.KB, typ, author, year string, uncited, cited bool) []citeRow {
	rows := []citeRow{}
	for _, key := range k.Bibliography.SortedKeys() {
		e, ok := k.Bibliography.Entry(key)
		if !ok {
			continue
		}
		if typ != "" && !strings.EqualFold(e.Type(), typ) {
			continue
		}
		if year != "" && e.Year() != year {
			continue
		}
		if author != "" && !authorMatches(e, author) {
			continue
		}
		isCited := len(k.Graph.CitedBy(key)) > 0
		if uncited && isCited {
			continue
		}
		if cited && !isCited {
			continue
		}
		rows = append(rows, citeRow{
			Key:    key,
			Type:   e.Type(),
			Year:   e.Year(),
			Author: firstAuthor(e),
			Title:  e.Title(),
			Cited:  isCited,
		})
	}
	return rows
}

func printCiteRows(out io.Writer, rows []citeRow) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Key, r.Type, r.Year, r.Author, r.Title)
	}
	tw.Flush()
}

func authorMatches(e *kb.BibEntry, needle string) bool {
	needle = strings.ToLower(needle)
	for _, a := range e.Authors() {
		if strings.Contains(strings.ToLower(a), needle) {
			return true
		}
	}
	return false
}

func firstAuthor(e *kb.BibEntry) string {
	if a := e.Authors(); len(a) > 0 {
		return a[0]
	}
	return ""
}

// citeShowCommand implements `stemma cite show`.
//
// It shows the record as it is written, not as it would be formatted: the
// entry's own bytes, every field verbatim beside its decoded value, where the
// record lives, and which pages cite it. Those are the four things a person
// needs to decide whether the record is right.
var citeShowCommand = &command{
	name:    "show",
	summary: "show one bibliography entry",
	args:    "KEY",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("cite show takes one key"))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}
			key := args[0]
			e, ok := k.Bibliography.Entry(key)
			if !ok {
				return w.fail(fmt.Errorf("the bibliography has no entry for %q", key))
			}

			report := citeShowReport{
				Key:     key,
				Type:    e.Type(),
				Path:    k.Bibliography.PathOf(key),
				Entry:   string(e.Bytes()),
				Fields:  citeFields(e),
				CitedBy: k.Graph.CitedBy(key),
			}
			if w.json {
				return w.emit(report)
			}
			fmt.Fprint(w.stdout, report.Entry, "\n")
			fmt.Fprintf(w.stdout, "defined in %s\n", report.Path)
			if len(report.CitedBy) == 0 {
				fmt.Fprintln(w.stdout, "cited by nothing")
				return ExitOK
			}
			fmt.Fprintf(w.stdout, "cited by %s\n", strings.Join(report.CitedBy, ", "))
			return ExitOK
		}
	},
}

// citeShowField is one field as written and as read.
type citeShowField struct {
	Name  string `json:"name"`
	Raw   string `json:"raw"`
	Value string `json:"value"`
}

type citeShowReport struct {
	Key     string          `json:"key"`
	Type    string          `json:"type"`
	Path    string          `json:"path"`
	Entry   string          `json:"entry"`
	Fields  []citeShowField `json:"fields"`
	CitedBy []string        `json:"cited_by"`
}

func citeFields(e *kb.BibEntry) []citeShowField {
	out := []citeShowField{}
	for _, f := range e.Fields() {
		v, _ := e.Value(f.Name)
		out = append(out, citeShowField{Name: f.Name, Raw: f.Raw, Value: v})
	}
	return out
}

// citeCitedByCommand implements `stemma cite cited-by`.
var citeCitedByCommand = &command{
	name:    "cited-by",
	summary: "list the pages citing a key",
	args:    "KEY",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("cite cited-by takes one key"))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}
			key := args[0]
			if !k.Bibliography.Has(key) {
				return w.fail(fmt.Errorf("the bibliography has no entry for %q", key))
			}
			pages := k.Graph.CitedBy(key)
			if w.json {
				return w.emit(citeCitedByReport{Key: key, Pages: pages})
			}
			for _, p := range pages {
				fmt.Fprintln(w.stdout, p)
			}
			return ExitOK
		}
	},
}

type citeCitedByReport struct {
	Key   string   `json:"key"`
	Pages []string `json:"pages"`
}

// citeCheckCommand implements `stemma cite check`.
//
// It reports findings and exits 1, like lint, and the two share the codes and
// the renderer so that a person reading either reads the same language.
var citeCheckCommand = &command{
	name:    "check",
	summary: "report what is wrong with the bibliography",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("cite check takes no arguments, got %q", args[0]))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}
			findings := k.CheckFindings()
			if w.json {
				return w.report(nil, findings)
			}
			return reportText(w.stdout, w.stderr, findings)
		}
	},
}

// citeExportCommand implements `stemma cite export`.
//
// It is the one command that produces output meant for another tool, so it
// writes bytes rather than a report: BibTeX for a reference manager, CSL-JSON
// for a citation processor. `--cited` narrows the export to the records some
// page actually relies on, which is the difference between shipping a KB's
// evidence and shipping its whole working library.
var citeExportCommand = &command{
	name:    "export",
	summary: "export the bibliography as BibTeX or CSL-JSON",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		format := fs.String("format", kb.FormatBibTeX, "output format: "+strings.Join(kb.ExportFormats(), " or "))
		cited := fs.Bool("cited", false, "only entries some page cites")
		dest := fs.String("o", "", "write to this file instead of standard output")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("cite export takes no arguments, got %q", args[0]))
			}
			if !kb.KnownExportFormat(*format) {
				return w.fail(fmt.Errorf("unknown format %q; use %s",
					*format, strings.Join(kb.ExportFormats(), " or ")))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}

			entries := k.Bibliography.Entries()
			if *cited {
				entries = citedEntries(k, entries)
			}

			var data []byte
			switch *format {
			case kb.FormatBibTeX:
				data = kb.ExportBibTeX(entries)
			case kb.FormatCSLJSON:
				b, err := kb.ExportCSLJSON(entries)
				if err != nil {
					return w.fail(err)
				}
				data = b
			}

			if *dest != "" {
				if err := os.WriteFile(*dest, data, 0o644); err != nil {
					return w.fail(err)
				}
				fmt.Fprintf(w.stdout, "exported %d entries to %s\n", len(entries), *dest)
				return ExitOK
			}
			if _, err := w.stdout.Write(data); err != nil {
				return w.fail(err)
			}
			return ExitOK
		}
	},
}

// citedEntries keeps the records whose key some page cites.
func citedEntries(k *kb.KB, entries []*kb.BibEntry) []*kb.BibEntry {
	cited := map[string]bool{}
	for _, key := range k.Graph.CitedKeys() {
		cited[key] = true
	}
	out := make([]*kb.BibEntry, 0, len(entries))
	for _, e := range entries {
		if cited[e.Key()] {
			out = append(out, e)
		}
	}
	return out
}

// Package export turns a KB into the machine-readable dump other systems read.
//
// It is scenario 5 of the export family (`D33`): where `build` produces
// something a person reads and `export page` produces a KB another person can
// keep, this produces something a program reads. The dump is one JSON document
// holding the pages, the links between them, the citations they carry, and the
// bibliography — everything needed to rebuild the graph without opening the KB
// tree, which is the point of it.
//
// One document rather than JSONL because a KB is a graph and a graph is not a
// stream: the whole answer is one object, and a consumer that wants records
// iterates an array. It also keeps the contract to one shape — no per-line
// discriminator, and no record order that becomes something to keep stable. The
// records are the same records, so a JSONL form can be added later without
// changing this one.
package export

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/render"
)

// Version is the dump's own schema version, and not the KB's. A consumer reads
// it to know which shape it is looking at; the format itself carries no version
// field (`D26`), and this one exists because a machine-readable artifact has
// nothing else to tell a reader that the shape moved.
const Version = 1

// The names a link's resolution is reported under. They are the words the
// renderer and the linter use for the same three outcomes, so a finding and a
// dump never describe one link differently.
const (
	Resolved   = "resolved"
	Unresolved = "unresolved"
	Ambiguous  = "ambiguous"
)

// Counts is how much the dump holds, so a caller can report on it without
// re-reading the file it just wrote.
type Counts struct {
	Pages        int `json:"pages"`
	Links        int `json:"links"`
	Citations    int `json:"citations"`
	Bibliography int `json:"bibliography"`
}

// Dump is the whole export.
//
// Every list is present even when it is empty, so a consumer never has to tell
// "none" from "not in this version".
type Dump struct {
	// Version is the schema version of this document.
	Version int `json:"version"`

	// KB is what the dump knows about the corpus it came from.
	KB Source `json:"kb"`

	// Counts is the length of each list below it.
	Counts Counts `json:"counts"`

	Pages        []Page     `json:"pages"`
	Links        []Link     `json:"links"`
	Citations    []Citation `json:"citations"`
	Bibliography []Entry    `json:"bibliography"`
}

// Source is the dump's description of the KB itself.
type Source struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// CitationStyle is the style the entries are formatted in, as the manifest
	// names it or as the default fills it in.
	CitationStyle string `json:"citation_style"`
}

// Page is one page. It carries the body as well as the metadata, so the dump
// stands alone: a consumer can read the prose without the KB tree. The body is
// the markdown exactly as it is on disk, frontmatter excluded, because the
// fields above are the frontmatter already read.
type Page struct {
	Path          string   `json:"path"`
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	Type          string   `json:"type"`
	Status        string   `json:"status"`
	Tags          []string `json:"tags"`
	Aliases       []string `json:"aliases"`
	ArchiveReason string   `json:"archive_reason,omitempty"`
	Body          string   `json:"body"`
}

// Link is one wikilink, with what it resolved to.
//
// Target is what the author wrote; To is the page it reached, present only when
// exactly one page answered to it. Matches names every claimant when more than
// one did, which is an ambiguity the KB has rather than something the dump
// resolved.
type Link struct {
	From       string   `json:"from"`
	Target     string   `json:"target"`
	Line       int      `json:"line"`
	Resolution string   `json:"resolution"`
	To         string   `json:"to,omitempty"`
	Matches    []string `json:"matches,omitempty"`
}

// Citation is one key cited by one page. Resolved says whether the bibliography
// defines it, so a consumer can see that a reference is short without also
// holding the bibliography.
type Citation struct {
	From     string `json:"from"`
	Key      string `json:"key"`
	Line     int    `json:"line"`
	Locator  string `json:"locator,omitempty"`
	Resolved bool   `json:"resolved"`
}

// Entry is one bibliography record.
//
// Fields is the entry exactly as written, which is what makes the dump lossless
// for fields this tool has no opinion about. Title, Year, Authors and URL are
// the tool's own readings of the same entry — the ones the citation formatter
// uses — because a year is a date string in BibTeX and a normalised year here.
type Entry struct {
	Key     string            `json:"key"`
	Type    string            `json:"type"`
	Title   string            `json:"title,omitempty"`
	Year    string            `json:"year,omitempty"`
	Authors []string          `json:"authors,omitempty"`
	URL     string            `json:"url,omitempty"`
	Path    string            `json:"path,omitempty"`
	Fields  map[string]string `json:"fields"`
}

// Result is where a dump went and what it held.
type Result struct {
	Out   string `json:"out"`
	Bytes int    `json:"bytes"`
	Counts
}

// Assemble reads a loaded KB into a dump.
//
// The result is deterministic: pages in path order, each page's links and
// citations in the order they appear in its body, and the bibliography in key
// order. Two runs over one KB therefore produce the same bytes, which is what
// makes the dump diffable and a golden test meaningful.
//
// Drafts are not in it. `kb.Load` does not read `inbox/`, because a draft is not
// part of the knowledge, and an export is a view of the knowledge.
func Assemble(k *kb.KB) *Dump {
	style := k.Manifest.CitationStyle
	if style == "" {
		style = kb.DefaultCitationStyle
	}
	d := &Dump{
		Version: Version,
		KB: Source{
			Title:         k.Manifest.Title,
			Description:   k.Manifest.Description,
			CitationStyle: style,
		},
		Pages:        []Page{},
		Links:        []Link{},
		Citations:    []Citation{},
		Bibliography: []Entry{},
	}

	for _, path := range k.Graph.Paths() {
		p, ok := k.Graph.Page(path)
		if !ok {
			continue
		}
		d.Pages = append(d.Pages, Page{
			Path:          path,
			URL:           render.PageURL(path),
			Title:         p.Title(),
			Type:          p.Type(),
			Status:        p.Status(),
			Tags:          nonNil(p.Tags()),
			Aliases:       nonNil(p.Aliases()),
			ArchiveReason: p.ArchiveReason(),
			Body:          string(p.Body()),
		})
		for _, l := range k.Graph.Links(path) {
			d.Links = append(d.Links, linkOf(path, l, k.Graph))
		}
		for _, c := range k.Graph.Citations(path) {
			d.Citations = append(d.Citations, citationOf(path, c, k.Bibliography))
		}
	}

	if k.Bibliography != nil {
		for _, key := range k.Bibliography.SortedKeys() {
			if e, ok := k.Bibliography.Entry(key); ok {
				d.Bibliography = append(d.Bibliography, entryOf(e))
			}
		}
	}

	d.Counts = Counts{
		Pages:        len(d.Pages),
		Links:        len(d.Links),
		Citations:    len(d.Citations),
		Bibliography: len(d.Bibliography),
	}
	return d
}

// Bytes renders the dump as indented JSON.
//
// HTML is not escaped, so a body keeps the angle brackets its author wrote
// rather than reading as \u003c. An encoder is used rather than MarshalIndent
// for exactly that reason, and it leaves the trailing newline a text file
// should end with.
func (d *Dump) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Write assembles the dump of a loaded KB and writes it to path, creating the
// directory it names when that is missing.
//
// The write is atomic, so an interrupted export does not leave a truncated
// document where a consumer expects a whole one.
func Write(k *kb.KB, path string) (Result, error) {
	d := Assemble(k)
	b, err := d.Bytes()
	if err != nil {
		return Result{}, err
	}
	if err := writeFile(filepath.Dir(path), filepath.Base(path), b); err != nil {
		return Result{}, err
	}
	return Result{Out: path, Bytes: len(b), Counts: d.Counts}, nil
}

// linkOf is one wikilink as the dump carries it.
func linkOf(from string, l kb.Link, g *kb.Graph) Link {
	out := Link{From: from, Target: l.Target, Line: l.Line}
	switch res := g.Resolve(l.Name); res.Kind {
	case kb.Resolved:
		out.Resolution, out.To = Resolved, res.Path
	case kb.Ambiguous:
		out.Resolution, out.Matches = Ambiguous, res.Matches
	default:
		out.Resolution = Unresolved
	}
	return out
}

// citationOf is one citation as the dump carries it. The citation's form —
// its group, its prefix, whether it is narrative — is how it should be
// rendered, and rendering is not what the dump is for; the locator stays,
// because it is where in the source the author was pointing.
func citationOf(from string, c kb.Citation, b *kb.Bibliography) Citation {
	resolved := false
	if b != nil {
		_, resolved = b.Entry(c.Key)
	}
	return Citation{From: from, Key: c.Key, Line: c.Line, Locator: c.Locator, Resolved: resolved}
}

// entryOf is one bibliography record as the dump carries it.
func entryOf(e *kb.BibEntry) Entry {
	out := Entry{
		Key:     e.Key(),
		Type:    e.Type(),
		Title:   e.Title(),
		Year:    e.Year(),
		Authors: e.Authors(),
		URL:     e.URL(),
		Path:    e.Path(),
		Fields:  map[string]string{},
	}
	for _, f := range e.Fields() {
		// Fields are held as written, delimiters and all, so a value is read
		// back through Value rather than taken from Raw.
		if v, ok := e.Value(f.Name); ok {
			out.Fields[f.Name] = v
		}
	}
	return out
}

// nonNil turns a nil slice into an empty one, so a list in the dump is [] rather
// than null and a consumer never has to handle both.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

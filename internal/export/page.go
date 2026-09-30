package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// The reasons a construct could not come along, as the report names them.
const (
	// ReasonOutside is a link whose page exists but is not in the slice.
	ReasonOutside = "outside"
	// ReasonUnresolved is a link no page answers to.
	ReasonUnresolved = "unresolved"
	// ReasonAmbiguous is a link more than one page answers to.
	ReasonAmbiguous = "ambiguous"
	// ReasonMissingKey is a citation the bibliography does not define.
	ReasonMissingKey = "missing-key"
)

// stateDir and stateName are where an extract records what it wrote. `.stemma/`
// is where a KB keeps what it derives, and this is derived: it is how a second
// run into the same directory refreshes an extract instead of adding to it, and
// it is the only thing a write is ever allowed to remove.
const stateName = "extract.json"

// Pruned is one construct an extract could not keep, and what became of it.
//
// It is the report `D51` asks for: a summary by default, this list on request,
// and in the JSON payload either way so a program can act on it.
type Pruned struct {
	Page   string `json:"page"`
	Line   int    `json:"line"`
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Reason string `json:"reason"`
	Became string `json:"became"`
}

// File is one file of an extract.
type File struct {
	Path string
	Body []byte
}

// Extract is a scoped export: one page, the pages it links to, and nothing
// else, written as a KB root in its own right (`D30`).
type Extract struct {
	// Out is where it was written, filled in by Write.
	Out string `json:"out,omitempty"`

	// Root is the page it is of, as a path in the source KB.
	Root string `json:"root"`

	// Depth is how many hops it reached; 0 means the whole reachable set.
	Depth int `json:"depth"`

	// Entry is where a reader should start, relative to the extract root. It is
	// the root page, under the entry document's name.
	Entry string `json:"entry"`

	// Pages is every page taken from the source, in the order it was reached.
	Pages []string `json:"pages"`

	// Sources is every bibliography key materialised into the extract.
	Sources []string `json:"sources"`

	// Pruned is every construct that could not be kept, in page order.
	Pruned []Pruned `json:"pruned"`

	// Files is the whole extract. It is not in the JSON report: the report is
	// about the extract, and Write is what puts the extract on disk.
	Files []File `json:"-"`
}

// Scoped builds the extract of root and the pages it links to, to depth hops. A
// depth of 0 reaches the whole set instead of stopping.
//
// The result is a KB root in the format rather than a rendering of one, because
// an extract is meant to be kept and worked on: every tool that reads a KB reads
// it, which is what `D30` asks for. Only two things change on the way out. The
// root page takes the entry document's place, so the page the extract is of is
// the page a reader arrives at and the one page nothing links to is the one page
// the format already expects nothing to link to. And a construct that cannot
// come along — a link out of the slice, a link that named nothing, a citation
// with no entry — becomes its own anchor text, so the prose still reads and
// nothing dangles (`D44`, `D51`, `D31`).
func Scoped(k *kb.KB, root string, depth int) (*Extract, error) {
	page, ok := k.Graph.Page(root)
	if !ok {
		return nil, fmt.Errorf("no page at %s", root)
	}
	if depth < 0 {
		return nil, fmt.Errorf("depth is %d; it is zero or more", depth)
	}

	order := walk(k.Graph, root, depth)
	inSlice := make(map[string]bool, len(order))
	for _, p := range order {
		inSlice[p] = true
	}

	x := &Extract{
		Root:    root,
		Depth:   depth,
		Entry:   kb.EntryDocument,
		Pages:   order,
		Sources: []string{},
		Pruned:  []Pruned{},
		Files:   make([]File, 0, len(order)+2),
	}

	cited := map[string]bool{}
	taken := map[string]bool{}
	for i, p := range order {
		src, ok := k.Graph.Page(p)
		if !ok {
			continue
		}
		// A copy, because the extract rewrites the body and the KB it was read
		// from must come back out of this unchanged.
		out, err := kb.ParsePage(src.Bytes())
		if err != nil {
			return nil, err
		}
		out.RewriteBody(func(in kb.Inline, text string) (string, bool) {
			if in.Link != nil {
				kept, replacement, reason := keepLink(k, in.Link, inSlice)
				if kept {
					return "", false
				}
				x.Pruned = append(x.Pruned, Pruned{Page: p, Line: in.Link.Line, Kind: "link",
					Target: in.Link.Target, Reason: reason, Became: replacement})
				return replacement, true
			}
			if len(in.Cites) == 0 {
				return "", false
			}
			kept, replacement, reason := keepCitations(k, in.Cites, text)
			if kept {
				for _, c := range in.Cites {
					cited[c.Key] = true
				}
				return "", false
			}
			x.Pruned = append(x.Pruned, Pruned{Page: p, Line: in.Cites[0].Line, Kind: "citation",
				Target: citationKeys(in.Cites), Reason: reason, Became: replacement})
			return replacement, true
		})
		x.Files = append(x.Files, File{Path: writtenPath(p, i == 0, taken), Body: out.Bytes()})
	}

	// Citation closure (`D31`): the entries the extract cites and only those, so
	// no reference dangles and no entry is carried that nothing points at. A key
	// that is cited but not defined is not here — its group was pruned above.
	entries := make([]*kb.BibEntry, 0, len(cited))
	if k.Bibliography != nil {
		for _, key := range k.Bibliography.SortedKeys() {
			if !cited[key] {
				continue
			}
			if e, ok := k.Bibliography.Entry(key); ok {
				entries = append(entries, e)
				x.Sources = append(x.Sources, key)
			}
		}
	}

	x.Files = append(x.Files, File{Path: kb.ManifestName, Body: manifestFor(k, page, depth)})
	if len(entries) > 0 {
		x.Files = append(x.Files, File{Path: kb.BibliographyName, Body: kb.ExportBibTeX(entries)})
	}
	return x, nil
}

// Write writes an extract into dir, which becomes a KB root.
//
// A directory that already holds an extract is refreshed: the files it wrote
// last time and this time are written, the ones it wrote last time and not this
// time are removed, and everything else is left where it is. A directory that is
// neither empty nor an extract is refused rather than written into, because
// scattering a KB root through someone's files is not a thing to do quietly.
func (x *Extract) Write(dir string) error {
	old, err := readState(dir)
	if err != nil {
		return err
	}
	if old == nil && !isEmpty(dir) {
		return fmt.Errorf("%s is not empty and holds no extract; write the extract into a fresh directory", dir)
	}

	written := make([]string, 0, len(x.Files))
	for _, f := range x.Files {
		if err := writeFile(dir, f.Path, f.Body); err != nil {
			return err
		}
		written = append(written, f.Path)
	}
	if old != nil {
		keep := make(map[string]bool, len(written))
		for _, p := range written {
			keep[p] = true
		}
		for _, p := range old.Files {
			if !keep[p] {
				removeFile(dir, p)
			}
		}
	}

	state, err := json.MarshalIndent(stateFile{Root: x.Root, Files: written}, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(dir, statePath(), append(state, '\n')); err != nil {
		return err
	}
	x.Out = dir
	return nil
}

// stateFile is what an extract records about itself.
type stateFile struct {
	Root  string   `json:"root"`
	Files []string `json:"files"`
}

func statePath() string { return filepath.Join(kb.GeneratedDir, stateName) }

// walk returns the pages an extract reaches, root first and then in the order
// they were discovered. Only resolved links are followed, because a link that
// does not name exactly one page has no page to follow to.
func walk(g *kb.Graph, root string, depth int) []string {
	order := []string{root}
	seen := map[string]bool{root: true}
	frontier := []string{root}

	for hop := 1; depth == 0 || hop <= depth; hop++ {
		var next []string
		for _, p := range frontier {
			for _, l := range g.Links(p) {
				res := g.Resolve(l.Name)
				if res.Kind != kb.Resolved || seen[res.Path] {
					continue
				}
				seen[res.Path] = true
				next = append(next, res.Path)
				order = append(order, res.Path)
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = next
	}
	return order
}

// keepLink decides whether a link can come along.
//
// A link is kept when it names exactly one page and that page is in the slice,
// which leaves three ways to lose one: it resolved to a page the extract does
// not hold, it named nothing, or it named more than one page. The replacement is
// the anchor text a reader of the source would have seen (`D69`), so an extract
// reads the way the page did, with the links that could not come along turned
// back into prose (`D51`).
func keepLink(k *kb.KB, l *kb.Link, inSlice map[string]bool) (kept bool, replacement, reason string) {
	switch res := k.Graph.Resolve(l.Name); {
	case res.Kind == kb.Resolved && inSlice[res.Path]:
		return true, "", ""
	case res.Kind == kb.Resolved:
		if target, ok := k.Graph.Page(res.Path); ok && target.Title() != "" {
			return false, target.Title(), ReasonOutside
		}
		return false, l.Target, ReasonOutside
	case res.Kind == kb.Ambiguous:
		return false, l.Target, ReasonAmbiguous
	default:
		return false, l.Target, ReasonUnresolved
	}
}

// keepCitations decides whether a citation group can come along.
//
// The group is the unit, because it is one parenthetical: "[@a; @b]" is read and
// rewritten as one construct, and rebuilding it from the keys that survived
// would be writing markdown the author did not write. So a group is kept when
// every key in it is defined, and otherwise it becomes its own text with the
// "@"s taken out: prose that still reads, and that no longer parses as a
// citation pointing at nothing.
//
// A defined key never arrives here as a pruned group, because references are
// closed over before this runs.
func keepCitations(k *kb.KB, cites []kb.Citation, text string) (kept bool, replacement, reason string) {
	for _, c := range cites {
		if k.Bibliography == nil || !k.Bibliography.Has(c.Key) {
			return false, strings.ReplaceAll(text, "@", ""), ReasonMissingKey
		}
	}
	return true, "", ""
}

// citationKeys names a group's keys, for a report that has to say which group it
// could not keep.
func citationKeys(cites []kb.Citation) string {
	keys := make([]string, 0, len(cites))
	for _, c := range cites {
		keys = append(keys, c.Key)
	}
	return strings.Join(keys, "; ")
}

// writtenPath is where a page lives inside the extract.
//
// The root takes the entry document's place, so the page the extract is of is
// the page a reader arrives at, and the one page nothing links to is the one the
// orphan check already exempts. Every other page keeps its own path; a page that
// would land on a path already taken — the source's own entry document, when the
// slice holds it — takes its name with a number instead. A file name is free to
// change because links resolve by name, not by path.
func writtenPath(src string, isRoot bool, taken map[string]bool) string {
	want := src
	if isRoot {
		want = kb.EntryDocument
	}
	if !taken[want] {
		taken[want] = true
		return want
	}
	stem := strings.TrimSuffix(want, ".md")
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d.md", stem, n)
		if !taken[candidate] {
			taken[candidate] = true
			return candidate
		}
	}
}

// manifestFor is the extract's manifest: the root page's title, and the source's
// vocabulary, default type, citation style and export defaults, so that every
// word the pages use is still a word the extract's KB knows. The ignore list is
// not carried over: an extract holds nothing it would want to ignore.
func manifestFor(k *kb.KB, root *kb.Page, depth int) []byte {
	title := root.Title()
	if title == "" {
		title = k.Manifest.Title
	}
	var about string
	switch depth {
	case 0:
		about = fmt.Sprintf("An extract of %s: the page %q and everything it reaches.", k.Manifest.Title, title)
	case 1:
		about = fmt.Sprintf("An extract of %s: the page %q and the pages it links to.", k.Manifest.Title, title)
	default:
		about = fmt.Sprintf("An extract of %s: the page %q and the pages within %d hops of it.", k.Manifest.Title, title, depth)
	}
	return kb.Manifest{
		Title:         title,
		Description:   about,
		DefaultType:   k.Manifest.DefaultType,
		Types:         k.Manifest.Types,
		CitationStyle: k.Manifest.CitationStyle,
		Export:        k.Manifest.Export,
	}.TOML()
}

// writeFile writes one file of an extract, making the directory it needs.
func writeFile(dir, name string, body []byte) error {
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return kb.WriteFileAtomic(full, body)
}

// removeFile deletes a file an earlier extract wrote, and the directories that
// leaves empty.
func removeFile(dir, name string) {
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.Remove(full); err != nil {
		return
	}
	for p := filepath.Dir(full); p != dir && strings.HasPrefix(p, dir+string(filepath.Separator)); p = filepath.Dir(p) {
		if err := os.Remove(p); err != nil {
			return // fails while not empty
		}
	}
}

// readState returns what an earlier extract into dir recorded, or nil when there
// was none.
func readState(dir string) (*stateFile, error) {
	b, err := os.ReadFile(filepath.Join(dir, statePath()))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s stateFile
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s is not readable: %w", filepath.Join(dir, statePath()), err)
	}
	return &s, nil
}

// isEmpty reports whether dir holds nothing. A directory that is not there is
// empty for this purpose.
func isEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return os.IsNotExist(err)
	}
	return len(entries) == 0
}

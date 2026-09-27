package kb

import (
	"sort"
	"strings"
)

// Finding codes raised across pages rather than within one page.
const (
	CodeUnresolvedLink = "unresolved-link"
	CodeAmbiguousLink  = "ambiguous-link"
	CodeNameCollision  = "name-collision"
	CodeOrphanPage     = "orphan-page"
)

// ResolutionKind says how a link resolved.
type ResolutionKind int

const (
	// Resolved means exactly one page answers to the name.
	Resolved ResolutionKind = iota
	// Unresolved means no page does.
	Unresolved
	// Ambiguous means more than one does.
	Ambiguous
)

// Resolution is the outcome of resolving one link name.
type Resolution struct {
	Kind ResolutionKind

	// Path is the page the name resolved to, when exactly one matched.
	Path string

	// Matches is every page claiming the name, sorted, when more than one did.
	Matches []string
}

// Graph is the Tier-0 view of a KB: the pages, the names they answer to, and
// the links between them, computed in memory from the pages themselves.
//
// There is no index and nothing generated, so nothing can go stale and nothing
// has to be built before a command can run. It stays fast because every
// operation is proportional to the pages and links involved: a name resolves
// through one map lookup, and adding one page touches only that page's own
// names and links. Nothing here is quadratic in the size of the KB.
type Graph struct {
	pages map[string]*Page

	// claims maps a normalised name to the paths answering to it.
	claims map[string]map[string]bool
	// links maps a path to its outgoing links, in order.
	links map[string][]Link
	// linked maps a normalised name to the paths whose bodies link to it. It is
	// keyed by name rather than by path so that a page can be added before the
	// page it links to exists, and the backlink is still found once it does.
	linked map[string]map[string]bool
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{
		pages:  map[string]*Page{},
		claims: map[string]map[string]bool{},
		links:  map[string][]Link{},
		linked: map[string]map[string]bool{},
	}
}

// Add puts a page in the graph at a path, replacing whatever was there.
//
// Adding a page touches only that page, so a KB can be kept current one edit at
// a time rather than re-resolved from scratch after every change.
func (g *Graph) Add(path string, p *Page) {
	g.Remove(path)
	g.pages[path] = p

	links := p.Links()
	g.links[path] = links
	for _, l := range links {
		insertInto(g.linked, l.Name, path)
	}
	for _, name := range pageNames(p) {
		insertInto(g.claims, name, path)
	}
}

// Remove takes a page out of the graph.
func (g *Graph) Remove(path string) {
	p, ok := g.pages[path]
	if !ok {
		return
	}
	delete(g.pages, path)
	for _, name := range pageNames(p) {
		removeFrom(g.claims, name, path)
	}
	for _, l := range g.links[path] {
		removeFrom(g.linked, l.Name, path)
	}
	delete(g.links, path)
}

// Len returns the number of pages in the graph.
func (g *Graph) Len() int { return len(g.pages) }

// Paths returns every path in the graph, sorted.
func (g *Graph) Paths() []string { return sortedKeys(g.pages) }

// Page returns the page at a path.
func (g *Graph) Page(path string) (*Page, bool) {
	p, ok := g.pages[path]
	return p, ok
}

// Links returns a page's outgoing links, in the order they appear.
func (g *Graph) Links(path string) []Link { return g.links[path] }

// Names returns every name claimed in the graph, sorted.
func (g *Graph) Names() []string { return sortedKeys(g.claims) }

// Claimants returns the pages answering to a name, sorted.
func (g *Graph) Claimants(name string) []string {
	return sortedKeys(g.claims[Normalize(name)])
}

// Resolve finds the single page a name refers to.
func (g *Graph) Resolve(name string) Resolution {
	paths := g.Claimants(name)
	switch len(paths) {
	case 0:
		return Resolution{Kind: Unresolved}
	case 1:
		return Resolution{Kind: Resolved, Path: paths[0]}
	}
	return Resolution{Kind: Ambiguous, Matches: paths}
}

// Backlinks returns the pages linking to a page, sorted.
//
// A page linking to itself is not its own backlink, so a page that nothing else
// points at is still an orphan.
func (g *Graph) Backlinks(path string) []string {
	p, ok := g.pages[path]
	if !ok {
		return nil
	}
	found := map[string]bool{}
	for _, name := range pageNames(p) {
		for from := range g.linked[name] {
			if from != path {
				found[from] = true
			}
		}
	}
	return sortedKeys(found)
}

// Orphans returns the pages nothing links to, sorted. Index pages are left out:
// they are navigation, and being pointed at is not what they are for.
func (g *Graph) Orphans() []string {
	var out []string
	for _, path := range g.Paths() {
		if g.pages[path].Type() == TypeIndex {
			continue
		}
		if len(g.Backlinks(path)) == 0 {
			out = append(out, path)
		}
	}
	return out
}

// Findings reports everything the graph can see: links that do not resolve to
// exactly one page, names two pages both claim, and pages nothing links to.
//
// An ambiguous link is an error whatever the mode, because the tool cannot
// guess which page was meant and must not pick one. Every other finding here is
// a warning by default and an error under Strict.
func (g *Graph) Findings(mode Mode) []Finding {
	soft := Warning
	if mode == Strict {
		soft = Error
	}
	var out []Finding

	for _, name := range g.Names() {
		paths := g.Claimants(name)
		if len(paths) < 2 {
			continue
		}
		out = append(out, Finding{
			Severity: soft,
			Code:     CodeNameCollision,
			Path:     paths[0],
			Message:  quote(name) + " is claimed by " + listOf(paths),
		})
	}

	for _, path := range g.Paths() {
		for _, l := range g.links[path] {
			switch r := g.Resolve(l.Name); r.Kind {
			case Unresolved:
				out = append(out, Finding{
					Severity: soft,
					Code:     CodeUnresolvedLink,
					Path:     path,
					Line:     l.Line,
					Message:  unresolvedMessage(l),
				})
			case Ambiguous:
				out = append(out, Finding{
					Severity: Error,
					Code:     CodeAmbiguousLink,
					Path:     path,
					Line:     l.Line,
					Message:  quote(l.Target) + " matches " + listOf(r.Matches),
				})
			}
		}
	}

	for _, path := range g.Orphans() {
		out = append(out, Finding{
			Severity: soft,
			Code:     CodeOrphanPage,
			Path:     path,
			Message:  "no page links here",
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Code < out[j].Code
	})
	return out
}

func unresolvedMessage(l Link) string {
	if l.Target == "" {
		return "an empty wikilink names nothing"
	}
	return "no page is called " + quote(l.Target)
}

// pageNames returns the names a page answers to: its normalised title and its
// normalised aliases, with duplicates removed. An alias that normalises to the
// title is the same name, not a collision with itself.
func pageNames(p *Page) []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		n := Normalize(raw)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	add(p.Title())
	for _, a := range p.Aliases() {
		add(a)
	}
	sort.Strings(out)
	return out
}

func insertInto(m map[string]map[string]bool, key, value string) {
	set, ok := m[key]
	if !ok {
		set = map[string]bool{}
		m[key] = set
	}
	set[value] = true
}

func removeFrom(m map[string]map[string]bool, key, value string) {
	set, ok := m[key]
	if !ok {
		return
	}
	delete(set, value)
	if len(set) == 0 {
		delete(m, key)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func listOf(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = quote(s)
	}
	switch len(quoted) {
	case 0:
		return "nothing"
	case 1:
		return quoted[0]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " and " + quoted[len(quoted)-1]
}

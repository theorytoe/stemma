package render

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"math"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// GraphNode is one page in a page's local graph.
type GraphNode struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	// Backlinks is how many pages point here, which the hover tip shows.
	Backlinks int `json:"backlinks"`
	// Degree is backlinks plus outgoing links, which sizes the node in the
	// whole-KB graph. It is left unset in the local graph, where the nodes are
	// drawn a uniform size.
	Degree int `json:"degree,omitempty"`
	// Color is which accent tints the node in the whole-KB graph, chosen in Go
	// so the static SVG and the script agree and the tint does not change from
	// one visit to the next.
	Color int `json:"color,omitempty"`
}

// GraphData is the one-hop graph around a page: the page, the pages that link
// to it, and the pages it links to.
//
// It is one value used twice. The SVG draws it, and the same value is marshalled
// into the page as JSON for the script to read, so a script that redraws the
// graph cannot show a different graph from the one the baseline drew.
type GraphData struct {
	Center        GraphNode   `json:"center"`
	Backlinks     []GraphNode `json:"backlinks"`
	Links         []GraphNode `json:"links"`
	MoreBacklinks int         `json:"more_backlinks,omitempty"`
	MoreLinks     int         `json:"more_links,omitempty"`
}

const (
	// maxGraphSide caps the neighbours drawn on each side, so a page with a
	// hundred backlinks still gets a small graph. The rest are counted.
	maxGraphSide = 8

	// graphColors is how many accents the whole-KB graph tints its nodes from.
	// It must match the .graph-color-N rules in the stylesheet.
	graphColors = 7

	graphWidth    = 640
	graphNodeW    = 172
	graphNodeH    = 28
	graphRowGap   = 12
	graphPad      = 12
	graphMinH     = 120
	graphLabelMax = 22
)

// localGraph builds the one-hop graph for an authored page.
func (r *Renderer) localGraph(pagePath, docURL string) *GraphData {
	self, ok := r.kb.Graph.Page(pagePath)
	if !ok {
		return nil
	}

	var links []string
	for _, l := range r.kb.Graph.Links(pagePath) {
		if res := r.kb.Graph.Resolve(l.Name); res.Kind == kb.Resolved {
			links = append(links, res.Path)
		}
	}
	return r.graphFrom(pagePath, self.Title(), docURL, r.backCounts[pagePath], r.kb.Graph.Backlinks(pagePath), links)
}

// sourceGraph is the graph for a virtual source page. A source page is not in
// the link graph, so its one connection is the other way round: the pages that
// cite it, which is also what sizes it.
func (r *Renderer) sourceGraph(key, title, docURL string) *GraphData {
	cited := r.kb.Graph.CitedBy(key)
	return r.graphFrom("", title, docURL, len(cited), cited, nil)
}

// graphFrom assembles a graph from the centre's neighbours. self seeds the
// seen set so a page is never drawn twice, and a page that is both a backlink
// and an outgoing link is drawn once, on the backlink side. An archived
// neighbour is left out, so a local graph agrees with the whole-KB one.
func (r *Renderer) graphFrom(self, title, docURL string, centerBacklinks int, backPaths, linkPaths []string) *GraphData {
	data := &GraphData{Center: GraphNode{Title: title, URL: docURL, Backlinks: centerBacklinks}}
	seen := map[string]bool{self: true}

	add := func(path string, dst *[]GraphNode, more *int) {
		if seen[path] || r.archived[path] {
			return
		}
		seen[path] = true
		page, ok := r.kb.Graph.Page(path)
		if !ok {
			return
		}
		if len(*dst) == maxGraphSide {
			*more++
			return
		}
		*dst = append(*dst, GraphNode{
			Title:     page.Title(),
			URL:       rel(docURL, PageURL(path)),
			Backlinks: r.backCounts[path],
		})
	}

	for _, p := range backPaths {
		add(p, &data.Backlinks, &data.MoreBacklinks)
	}
	for _, p := range linkPaths {
		add(p, &data.Links, &data.MoreLinks)
	}
	return data
}

// graphSVG draws the graph: the page in the middle, its backlinks down the left
// and the pages it links to down the right. Nodes are real links, so the graph
// is navigable without a script; the shape is a star, which is what one hop is.
func graphSVG(d *GraphData) template.HTML {
	backRows := len(d.Backlinks)
	if d.MoreBacklinks > 0 {
		backRows++
	}
	linkRows := len(d.Links)
	if d.MoreLinks > 0 {
		linkRows++
	}
	rows := backRows
	if linkRows > rows {
		rows = linkRows
	}
	if rows == 0 {
		rows = 1
	}

	height := rows*(graphNodeH+graphRowGap) - graphRowGap + 2*graphPad
	if height < graphMinH {
		height = graphMinH
	}
	cx := graphWidth / 2
	cy := height / 2
	rightX := graphWidth - graphPad - graphNodeW

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="graph-svg" viewBox="0 0 %d %d" role="group" aria-label="Local graph">`, graphWidth, height)
	b.WriteString("<title>Local graph</title>")

	// Edges first, so the nodes draw over them.
	for i := range d.Backlinks {
		y := graphRowY(i, rows, height)
		fmt.Fprintf(&b, `<line class="graph-edge" x1="%d" y1="%d" x2="%d" y2="%d"/>`,
			cx-graphNodeW/2, cy, graphPad+graphNodeW, y+graphNodeH/2)
	}
	for i := range d.Links {
		y := graphRowY(i, rows, height)
		fmt.Fprintf(&b, `<line class="graph-edge" x1="%d" y1="%d" x2="%d" y2="%d"/>`,
			cx+graphNodeW/2, cy, rightX, y+graphNodeH/2)
	}

	for i, n := range d.Backlinks {
		b.WriteString(graphNodeMarkup(n, graphPad, graphRowY(i, rows, height), false))
	}
	for i, n := range d.Links {
		b.WriteString(graphNodeMarkup(n, rightX, graphRowY(i, rows, height), false))
	}
	b.WriteString(graphNodeMarkup(d.Center, cx-graphNodeW/2, cy-graphNodeH/2, true))

	if d.MoreBacklinks > 0 {
		y := graphRowY(len(d.Backlinks), rows, height)
		fmt.Fprintf(&b, `<text class="graph-more" x="%d" y="%d">+%d more</text>`,
			graphPad+9, y+graphNodeH/2+4, d.MoreBacklinks)
	}
	if d.MoreLinks > 0 {
		y := graphRowY(len(d.Links), rows, height)
		fmt.Fprintf(&b, `<text class="graph-more" x="%d" y="%d">+%d more</text>`,
			rightX+9, y+graphNodeH/2+4, d.MoreLinks)
	}

	b.WriteString("</svg>")
	return template.HTML(b.String())
}

// graphRowY is the top of row i when rows rows are centred in height.
func graphRowY(i, rows, height int) int {
	span := rows*(graphNodeH+graphRowGap) - graphRowGap
	start := (height - span) / 2
	return start + i*(graphNodeH+graphRowGap)
}

func graphNodeMarkup(n GraphNode, x, y int, self bool) string {
	tag := "a"
	if self {
		tag = "g"
	}
	class := "graph-node"
	if self {
		class += " graph-self"
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<%s class="%s"`, tag, class)
	if !self {
		fmt.Fprintf(&b, ` href="%s"`, html.EscapeString(n.URL))
	}
	b.WriteString(">")
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="5"/>`, x, y, graphNodeW, graphNodeH)
	fmt.Fprintf(&b, `<text x="%d" y="%d">%s</text>`, x+9, y+graphNodeH/2+4, html.EscapeString(graphLabel(n.Title)))
	fmt.Fprintf(&b, "</%s>", tag)
	return b.String()
}

// graphLabel is a node's title cut to fit the node. Titles are prose and can be
// long; the graph is a map, not the place to read a whole title.
func graphLabel(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= graphLabelMax {
		return s
	}
	return strings.TrimSpace(string(r[:graphLabelMax-3])) + "..."
}

// graphJSON is the graph as the inert JSON the page embeds for the script.
func graphJSON(d *GraphData) template.JS {
	b, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

// GraphJSON is a whole graph as the script reads it: the nodes, and the edges
// between them as index pairs. The page's local graph and the whole-KB graph
// both reach the script this way, so one script draws both.
type GraphJSON struct {
	Nodes     []GraphNode `json:"nodes"`
	Edges     [][2]int    `json:"edges"`
	MaxDegree int         `json:"max_degree"`
}

// allGraph builds the whole-KB graph: every page that is not archived, and
// every resolved wikilink between two of them. A link written twice is one edge,
// and a page linking to itself is no edge at all. An unresolved link has no
// node to point at, so it is left to lint rather than drawn as a line to
// nowhere. A link to or from an archived page is dropped with the page, because
// an edge to a node the graph does not draw is a line to nowhere too.
func (r *Renderer) allGraph(docURL string) GraphJSON {
	paths := r.kb.Graph.Paths()
	at := make(map[string]int, len(paths))
	nodes := make([]GraphNode, 0, len(paths))
	for _, p := range paths {
		page, ok := r.kb.Graph.Page(p)
		if !ok || r.archived[p] {
			continue
		}
		at[p] = len(nodes)
		nodes = append(nodes, GraphNode{
			Title:     page.Title(),
			URL:       rel(docURL, PageURL(p)),
			Backlinks: r.backCounts[p],
			Degree:    r.backCounts[p] + r.outCounts[p],
			Color:     colorIndex(p),
		})
	}

	var edges [][2]int
	seen := map[[2]int]bool{}
	for _, p := range paths {
		from, ok := at[p]
		if !ok {
			continue
		}
		for _, l := range r.kb.Graph.Links(p) {
			res := r.kb.Graph.Resolve(l.Name)
			if res.Kind != kb.Resolved {
				continue
			}
			to, ok := at[res.Path]
			if !ok || from == to {
				continue
			}
			a, b := from, to
			if a > b {
				a, b = b, a
			}
			if seen[[2]int{a, b}] {
				continue
			}
			seen[[2]int{a, b}] = true
			edges = append(edges, [2]int{a, b})
		}
	}
	return GraphJSON{Nodes: nodes, Edges: edges, MaxDegree: r.topDegree}
}

// archivedPaths is the set of retired pages. The graph leaves them out: an
// archived page is off the map, so neither it nor a link to it is drawn.
func archivedPaths(k *kb.KB) map[string]bool {
	out := map[string]bool{}
	for _, p := range k.Graph.Paths() {
		if page, ok := k.Graph.Page(p); ok && page.Status() == kb.StatusArchived {
			out[p] = true
		}
	}
	return out
}

// nodeCounts is the metric the whole-KB graph sizes its nodes by. A page's
// degree is how many pages point at it plus how many distinct pages it points
// at -- how connected it is, either way. It is computed once, when the renderer
// is built, so rendering every page does not recompute every page's count.
//
// Outgoing links are counted after resolution and deduplication, the way the
// edges are drawn, so a link written twice or a link to nowhere does not make a
// node look busier than the graph shows. Archived pages are not counted on
// either end, because the edges to them are not drawn either.
func nodeCounts(k *kb.KB, archived map[string]bool) (back, out map[string]int, topDegree int) {
	back = make(map[string]int, k.Graph.Len())
	out = make(map[string]int, k.Graph.Len())
	for _, p := range k.Graph.Paths() {
		in := 0
		for _, b := range k.Graph.Backlinks(p) {
			if !archived[b] {
				in++
			}
		}
		back[p] = in

		seen := map[string]bool{}
		for _, l := range k.Graph.Links(p) {
			res := k.Graph.Resolve(l.Name)
			if res.Kind != kb.Resolved || res.Path == p || archived[res.Path] || seen[res.Path] {
				continue
			}
			seen[res.Path] = true
		}
		out[p] = len(seen)

		if !archived[p] {
			if d := in + len(seen); d > topDegree {
				topDegree = d
			}
		}
	}
	return back, out, topDegree
}

// allGraphSVG draws the whole-KB graph as a circle: every page on a ring, every
// link a chord, its title alongside. It is not a readable picture of a large KB
// -- the drawn-out version is the script's -- but it is the same graph, it is
// cheap whatever the size, and its nodes are real links, so the page navigates
// without a script.
func allGraphSVG(g GraphJSON) template.HTML {
	const (
		size     = 720
		radius   = 5
		ring     = 268
		outward  = 1.06
		baseline = 4
	)
	cx, cy := float64(size)/2, float64(size)/2
	n := len(g.Nodes)
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i := range g.Nodes {
		angle := float64(i)/float64(maxInt(n, 1))*2*math.Pi - math.Pi/2
		xs[i] = cx + math.Cos(angle)*ring
		ys[i] = cy + math.Sin(angle)*ring
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="graph-svg" viewBox="0 0 %d %d" role="group" aria-label="Graph">`, size, size)
	b.WriteString("<title>Graph</title>")
	for _, e := range g.Edges {
		fmt.Fprintf(&b, `<line class="graph-edge" x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f"/>`,
			xs[e[0]], ys[e[0]], xs[e[1]], ys[e[1]])
	}
	for i, node := range g.Nodes {
		anchor := "start"
		if xs[i] < cx {
			anchor = "end"
		}
		lx := cx + (xs[i]-cx)*outward
		ly := cy + (ys[i]-cy)*outward
		fmt.Fprintf(&b, `<a class="graph-node graph-color-%d" href="%s"><title>%s</title>`,
			node.Color, html.EscapeString(node.URL), html.EscapeString(node.Title))
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="%d"/>`, xs[i], ys[i], radius)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="%s">%s</text>`, lx, ly+baseline, anchor, html.EscapeString(graphLabel(node.Title)))
		b.WriteString("</a>")
	}
	b.WriteString("</svg>")
	return template.HTML(b.String())
}

// allGraphJSON is the whole-KB graph as the inert JSON the page embeds.
func allGraphJSON(g GraphJSON) template.JS {
	b, err := json.Marshal(g)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// colorIndex picks a node's accent from its path. It is a hash rather than a
// counter so neighbours do not cycle through the palette in a visible order,
// and it is a hash of the path rather than a random number so a node keeps its
// colour between renders.
func colorIndex(path string) int {
	var h uint32 = 2166136261
	for i := 0; i < len(path); i++ {
		h ^= uint32(path[i])
		h *= 16777619
	}
	return int(h % graphColors)
}

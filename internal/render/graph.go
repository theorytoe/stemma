package render

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// GraphNode is one page in a page's local graph.
type GraphNode struct {
	Title string `json:"title"`
	URL   string `json:"url"`
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
	return r.graphFrom(pagePath, self.Title(), docURL, r.kb.Graph.Backlinks(pagePath), links)
}

// sourceGraph is the graph for a virtual source page. A source page is not in
// the link graph, so its one connection is the other way round: the pages that
// cite it.
func (r *Renderer) sourceGraph(key, title, docURL string) *GraphData {
	return r.graphFrom("", title, docURL, r.kb.Graph.CitedBy(key), nil)
}

// graphFrom assembles a graph from the centre's neighbours. self seeds the
// seen set so a page is never drawn twice, and a page that is both a backlink
// and an outgoing link is drawn once, on the backlink side.
func (r *Renderer) graphFrom(self, title, docURL string, backPaths, linkPaths []string) *GraphData {
	data := &GraphData{Center: GraphNode{Title: title, URL: docURL}}
	seen := map[string]bool{self: true}

	add := func(path string, dst *[]GraphNode, more *int) {
		if seen[path] {
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
		*dst = append(*dst, GraphNode{Title: page.Title(), URL: rel(docURL, PageURL(path))})
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

package index

import (
	"fmt"
	"sort"
	"strings"
)

// Direction is which way a graph walk goes.
type Direction int

const (
	// Out follows a page's own links.
	Out Direction = iota
	// In follows the links that point at a page.
	In
)

// Neighbor is one page reached by a walk, with the hop it was reached at.
type Neighbor struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	Depth int    `json:"depth"`
}

// ResolvePage turns a page name or path into the path of the one page it names.
//
// A path is taken as-is when it is in the corpus, which is what makes it
// possible to name one page when two claim the same title. A name that matches
// nothing, or more than one page, is an error: the caller asked for one page and
// cannot be given a guess.
func ResolvePage(src Source, name string) (string, error) {
	pages, err := src.Pages()
	if err != nil {
		return "", err
	}
	for _, m := range pages {
		if m.Path == name {
			return name, nil
		}
	}
	claims, err := src.Claimants(name)
	if err != nil {
		return "", err
	}
	switch len(claims) {
	case 0:
		return "", fmt.Errorf("no page is called %q", name)
	case 1:
		return claims[0], nil
	default:
		return "", fmt.Errorf("%q could be %s", name, strings.Join(claims, " or "))
	}
}

// Neighbors walks the graph from a page, following the given directions for at
// most depth hops. A depth of zero or less means the whole reachable set, which
// terminates because a page is visited once.
//
// A page is reported once, at the first hop that reached it, so a corpus where
// two paths reach the same page does not show it twice. Forward links that do
// not resolve to exactly one page are not edges and are dropped here; lint is
// where they are reported.
func Neighbors(src Source, start string, dirs []Direction, depth int) ([]Neighbor, error) {
	pages, err := src.Pages()
	if err != nil {
		return nil, err
	}
	title := make(map[string]string, len(pages))
	for _, m := range pages {
		title[m.Path] = m.Title
	}

	seen := map[string]bool{start: true}
	frontier := []string{start}
	var out []Neighbor
	for d := 1; len(frontier) > 0; d++ {
		if depth > 0 && d > depth {
			break
		}
		var next []string
		for _, path := range frontier {
			reached, err := neighborsOf(src, path, dirs)
			if err != nil {
				return nil, err
			}
			for _, p := range reached {
				if seen[p] {
					continue
				}
				seen[p] = true
				out = append(out, Neighbor{Path: p, Title: title[p], Depth: d})
				next = append(next, p)
			}
		}
		frontier = next
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Depth != out[j].Depth {
			return out[i].Depth < out[j].Depth
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// neighborsOf returns the pages one hop from path in the wanted directions.
func neighborsOf(src Source, path string, dirs []Direction) ([]string, error) {
	var out []string
	for _, dir := range dirs {
		switch dir {
		case Out:
			links, err := src.Links(path)
			if err != nil {
				return nil, err
			}
			for _, l := range links {
				claims, err := src.Claimants(l.Name)
				if err != nil {
					return nil, err
				}
				if len(claims) == 1 {
					out = append(out, claims[0])
				}
			}
		case In:
			backs, err := src.Backlinks(path)
			if err != nil {
				return nil, err
			}
			out = append(out, backs...)
		}
	}
	return out, nil
}

package source

import (
	"context"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// resolvearXiv asks arXiv's own BibTeX export for the record.
//
// arXiv serves a ready-made BibTeX entry for every paper, so the record is
// arXiv's rather than a reconstruction from an Atom feed. The key it hands out
// is kept as written: it is the key arXiv would give the paper, and rewriting
// it here would only make the tool's copy differ from everyone else's.
func (r *Resolver) resolvearXiv(ctx context.Context, id string) (*Result, error) {
	aid := kb.NormalizeArXiv(id)
	if !looksLikearXiv(aid) {
		return nil, invalid(id, "not an arXiv identifier")
	}
	url := strings.TrimRight(r.Endpoints.ArXiv, "/") + "/bibtex/" + aid
	return r.fetchBibTeX(ctx, id, KindarXiv, "arXiv", url)
}

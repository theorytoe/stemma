package source

import (
	"context"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// resolveDOI asks the DOI resolver for the record, using content negotiation.
//
// The DOI Foundation serves BibTeX through doi.org itself, so one request
// reaches Crossref, DataCite and mEDRA without the tool having to know which
// agency registered a DOI. That is why there is no agency-specific path here.
func (r *Resolver) resolveDOI(ctx context.Context, id string) (*Result, error) {
	doi := kb.NormalizeDOI(id)
	if !looksLikeDOI(doi) {
		return nil, invalid(id, "not a DOI")
	}
	url := strings.TrimRight(r.Endpoints.DOI, "/") + "/" + doi
	return r.fetchBibTeX(ctx, id, KindDOI, "the DOI resolver", url)
}

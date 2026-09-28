package source

import (
	"context"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// resolveURL builds a record from what a page says about itself.
//
// A URL has no authoritative resolver, so the page is the authority. Its head
// carries the citation metadata four vocabularies agree to write there, and a
// DOI among them is trusted over everything else: when a landing page names the
// work's DOI, the DOI resolver's record is the record, and the page is only how
// the DOI was found. A page that says nothing still yields a pointer, which is
// the least a URL can honestly be.
func (r *Resolver) resolveURL(ctx context.Context, id string) (*Result, error) {
	resp, err := r.Client.Get(ctx, id, "text/html,application/xhtml+xml")
	if err != nil {
		return nil, network(id, err)
	}
	if err := statusError(id, resp, "the server"); err != nil {
		resp.Body.Close()
		return nil, err
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, network(id, err)
	}
	final := id
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}

	meta := parseHTMLMeta(body)
	if meta.doi != "" {
		result, err := r.resolveDOI(ctx, meta.doi)
		if err == nil {
			return result, nil
		}
		// A DOI the page names but the resolver cannot produce a record for is the
		// page's mistake; the page's own metadata is still worth keeping. A
		// network failure is not the page's mistake and is not hidden.
		if k, _ := KindOf(err); k == ErrNetwork || k == ErrOffline {
			return nil, err
		}
	}

	typ := "misc"
	switch {
	case meta.isbn != "" && meta.journal == "":
		typ = "book"
	case meta.journal != "":
		typ = "article"
	}

	e := kb.NewBibEntry(typ, CiteKey(meta.authors, meta.date, meta.title, final))
	setIf(e, "title", meta.title)
	if len(meta.authors) > 0 {
		e.Set("author", bibValue(strings.Join(meta.authors, " and ")))
	}
	setIf(e, "journal", meta.journal)
	setIf(e, "year", kb.YearOf(meta.date))
	setIf(e, "publisher", meta.publisher)
	setIf(e, "isbn", meta.isbn)
	setIf(e, "url", final)
	now := r.now()
	stampAt(e, now)
	return &Result{Entry: e, Kind: KindURL, URL: final, Record: body, Retrieved: now}, nil
}

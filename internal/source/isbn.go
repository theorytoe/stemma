package source

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// Open Library's edition record. Only the fields a citation needs are read;
// the record carries far more, and none of the rest is this package's business.
type olEdition struct {
	Title         string   `json:"title"`
	Publishers    []string `json:"publishers"`
	PublishDate   string   `json:"publish_date"`
	NumberOfPages int      `json:"number_of_pages"`
	Contributions []string `json:"contributions"`
	ByStatement   string   `json:"by_statement"`
	Works         []olRef  `json:"works"`
	Key           string   `json:"key"`
}

type olRef struct {
	Key string `json:"key"`
}

type olWork struct {
	Authors []olRef `json:"authors"`
}

type olAuthor struct {
	Name string `json:"name"`
}

// resolveISBN reads an edition from Open Library and turns it into a book.
//
// Author names are the reason this makes a second request: an edition names its
// author by reference, and the name itself lives on the work. When the work or
// an author cannot be read, by_statement stands in, so a partial record is
// still a record rather than a failure.
func (r *Resolver) resolveISBN(ctx context.Context, id string) (*Result, error) {
	isbn := kb.NormalizeISBN(id)
	if !validISBN(isbn) {
		return nil, invalid(id, "not a valid ISBN")
	}
	base := strings.TrimRight(r.Endpoints.OpenLibrary, "/")

	resp, err := r.Client.Get(ctx, base+"/isbn/"+isbn+".json", "application/json")
	if err != nil {
		return nil, network(id, err)
	}
	if err := statusError(id, resp, "Open Library"); err != nil {
		resp.Body.Close()
		return nil, err
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, network(id, err)
	}

	var ed olEdition
	if err := json.Unmarshal(body, &ed); err != nil {
		return nil, network(id, fmt.Errorf("Open Library returned something that is not a record: %w", err))
	}
	if strings.TrimSpace(ed.Title) == "" {
		return nil, notFound(id, "Open Library has no usable record for it")
	}

	authors := append([]string(nil), ed.Contributions...)
	if len(authors) == 0 && len(ed.Works) > 0 {
		authors = r.openLibraryAuthors(ctx, base, ed.Works[0].Key)
	}
	if len(authors) == 0 && ed.ByStatement != "" {
		authors = []string{ed.ByStatement}
	}

	publisher := ""
	if len(ed.Publishers) > 0 {
		publisher = ed.Publishers[0]
	}
	year := kb.YearOf(ed.PublishDate)

	e := kb.NewBibEntry("book", CiteKey(authors, year, ed.Title, isbn))
	setIf(e, "title", ed.Title)
	if len(authors) > 0 {
		e.Set("author", bibValue(strings.Join(authors, " and ")))
	}
	setIf(e, "publisher", publisher)
	setIf(e, "year", year)
	setIf(e, "isbn", isbn)
	e.Set("url", bibValue(base+"/isbn/"+isbn))
	now := r.now()
	stampAt(e, now)
	return &Result{Entry: e, Kind: KindISBN, URL: base + "/isbn/" + isbn, Record: body, Retrieved: now}, nil
}

// openLibraryAuthors follows a work to its author names. It is best effort: a
// name that cannot be read is left out rather than failing the whole record.
func (r *Resolver) openLibraryAuthors(ctx context.Context, base, workKey string) []string {
	raw, err := r.getJSON(ctx, base+workKey+".json")
	if err != nil {
		return nil
	}
	var w olWork
	if json.Unmarshal(raw, &w) != nil {
		return nil
	}
	var out []string
	for i, ref := range w.Authors {
		if i >= 10 {
			break
		}
		raw, err := r.getJSON(ctx, base+ref.Key+".json")
		if err != nil {
			continue
		}
		var a olAuthor
		if json.Unmarshal(raw, &a) != nil {
			continue
		}
		if strings.TrimSpace(a.Name) != "" {
			out = append(out, a.Name)
		}
	}
	return out
}

// getJSON reads a small JSON document, used for the follow-up requests an ISBN
// resolution makes.
func (r *Resolver) getJSON(ctx context.Context, url string) ([]byte, error) {
	resp, err := r.Client.Get(ctx, url, "application/json")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return readBody(resp)
}

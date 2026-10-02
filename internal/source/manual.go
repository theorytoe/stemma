package source

import (
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// Fields is the metadata a person supplies when a record has to be entered by
// hand. Every field is optional; a title or an identifier is enough to build an
// entry from.
type Fields struct {
	// ID is the identifier the record came from, kept for the key fallback.
	ID string
	// Type overrides the entry type. When empty it is inferred from the rest.
	Type string
	// Key overrides the generated citation key.
	Key string

	Title   string
	Authors []string
	Year    string

	// Container is the journal for an article and the book title for a chapter
	// or a paper in a proceedings.
	Container string
	Publisher string
	Volume    string
	Issue     string
	Pages     string
	Edition   string

	DOI   string
	ArXiv string
	ISBN  string
	URL   string
	// Path is a file on this machine, kept exactly as given. It is the local
	// alternative to URL and the pointer a local capture is read from.
	Path string
}

// Manual builds an entry from supplied metadata, which is the fallback when the
// network is unavailable or unwanted.
//
// A bibliography that cannot be edited without a network is not usable, so this
// path exists and is not second-class: it produces the same kind of entry
// resolution does, and the only thing it does not add is a retrieval date,
// because nothing was retrieved. That absence is honest and is what makes a
// hand-entered record visibly different from a fetched one.
func Manual(f Fields) (*kb.BibEntry, error) {
	title := strings.TrimSpace(f.Title)
	if title == "" && firstNonEmpty(f.DOI, f.ArXiv, f.ISBN, f.URL, f.Path, f.ID) == "" {
		return nil, invalid(f.ID, "a record needs a title or an identifier")
	}

	typ := strings.TrimSpace(f.Type)
	if typ == "" {
		switch {
		case f.ISBN != "" && strings.TrimSpace(f.Container) == "":
			typ = "book"
		case strings.TrimSpace(f.Container) != "":
			typ = "article"
		default:
			typ = "misc"
		}
	}

	key := strings.TrimSpace(f.Key)
	if key == "" {
		key = CiteKey(f.Authors, f.Year, title, firstNonEmpty(f.DOI, f.ArXiv, f.ISBN, f.URL, f.Path, f.ID))
	}

	e := kb.NewBibEntry(typ, key)
	ApplyFields(e, f)
	setIf(e, "doi", kb.NormalizeDOI(f.DOI))
	setIf(e, "isbn", kb.NormalizeISBN(f.ISBN))

	if f.ArXiv != "" {
		setIf(e, "eprint", kb.NormalizeArXiv(f.ArXiv))
		e.Set("archivePrefix", "{arXiv}")
	}
	setIf(e, "url", f.URL)
	setIf(e, "path", f.Path)
	return e, nil
}

// ApplyFields writes the metadata a person supplied onto an entry, overwriting
// what is there. It is the same mapping Manual uses, split out so that a record
// the resolver produced can take a person's correction without being rebuilt.
//
// Only the descriptive fields are written. An identifier is not metadata: it is
// what gets resolved, and a resolved record already carries its own.
func ApplyFields(e *kb.BibEntry, f Fields) {
	setIf(e, "title", strings.TrimSpace(f.Title))
	if len(f.Authors) > 0 {
		e.Set("author", bibValue(strings.Join(f.Authors, " and ")))
	}
	if strings.TrimSpace(f.Container) != "" {
		if e.Type() == "article" {
			setIf(e, "journal", f.Container)
		} else {
			setIf(e, "booktitle", f.Container)
		}
	}
	setIf(e, "year", kb.YearOf(f.Year))
	setIf(e, "publisher", f.Publisher)
	setIf(e, "volume", f.Volume)
	setIf(e, "number", f.Issue)
	setIf(e, "pages", f.Pages)
	setIf(e, "edition", f.Edition)
}

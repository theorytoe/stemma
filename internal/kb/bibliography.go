package kb

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// BibSources returns every bibliography file a KB keeps, in the order they are
// read.
//
// A KB may keep its bibliography in one file, in a directory of files, or — long
// enough to have grown from one into the other — in both at once. Every command
// has to see the same set, so this is the one place that decides what that set
// is.
func (k *KB) BibSources() ([]string, error) {
	var out []string
	if _, err := os.Stat(k.Path(BibliographyName)); err == nil {
		out = append(out, BibliographyName)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	entries, err := os.ReadDir(k.Path(BibliographyDir))
	switch {
	case err == nil:
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".bib") {
				continue
			}
			out = append(out, path.Join(BibliographyDir, e.Name()))
		}
	case !os.IsNotExist(err):
		return nil, err
	}
	return out, nil
}

// ReadBibliography reads one bibliography file into the preserving model.
func (k *KB) ReadBibliography(name string) (*BibFile, error) {
	raw, err := os.ReadFile(k.Path(name))
	if err != nil {
		return nil, err
	}
	return ParseBibFile(name, raw)
}

// WriteBibliography writes a bibliography file back, atomically, so a failure
// part way through cannot leave a bibliography half written.
func (k *KB) WriteBibliography(name string, f *BibFile) error {
	full := k.Path(name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return WriteFileAtomic(full, f.Bytes())
}

// BibliographyFor returns the file a new entry should be written to, read when
// it already exists and empty when it does not.
//
// The single-file form is the default and wins whenever it exists, even when a
// directory is present too: both forms together are a migration in progress,
// and a new entry joins the default file rather than the half-drained one. A KB
// with only the directory form gets one file per entry, named from the key; an
// entry that is already there keeps the file it is in.
func (k *KB) BibliographyFor(key string) (*BibFile, string, error) {
	if _, err := os.Stat(k.Path(BibliographyName)); err == nil {
		f, err := k.ReadBibliography(BibliographyName)
		return f, BibliographyName, err
	} else if !os.IsNotExist(err) {
		return nil, "", err
	}

	info, err := os.Stat(k.Path(BibliographyDir))
	switch {
	case err == nil && info.IsDir():
		return k.directoryFileFor(key)
	case err != nil && !os.IsNotExist(err):
		return nil, "", err
	}

	f, err := ParseBibFile(BibliographyName, nil)
	if err != nil {
		return nil, "", err
	}
	return f, BibliographyName, nil
}

// directoryFileFor finds the per-entry file a key belongs in.
//
// A file that already defines the key is reused, so an update lands where the
// entry lives. Otherwise the first free name is taken, which is what keeps two
// keys whose names normalise alike from sharing a file.
func (k *KB) directoryFileFor(key string) (*BibFile, string, error) {
	base := strings.TrimSuffix(fileNameForKey(key), ".bib")
	for i := 1; ; i++ {
		name := base + ".bib"
		if i > 1 {
			name = fmt.Sprintf("%s-%d.bib", base, i)
		}
		full := path.Join(BibliographyDir, name)

		f, err := k.ReadBibliography(full)
		switch {
		case err == nil:
			if f.Has(key) {
				return f, full, nil
			}
			continue
		case os.IsNotExist(err):
			f, err := ParseBibFile(full, nil)
			if err != nil {
				return nil, "", err
			}
			return f, full, nil
		default:
			return nil, "", err
		}
	}
}

// fileNameForKey turns a citation key into a filename for the directory form. A
// key may carry characters a filename should not, so it is normalised the same
// way a page title is.
func fileNameForKey(key string) string {
	if s := Normalize(key); s != "" {
		return s + ".bib"
	}
	return "entry.bib"
}

// SameWork reports whether two entries describe the same work.
//
// The key is checked first, because an entry is identified by it. The key is not
// enough on its own, though: the same paper arrives under a fresh key whenever
// it comes from a different source, since a resolver mints the key from what it
// knows. Identifiers catch that case exactly, and the title, year and first
// author catch the hand-entered record that carries no identifier at all.
func SameWork(a, b *BibEntry) bool {
	if a.Key() == b.Key() {
		return true
	}
	if sameIdentifier(a, b, "doi", NormalizeDOI) {
		return true
	}
	if sameIdentifier(a, b, "eprint", ArXivIdentity) {
		return true
	}
	if sameIdentifier(a, b, "isbn", NormalizeISBN) {
		return true
	}
	if sameIdentifier(a, b, "url", NormalizeURL) {
		return true
	}
	return sameFields(a, b)
}

// MergeFrom copies every field src carries into e, keeping e's key and every
// field e already had that src does not mention.
//
// Nothing is ever deleted, because the tool does not destroy what it does not
// understand: a field a person added by hand survives an update, and so does a
// field from an older record that a new one happens to omit.
func (e *BibEntry) MergeFrom(src *BibEntry) {
	for _, f := range src.Fields() {
		e.Set(f.Name, f.Raw)
	}
}

func sameIdentifier(a, b *BibEntry, field string, normalize func(string) string) bool {
	x, y := fieldValue(a, field), fieldValue(b, field)
	if x == "" || y == "" {
		return false
	}
	return normalize(x) == normalize(y)
}

func sameFields(a, b *BibEntry) bool {
	at, bt := Normalize(fieldValue(a, "title")), Normalize(fieldValue(b, "title"))
	if at == "" || at != bt {
		return false
	}
	ay, by := YearOf(fieldValue(a, "year")), YearOf(fieldValue(b, "year"))
	if ay == "" || ay != by {
		return false
	}
	aa, ba := firstSurname(a), firstSurname(b)
	return aa != "" && aa == ba
}

func fieldValue(e *BibEntry, name string) string {
	v, ok := e.Value(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// Title is the record's title, or the empty string when it has none.
func (e *BibEntry) Title() string { return fieldValue(e, "title") }

// Year is the four-digit year the record carries, whatever else its date field
// says.
func (e *BibEntry) Year() string { return YearOf(fieldValue(e, "year")) }

// Authors is the record's authors, in order.
func (e *BibEntry) Authors() []string { return splitNameList(fieldValue(e, "author")) }

// URL is the entry's `url` field, which is where a source's readable location
// lives: an address, or a path to a file on this machine. It is the field a
// capture is read from, so it is the one an entry needs before it can be vendored.
func (e *BibEntry) URL() string { return fieldValue(e, "url") }

// firstSurname is the family name of an entry's first author, normalised, which
// is the part of an author list two records for one work agree on.
func firstSurname(e *BibEntry) string {
	authors := e.Authors()
	if len(authors) == 0 {
		return ""
	}
	return Normalize(Surname(authors[0]))
}

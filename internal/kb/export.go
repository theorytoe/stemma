package kb

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// The formats the bibliography can leave in.
//
// BibTeX is the KB's own form and is what a reference manager reads; CSL-JSON
// is what a citation processor reads. There is no third form and no style
// engine here: a KB exports its evidence, and formatting a citation is the
// consumer's business (D59).
const (
	FormatBibTeX  = "bibtex"
	FormatCSLJSON = "csl-json"
)

// ExportFormats returns the export formats, in the order to list them.
func ExportFormats() []string { return []string{FormatBibTeX, FormatCSLJSON} }

// KnownExportFormat reports whether name is a format the tool can write.
func KnownExportFormat(name string) bool {
	for _, f := range ExportFormats() {
		if name == f {
			return true
		}
	}
	return false
}

// ExportBibTeX renders records as one self-contained BibTeX file.
//
// Every entry is re-rendered with its values resolved, so any @string macro it
// used in the file it came from is written out in full. That is what makes the
// result stand alone: a directory-form KB whose files each defined their own
// macros exports to a file that needs none of them, and a reader never has to
// be told where the entry came from. Entry types, keys, field names, values and
// the tool's own provenance fields all survive; what is left behind is the
// @string and @preamble scaffolding, which is not evidence.
//
// Records come out in key order, so two exports of the same bibliography are
// the same bytes.
func ExportBibTeX(entries []*BibEntry) []byte {
	var buf bytes.Buffer
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.Write(e.canonicalBytes())
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// canonicalBytes renders the entry so that it stands alone.
//
// The entry itself is not touched: the expansion happens on a copy, because the
// bytes on disk are the source of truth and an export must not edit the KB.
func (e *BibEntry) canonicalBytes() []byte {
	out := NewBibEntry(e.typ, e.key)
	for _, f := range e.fields {
		v, _ := e.Value(f.Name)
		out.Set(f.Name, "{"+v+"}")
	}
	return out.Bytes()
}

// ExportCSLJSON renders records as CSL-JSON, the item array a citation
// processor reads.
//
// A BibTeX field with a CSL name is mapped; a field with none is carried
// through under its own name, so the retrieval date and the content hash a
// source is held to do not vanish on the way out. A consumer that knows only
// CSL ignores the extra keys, and a consumer that wants the provenance can
// still read it.
func ExportCSLJSON(entries []*BibEntry) ([]byte, error) {
	items := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		items = append(items, cslItem(e))
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// cslFromBibTeX maps a BibTeX entry type to its CSL name. An entry type CSL has
// no name for becomes a plain document rather than being dropped, because an
// export loses no record.
var cslFromBibTeX = map[string]string{
	"article":       "article-journal",
	"book":          "book",
	"booklet":       "pamphlet",
	"conference":    "paper-conference",
	"dataset":       "dataset",
	"electronic":    "webpage",
	"inbook":        "chapter",
	"incollection":  "chapter",
	"inproceedings": "paper-conference",
	"manual":        "book",
	"mastersthesis": "thesis",
	"misc":          "document",
	"online":        "webpage",
	"patent":        "patent",
	"periodical":    "article-journal",
	"phdthesis":     "thesis",
	"proceedings":   "book",
	"report":        "report",
	"software":      "software",
	"standard":      "standard",
	"techreport":    "report",
	"thesis":        "thesis",
	"unpublished":   "manuscript",
	"www":           "webpage",
}

// cslFromField maps a BibTeX field to the CSL field it fills. A field named
// here does not also pass through under its own name.
var cslFromField = map[string]string{
	"title":        "title",
	"subtitle":     "subtitle",
	"abstract":     "abstract",
	"keywords":     "keyword",
	"note":         "note",
	"language":     "language",
	"booktitle":    "container-title",
	"journal":      "container-title",
	"journaltitle": "container-title",
	"series":       "collection-title",
	"volume":       "volume",
	"number":       "issue",
	"issue":        "issue",
	"pages":        "page",
	"page":         "page",
	"publisher":    "publisher",
	"address":      "publisher-place",
	"edition":      "edition",
	"chapter":      "chapter-number",
	"doi":          "DOI",
	"url":          "URL",
	"isbn":         "ISBN",
	"issn":         "ISSN",
	"pmid":         "PMID",
	"pmcid":        "PMCID",
	"type":         "genre",
}

// cslNameFields are the fields whose value is a name list rather than text.
var cslNameFields = map[string]bool{
	"author":     true,
	"editor":     true,
	"translator": true,
}

// publisherFallbacks supply publisher from the fields BibTeX uses instead of it
// when a record has no publisher of its own.
var publisherFallbacks = []string{"school", "institution", "organization"}

// cslItem builds one CSL-JSON record.
func cslItem(e *BibEntry) map[string]any {
	item := map[string]any{
		"id":   e.Key(),
		"type": cslType(e.Type()),
	}
	for _, f := range e.fields {
		name := strings.ToLower(f.Name)
		value, _ := e.Value(f.Name)
		if value == "" {
			continue
		}
		switch {
		case cslNameFields[name]:
			if names := cslNameList(value); len(names) > 0 {
				item[name] = names
			}
		case name == "year" || name == "month" || name == "day":
			// Read together as the issued date, below.
		case name == strings.ToLower(FieldRetrieved):
			if parts := dateParts(value); len(parts) > 0 {
				item["accessed"] = cslDate(parts)
			}
		default:
			if csl, ok := cslFromField[name]; ok {
				item[csl] = value
			} else {
				item[name] = value
			}
		}
	}
	if date := issuedDate(e); date != nil {
		item["issued"] = date
	}
	if _, ok := item["publisher"]; !ok {
		for _, alt := range publisherFallbacks {
			if v, ok := e.Value(alt); ok && v != "" {
				item["publisher"] = v
				break
			}
		}
	}
	return item
}

// cslType is the CSL name for a BibTeX entry type.
func cslType(typ string) string {
	if csl, ok := cslFromBibTeX[strings.ToLower(typ)]; ok {
		return csl
	}
	return "document"
}

// issuedDate reads year, month and day as CSL's issued date.
//
// The month is only read when there is a year, and the day only when there is a
// month, so a stray day never invents a date.
func issuedDate(e *BibEntry) map[string]any {
	year := e.Year()
	if year == "" {
		return nil
	}
	parts := []int{atoiSafe(year)}
	if m, ok := e.Value("month"); ok {
		if n := monthNumber(m); n > 0 {
			parts = append(parts, n)
		}
	}
	if len(parts) == 2 {
		if d, ok := e.Value("day"); ok {
			if n := atoiSafe(d); n > 0 {
				parts = append(parts, n)
			}
		}
	}
	return cslDate(parts)
}

// cslDate is a CSL date literal.
func cslDate(parts []int) map[string]any {
	return map[string]any{"date-parts": [][]int{parts}}
}

// monthNumber reads a BibTeX month, which may be a name, an abbreviation or a
// number.
func monthNumber(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	if n := atoiSafe(s); n >= 1 && n <= 12 {
		return n
	}
	return cslMonths[s]
}

var cslMonths = map[string]int{
	"jan": 1, "january": 1,
	"feb": 2, "february": 2,
	"mar": 3, "march": 3,
	"apr": 4, "april": 4,
	"may": 5,
	"jun": 6, "june": 6,
	"jul": 7, "july": 7,
	"aug": 8, "august": 8,
	"sep": 9, "sept": 9, "september": 9,
	"oct": 10, "october": 10,
	"nov": 11, "november": 11,
	"dec": 12, "december": 12,
}

// dateParts reads a date as CSL's date-parts.
//
// It accepts a year, a year and month, a full date, and the timestamp form a
// retrieval date is stored in, and it reads nothing else: a value it cannot
// make a year out of is dropped rather than guessed at.
func dateParts(s string) []int {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "T "); i >= 0 {
		s = s[:i]
	}
	s = strings.ReplaceAll(s, "/", "-")
	var parts []int
	for _, seg := range strings.Split(s, "-") {
		n := atoiSafe(seg)
		if n == 0 {
			break
		}
		parts = append(parts, n)
		if len(parts) == 3 {
			break
		}
	}
	if len(parts) == 0 || parts[0] < 1000 {
		return nil
	}
	return parts
}

// cslNameList reads a BibTeX name list into CSL names.
func cslNameList(value string) []map[string]any {
	var out []map[string]any
	for _, n := range SplitNameList(value) {
		out = append(out, cslName(n))
	}
	return out
}

// cslName is one name as CSL keeps it: a literal for a name that must not be
// split, and family, given and suffix for one that can be.
func cslName(n Name) map[string]any {
	if n.Literal != "" {
		return map[string]any{"literal": n.Literal}
	}
	out := map[string]any{"family": n.Family}
	if n.Given != "" {
		out["given"] = n.Given
	}
	if n.Suffix != "" {
		out["suffix"] = n.Suffix
	}
	return out
}

// atoiSafe reads a decimal number, and zero for anything else.
func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

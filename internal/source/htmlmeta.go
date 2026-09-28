package source

import (
	"bytes"
	"encoding/json"
	"html"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// htmlMeta is what a page says about itself: the citation metadata a landing
// page carries so that a reference manager can build a record from it.
type htmlMeta struct {
	title     string
	authors   []string
	date      string
	publisher string
	journal   string
	doi       string
	isbn      string
	arxiv     string
}

// parseHTMLMeta reads the citation metadata a page carries.
//
// The standard library has no HTML parser and the dependency list is fixed, so
// this finds the tags that carry metadata and ignores everything else. That is
// enough for the job: a head is a flat list of meta tags, and a tag that does
// not parse is skipped rather than allowed to stop the scan. Four vocabularies
// are read, most-citation-specific first: Highwire citation_*, Dublin Core,
// schema.org JSON-LD, and OpenGraph.
func parseHTMLMeta(body []byte) htmlMeta {
	meta := map[string][]string{}
	var title string
	var ld [][]byte

	eachTag(body, func(name string, attrs map[string]string, text []byte) {
		switch name {
		case "meta":
			key := strings.ToLower(firstNonEmpty(attrs["name"], attrs["property"]))
			if key == "" {
				return
			}
			meta[key] = append(meta[key], html.UnescapeString(attrs["content"]))
		case "title":
			title = html.UnescapeString(strings.TrimSpace(string(text)))
		case "script":
			if strings.EqualFold(attrs["type"], "application/ld+json") {
				ld = append(ld, text)
			}
		}
	})

	m := htmlMeta{
		title:     pick(meta, "citation_title", "dc.title", "og:title", "twitter:title"),
		authors:   append(values(meta, "citation_author"), values(meta, "dc.creator")...),
		date:      pick(meta, "citation_publication_date", "dc.date", "article:published_time"),
		publisher: pick(meta, "citation_publisher", "dc.publisher", "og:site_name"),
		journal:   pick(meta, "citation_journal_title", "citation_conference_title"),
		doi:       pick(meta, "citation_doi"),
		isbn:      pick(meta, "citation_isbn"),
		arxiv:     pick(meta, "citation_arxiv_id"),
	}
	if m.title == "" {
		m.title = title
	}
	if m.doi == "" {
		for _, v := range values(meta, "dc.identifier") {
			if looksLikeDOI(kb.NormalizeDOI(v)) {
				m.doi = kb.NormalizeDOI(v)
				break
			}
		}
	}
	mergeJSONLD(&m, ld)
	return m
}

// eachTag calls fn for every start tag that could carry metadata. A comment, a
// doctype and a closing tag are stepped over; the text of a <title> or a
// JSON-LD <script> is handed to fn along with the tag.
func eachTag(body []byte, fn func(name string, attrs map[string]string, text []byte)) {
	for i := 0; i < len(body); {
		lt := bytes.IndexByte(body[i:], '<')
		if lt < 0 {
			return
		}
		i += lt
		if bytes.HasPrefix(body[i:], []byte("<!--")) {
			end := bytes.Index(body[i:], []byte("-->"))
			if end < 0 {
				return
			}
			i += end + 3
			continue
		}
		gt := tagEnd(body, i)
		if gt < 0 {
			return
		}
		raw := body[i+1 : gt]
		i = gt + 1
		if len(raw) == 0 || raw[0] == '/' || raw[0] == '!' || raw[0] == '?' {
			continue
		}

		name, attrs := parseTag(raw)
		name = strings.ToLower(name)
		if name != "meta" && name != "title" && name != "script" {
			continue
		}
		var text []byte
		if name == "title" || (name == "script" && strings.EqualFold(attrs["type"], "application/ld+json")) {
			if ci := indexFold(body[i:], []byte("</"+name)); ci >= 0 {
				text = body[i : i+ci]
				if ce := tagEnd(body, i+ci); ce >= 0 {
					i = ce + 1
				}
			}
		}
		fn(name, attrs, text)
	}
}

// tagEnd returns the offset of the ">" that closes a tag, respecting quoted
// attribute values so that a ">" inside one is not mistaken for the end.
func tagEnd(body []byte, start int) int {
	quote := byte(0)
	for i := start + 1; i < len(body); i++ {
		c := body[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '>':
			return i
		}
	}
	return -1
}

// parseTag splits a tag into its name and its attributes. Attribute names are
// lowercased; values keep their case and quotes are removed.
func parseTag(raw []byte) (string, map[string]string) {
	i := 0
	for i < len(raw) && isTagNameByte(raw[i]) {
		i++
	}
	name := string(raw[:i])
	attrs := map[string]string{}
	for i < len(raw) {
		for i < len(raw) && isTagSpace(raw[i]) {
			i++
		}
		if i >= len(raw) {
			break
		}
		start := i
		for i < len(raw) && !isTagSpace(raw[i]) && raw[i] != '=' {
			i++
		}
		key := strings.ToLower(string(raw[start:i]))
		for i < len(raw) && isTagSpace(raw[i]) {
			i++
		}
		if key == "" {
			continue
		}
		if i >= len(raw) || raw[i] != '=' {
			attrs[key] = ""
			continue
		}
		i++ // the "="
		for i < len(raw) && isTagSpace(raw[i]) {
			i++
		}
		var val string
		if i < len(raw) && (raw[i] == '"' || raw[i] == '\'') {
			q := raw[i]
			i++
			start := i
			for i < len(raw) && raw[i] != q {
				i++
			}
			val = string(raw[start:i])
			if i < len(raw) {
				i++
			}
		} else {
			start := i
			for i < len(raw) && !isHTMLSpace(raw[i]) {
				i++
			}
			val = string(raw[start:i])
		}
		attrs[key] = val
	}
	return name, attrs
}

// mergeJSONLD fills whatever the meta tags did not already say. JSON-LD is
// consulted after them because citation_* is written for exactly this purpose,
// while JSON-LD is written for search engines and only happens to answer.
//
// A page's JSON-LD often describes the site as well as the work — a WebSite
// node and an Article node in one @graph — so the work is read first and
// everything else only fills what is still empty. Otherwise a site's name would
// win the title over the article's.
func mergeJSONLD(m *htmlMeta, blocks [][]byte) {
	var docs []any
	for _, b := range blocks {
		var v any
		if json.Unmarshal(b, &v) == nil {
			docs = append(docs, v)
		}
	}
	for _, work := range []bool{true, false} {
		for _, doc := range docs {
			walkJSONLD(m, doc, work)
		}
	}
}

func walkJSONLD(m *htmlMeta, v any, work bool) {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			walkJSONLD(m, e, work)
		}
	case map[string]any:
		if g, ok := t["@graph"]; ok {
			walkJSONLD(m, g, work)
		}
		if isWorkType(jsonType(t)) != work {
			return
		}
		if m.title == "" {
			m.title = firstNonEmpty(jsonString(t, "headline"), jsonString(t, "name"))
		}
		if len(m.authors) == 0 {
			m.authors = jsonAuthors(t["author"])
		}
		if m.date == "" {
			m.date = jsonString(t, "datePublished")
		}
		if m.publisher == "" {
			m.publisher = jsonPublisher(t["publisher"])
		}
		if m.isbn == "" {
			m.isbn = jsonString(t, "isbn")
		}
		if m.doi == "" {
			if d := kb.NormalizeDOI(jsonString(t, "doi")); looksLikeDOI(d) {
				m.doi = d
			}
		}
	}
}

func jsonString(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// jsonType is an object's @type, which may be written as a string or as a list.
func jsonType(m map[string]any) string {
	switch v := m["@type"].(type) {
	case string:
		return v
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok {
				return s
			}
		}
	}
	return ""
}

// isWorkType reports whether a schema.org type names a work rather than the
// site, the publisher or the person behind it.
func isWorkType(t string) bool {
	switch t {
	case "ScholarlyArticle", "Article", "NewsArticle", "BlogPosting", "TechArticle",
		"Report", "Thesis", "Book", "Chapter", "Dataset", "CreativeWork",
		"Review", "SoftwareSourceCode":
		return true
	}
	return false
}

func jsonAuthors(v any) []string {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{t}
	case map[string]any:
		if name := jsonString(t, "name"); name != "" {
			return []string{name}
		}
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, jsonAuthors(e)...)
		}
		return out
	}
	return nil
}

func jsonPublisher(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return jsonString(t, "name")
	case []any:
		if len(t) > 0 {
			return jsonPublisher(t[0])
		}
	}
	return ""
}

// pick returns the first non-empty value across keys, in the order the keys are
// given, so that a caller states its preference by the order it lists them.
func pick(meta map[string][]string, keys ...string) string {
	for _, key := range keys {
		for _, v := range meta[key] {
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func values(meta map[string][]string, key string) []string {
	var out []string
	for _, v := range meta[key] {
		if s := strings.TrimSpace(v); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// indexFold is a case-insensitive bytes.Index, for finding a closing tag whose
// case is not guaranteed.
func indexFold(haystack, needle []byte) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if bytes.EqualFold(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func isTagNameByte(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || isDigit(b) || b == '-' || b == ':' || b == '_'
}

func isTagSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '/'
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

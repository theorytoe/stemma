package source

import (
	"reflect"
	"testing"
)

func TestParseHTMLMetaCitationTags(t *testing.T) {
	body := []byte(`<!doctype html>
<html><head>
<meta name="citation_title" content="A Study of &amp; Things">
<meta name='citation_author' content='Doe, Jane'>
<meta name="citation_author" content="Roe, John">
<meta name="citation_publication_date" content="2019-04-01">
<meta name="citation_journal_title" content="Journal of Things">
<meta name="citation_publisher" content="Things Press">
<meta name="citation_doi" content="10.1/x">
<meta name="citation_isbn" content="9780262033848">
</head></html>`)

	m := parseHTMLMeta(body)
	if m.title != "A Study of & Things" {
		t.Errorf("title = %q", m.title)
	}
	if !reflect.DeepEqual(m.authors, []string{"Doe, Jane", "Roe, John"}) {
		t.Errorf("authors = %q", m.authors)
	}
	if m.date != "2019-04-01" || m.journal != "Journal of Things" {
		t.Errorf("date = %q, journal = %q", m.date, m.journal)
	}
	if m.publisher != "Things Press" || m.doi != "10.1/x" || m.isbn != "9780262033848" {
		t.Errorf("publisher = %q, doi = %q, isbn = %q", m.publisher, m.doi, m.isbn)
	}
}

func TestParseHTMLMetaPrecedence(t *testing.T) {
	body := []byte(`<html><head>
<title>The document title</title>
<meta property="og:title" content="The OpenGraph title">
<meta name="dc.title" content="The Dublin Core title">
<meta name="citation_title" content="The citation title">
</head></html>`)

	if got := parseHTMLMeta(body).title; got != "The citation title" {
		t.Errorf("title = %q, want the citation title to win", got)
	}

	// With no citation tag, Dublin Core beats OpenGraph, which beats <title>.
	body = []byte(`<html><head>
<title>The document title</title>
<meta property="og:title" content="The OpenGraph title">
<meta name="dc.title" content="The Dublin Core title">
</head></html>`)
	if got := parseHTMLMeta(body).title; got != "The Dublin Core title" {
		t.Errorf("title = %q, want the Dublin Core title", got)
	}
}

func TestParseHTMLMetaTitleFallback(t *testing.T) {
	body := []byte(`<html><head><title>A &lt;plain&gt; page</title></head></html>`)
	if got := parseHTMLMeta(body).title; got != "A <plain> page" {
		t.Errorf("title = %q", got)
	}
}

func TestParseHTMLMetaOpenGraphOnly(t *testing.T) {
	body := []byte(`<html><head>
<meta property="og:title" content="An OpenGraph page">
<meta property="og:site_name" content="Example">
</head></html>`)
	m := parseHTMLMeta(body)
	if m.title != "An OpenGraph page" {
		t.Errorf("title = %q", m.title)
	}
	if m.publisher != "Example" {
		t.Errorf("publisher = %q", m.publisher)
	}
}

func TestParseHTMLMetaDublinCoreDOI(t *testing.T) {
	body := []byte(`<html><head>
<meta name="DC.title" content="A paper">
<meta name="DC.creator" content="Doe, Jane">
<meta name="DC.date" content="2020">
<meta name="DC.identifier" content="https://doi.org/10.1145/y">
</head></html>`)
	m := parseHTMLMeta(body)
	if m.doi != "10.1145/y" {
		t.Errorf("doi = %q", m.doi)
	}
	if !reflect.DeepEqual(m.authors, []string{"Doe, Jane"}) {
		t.Errorf("authors = %q", m.authors)
	}
}

func TestParseHTMLMetaJSONLD(t *testing.T) {
	body := []byte(`<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"ScholarlyArticle",
 "headline":"A Structured Paper",
 "author":[{"@type":"Person","name":"Doe, Jane"},{"name":"Roe, John"}],
 "datePublished":"2021-01-02",
 "publisher":{"@type":"Organization","name":"Example Press"},
 "isbn":"9780262033848"}
</script>
</head></html>`)
	m := parseHTMLMeta(body)
	if m.title != "A Structured Paper" {
		t.Errorf("title = %q", m.title)
	}
	if !reflect.DeepEqual(m.authors, []string{"Doe, Jane", "Roe, John"}) {
		t.Errorf("authors = %q", m.authors)
	}
	if m.date != "2021-01-02" || m.publisher != "Example Press" || m.isbn != "9780262033848" {
		t.Errorf("date = %q, publisher = %q, isbn = %q", m.date, m.publisher, m.isbn)
	}
}

func TestParseHTMLMetaJSONLDGraph(t *testing.T) {
	body := []byte(`<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@graph":[
  {"@type":"WebSite","name":"The Site"},
  {"@type":"ScholarlyArticle","name":"The Article","datePublished":"2018"}
]}
</script>
</head></html>`)
	m := parseHTMLMeta(body)
	if m.title != "The Article" {
		t.Errorf("title = %q, want the article, not the site", m.title)
	}
}

func TestParseTagHandlesQuotedAngleBracket(t *testing.T) {
	raw := []byte(`meta name="citation_title" content="a > b"`)
	name, attrs := parseTag(raw)
	if name != "meta" {
		t.Fatalf("name = %q", name)
	}
	if attrs["content"] != "a > b" {
		t.Errorf("content = %q, want a > b", attrs["content"])
	}
}

func TestParseHTMLMetaIgnoresComments(t *testing.T) {
	body := []byte(`<html><head>
<meta name="citation_title" content="Kept">
<!-- <meta name="citation_author" content="Commented out"> -->
<meta name="citation_author" content="Doe, Jane">
</head></html>`)
	m := parseHTMLMeta(body)
	if m.title != "Kept" {
		t.Errorf("title = %q", m.title)
	}
	if !reflect.DeepEqual(m.authors, []string{"Doe, Jane"}) {
		t.Errorf("authors = %q, want only the uncommented one", m.authors)
	}
}

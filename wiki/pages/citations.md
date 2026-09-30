---
title: Citations
type: concept
aliases:
  - the bibliography
tags: [format]
---
A claim that rests on a source names it in brackets: `[@bush1945]`. What is
inside is a citation key, not a title, and it points into the bibliography of
the knowledge base (KB), which is BibTeX at the KB root. A key is an identifier,
so it is matched exactly — unlike a page name, case matters.

```markdown
That is the argument Bush made for the memex [@bush1945].
```

A citation is not a wikilink. A wikilink names a page; a citation names a source,
and a source is not part of the page namespace, so `[[bush1945]]` does not reach
one. See [[Wikilinks]] for the other construct.

The bibliography records a source by pointer — a DOI, an arXiv identifier, a
URL, an ISBN, or a local file — with the date it was retrieved and a hash of
what was retrieved. The full text is not stored unless vendoring is asked for,
and the pointer and the hash are what let drift be reported rather than
discovered later.

The site materialises a source page for every key a page cites; see
[[The rendered site]]. It is built from the entry and never committed, because
it is generated. A citation renders in the style the manifest names, and every
key in a group links to its own source page. A key the bibliography does not
define stays as text and is reported, the way an unresolved link is.

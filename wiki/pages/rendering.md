---
title: The rendered site
type: concept
aliases:
  - rendering
tags: [site]
---
A KB is plain text, but plain was never the same as readable. The rendered site
is the human interface, and the format did not have to bend to make it one: a
page becomes an HTML document, and the two constructs the format gives meaning
to become links rather than staying punctuation.

- A wikilink becomes a link to the page it names, and it shows that page's
  title rather than the spelling that was written. An alias therefore reads as
  the title too.
- A link that resolves to nothing, or to more than one page, is neither dropped
  nor turned into a link to nowhere. The written text stays, in a span a
  stylesheet can mark.
- A citation becomes the formatter's label, and each key the bibliography
  defines becomes a link to that key's source page. What the bibliography does
  not define stays text. See [[Citations]].

Around the body, a page carries its title; a line of what the frontmatter says,
in the order a reader would ask for it — type, an archived page's reason, tags,
aliases; the reference list of what it cites; and the pages that link to it. A
section with nothing in it is left out rather than shown empty.

Some of the site is generated rather than written. The Index lists every page,
and a Types page and a Tags page list the types and the tags with their counts;
each type and each tag then has an index of its own, and each cited key gets a
source page built from its BibTeX entry. A type or a tag with no pages under it
gets no index of its own, because an empty index is noise rather than
navigation. The site's front page is generated the same way — [[The home page]]
says what it shows.

A page at `pages/notes/one.md` is served at `notes/one.html`, and every address
is written relative to the document that carries it. That is what lets the
output be moved or opened from disk without a link breaking.

One stylesheet, no framework and no build step. Text reads in Work Sans, falling
back to Roboto and then the reader's own system face; the fonts are named rather
than shipped, so the site stays self-contained. Light and dark follow the system
preference, and a toggle adds an explicit choice that is remembered. The toggle
is JavaScript, and it is polish: with none, the preference decides and a reader
never sees a control that would do nothing. Read the site with
[[Serving the site]].

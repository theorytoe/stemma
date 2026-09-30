---
title: The home page
type: concept
aliases:
  - landing page
tags: [site]
---
Every knowledge base (KB) gets a home page the tool writes, at `index.html`, and
it is not really a page anyone authored. It says what the KB holds, and
everything it says is a consequence of the content rather than of anyone's
choice.

- The numbers: how many pages, sources, types and tags there are, each linking to
  the list it counts.
- Browse: the types and the tags with the most pages under them, busiest first,
  and a link from each heading to the whole list.
- Newest sources: the sources with the most recent retrieval date, and the pages
  that lean on them. Recency comes from here and nowhere else, because
  [[The format]] records no dates for pages at all — a source is read on a day and
  the entry keeps that day, while a page's file time means nothing once a KB has
  been cloned, when every file arrives with the same one.
- Needs attention: what lint would report. It is absent when there is nothing to
  report, which is what lets it sit on a published page.

The one part a person wrote is the opening. When a page exists at `pages/index.md`
it is the entry document, and its body is the home page's opening: where someone
says which page to read first, and why. That is the part a tool cannot supply, and
the page is optional — delete it and the home page is generated alone.

Nothing on the home page is dated, shuffled or random, so two builds of one KB are
the same bytes. [[The rendered site]] describes what else is generated, and
[[Invoking the tool]] the commands that write it.

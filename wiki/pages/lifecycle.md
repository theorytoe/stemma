---
title: Creating and maintaining a KB
type: concept
aliases:
  - the kb lifecycle
tags: [workflow]
---
A KB begins as a directory and then lives by editing. The tool creates the
shape, checks it and repairs the links; the prose is written by hand and stays
that way. What follows is the ordinary life of a KB, from creation to the loop
that keeps it healthy.

**Creating one.** `stemma init [DIRECTORY]` writes the whole root at once: the
manifest, an entry document at `pages/index.md`, a `bibliography.bib`, an
`inbox/`, and the extraction shim under `.stemma/`. The title defaults to the
directory's name and `--title` overrides it. It refuses to run over a directory
that already looks like a KB root — a manifest, a `pages/`, or a bibliography —
because a new KB should never be bought by replacing a page someone is using.
The entry document is the page every other page is reachable from, and it is the
site's home. See [[Structure]].

**Adding a page.** `stemma new "A Title"` writes `pages/<slug>.md`, carrying
nothing but a title and a type; the type comes from `--type` or the manifest's
`default_type`. The filename is the title's slug, so nobody has to choose one.
The command refuses a title that is already a name another page answers to,
because a second page with that name would make every link to it ambiguous,
which the format treats as a hard error.

**Adding a draft.** `stemma new --draft "A Title"` writes into `inbox/` instead.
A draft is not a page: it is not indexed, not linted, and a link to a name
nothing answers to is simply unresolved. `stemma promote <draft>` is the one
transition. It checks the draft against every page rule at once and refuses one
that cannot become a valid page rather than moving it and leaving it broken; then
it moves the file and reports how many existing links now resolve, because
promotion is by title and nothing was rewritten to make them. See [[Drafts]].

**Writing.** From here the work is yours: open the file and write. The tool never
writes prose. The two things that connect pages are written inline — `[[name]]`
for a page and `[@key]` for a source — and both are checked the next time the KB
is linted. Sources join the KB through `stemma cite add`, which resolves a DOI,
an arXiv identifier, a URL or an ISBN, or takes the fields by hand. See
[[Citations]].

**Keeping it healthy.** `stemma lint` reports everything wrong and `stemma
status` summarises the whole KB — pages by type, orphans, ambiguous names,
sources, drafts, and whether an index exists. `--strict` turns warnings into
errors, so the same checks are advisory while writing and a gate in CI.

The maintenance verbs are ordered so that structure can change without breaking
a name:

- `stemma move PAGE DIRECTORY` moves a page between directories. Directories
  carry no meaning, so no link changes; the filename does not change either.
- `stemma rename PAGE "New Title"` retitles and rewrites every inbound link that
  named the old title. Links that named it by an alias are left alone, because
  they still work. An interrupted rename can be run again.
- `stemma archive PAGE --reason "..."` is the closest thing to removal. The tool
  never deletes a page; it records why the page was retired and leaves the text
  in place.

Retrieval is optional and additive. `stemma index` builds the Tier-1 index,
`search` and `graph` read it when it is fresh and the KB itself when it is not,
and `serve` renders the site; see [[The rendered site]]. Nothing generated is
committed, and `.stemma/` can be deleted at any time. [[The command surface]]
lists the whole surface.

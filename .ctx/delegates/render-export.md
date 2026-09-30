---
title: Stemma — Render and export
date: 2026-09-26
---

This is a delegate plan. It covers Task 6 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Make the KB readable by a human, and make it possible to hand a coherent piece of
it to someone else. This delegate is where the knowledge base stops being files
and becomes something a person can use.

## The human interface

Tool-free readability of the raw files was explicitly dropped as a requirement
(`D13`). The rendered site is the human interface. That decision is what makes the
whole delegate necessary — links do not have to look right in a text editor
because they will look right here.

## Rendering

Markdown rendering uses `github.com/russross/blackfriday/v2`. This is a deliberate
choice even though gogs, gitea, and Hugo all migrated to goldmark: the project needs
a custom renderer to resolve `[[wikilinks]]` and `[@key]` citations into real links,
and blackfriday's `Renderer` interface is more direct for that than goldmark's
layered extension model. Do not re-litigate this.

Rendering must not be the only consumer of link resolution. Use the same resolver
as lint so a page that lints clean renders correctly, and so rendering cannot
disagree with validation.

## The no-JS requirement

Every web surface must work with JavaScript disabled (`P9`, `D40`). This is a
functional requirement, not a preference. Readability, navigation, per-type and
per-tag index pages, backlink lists, and search in server mode are all plain HTML.
JavaScript is permitted only for polish — incremental client-side search, filtering
— and is loaded as a plain static asset with no build step, no npm, and no
framework (`D9`). Task 5 is the gate that keeps this honest.

## One renderer, two entry points

`serve` is local with live reload; `build` writes a static site to an output
directory. Both use the same templates via `html/template` and `embed`, so there is
one implementation and two ways to invoke it (`D40`).

## Export

Two exports exist, and only two. Whole-KB sharing is `git clone` or `tar` and needs
no tooling at all (`D25`), which is why there is no bundling subsystem.

`export json` dumps pages, links, and citations in a machine-readable form — nearly
free once the parser exists, and the thing that makes a KB consumable by other
agent and RAG systems (`D33`).

`export page` performs scoped extraction: a root document plus the documents it
links to, configurable in depth, defaulting to one hop, with `--depth all` for
transitive closure (`D27`, `D29`). Citation entries are materialised into the
exported tree so references never dangle (`D31`). Links pointing outside the slice
are pruned, retaining their anchor text so prose still reads correctly (`D44`,
`D51`); a summary is reported by default and `--report-pruned` lists them
individually. The output is a KB root in the same format, so every existing tool
works on it unchanged (`D30`).

The acceptance criterion for Task 8 follows directly: **an extract must itself be a
valid KB root that lints clean under `--strict`.** If it does not, the export is
broken regardless of how it looks.

## Dependencies

Task 1 for parsing and resolution. Task 3 of the project for citation rendering and
virtual source pages. Task 5 of the project is optional here — the site must
build without an index.

## Working agreement

Two things about how this delegate is built, agreed with the author before Task 1
began.

Subtasks 2, 3 and 4 — templates and assets, `serve`, and `build` — are designed
interactively. The author reviews the look and feel as it develops, so work
bounces between the renderer, the templates, and the two entry points until it is
right. Changes are shown small and often rather than batched.

This delegate may need to change code authored by Tasks 1–5. Every such change is
recorded, as it happens, in `.ctx/design/render-export-changes.md`, and a change to
an interface another delegate depends on is raised as a decision rather than taken
quietly. New files owned only by this delegate are not logged there.

# Tasks

## Task 1:

Status: Done

Renderer core. Blackfriday with a custom renderer that resolves wikilinks and
citations through the shared resolver, reports unresolved links rather than
emitting them silently, and produces stable output for golden tests. Link
resolution and validation must agree by construction, not by convention.

It closed with `internal/render`: one `Renderer` that turns a page body into
HTML, resolving wikilinks through `Graph.Resolve` and citations through the
bibliography and `citestyle`, with `PageURL` and `SourceURL` fixing the site's
addresses. Rendering and lint read one scan — `kb.Inlines()`, added for this
task — because blackfriday has no inline hook and splits a link that contains
emphasis, so a tree walk would miss what lint counts. A page's findings come from
`kb.FindingsFor`, the same helpers lint reads, and `TestFindingsMatchLint` holds
the two together. `testdata/body.golden` fixes the output. The prior-code changes
and the new decision `D69`, which settles what a link and a citation become, are
recorded in `.ctx/design/render-export-changes.md` and the register.

One deviation from the plan as written: the resolution is not done inside a
blackfriday `Renderer`. Blackfriday exposes no inline-extension point, so the
shared scan expands the constructs to HTML before the parse and blackfriday's own
`HTMLRenderer` renders the result, which keeps its text escaping and smart
punctuation intact. The custom-renderer seam remains available for subtask 2 if
the site needs it.

## Task 2:

Status: Done

Templates and assets. `html/template` templates and embedded static assets, one set
serving both entry points. Include the plain-HTML index pages per type and per tag,
backlink lists, and the reference list per page. Visual design is not the point;
navigability and correctness are.

It closed with the document layer in `internal/render`: `templates/` and
`assets/` embedded in the binary, and a `Renderer` that renders a page, the Index
page, a type index, a tag index and a virtual source page, and enumerates the
whole site as `[]Document`. `Documents()` is the one surface both entry points
will read, and it reports a URL produced twice instead of letting one document
overwrite another. A page shows a meta line (type, status, tags, aliases), then
its reference list, then its backlinks; an empty section is omitted, so a page
nothing links to has no backlinks heading. A type or tag with no page gets no
index, and a tag's identity is its normalised form, so every spelling shares one
page. No document emits a script, which is the `P9` gate held at the template
level. Addresses and the light/dark stylesheet are those of `D69`; the visual
design is expected to change once the site can be seen in a browser.

## Task 3:

Status: Done

`serve`. A local HTTP server with live reload on change, over the same renderer.
Server-side search, so search works here without any JavaScript at all.

It closed with `internal/serve`: a `Server` over `kb.Hashes` and the renderer's
`Documents()`, so the KB is reloaded when its content changes and served
unchanged when it has not. It answers a document, the assets, `/search`, and
`/__reload`. Search runs on the server through `index.Search`, so the form works
with JavaScript off; the one script anywhere is the reload helper, which the
server injects and `build` never writes. While the snippet's `[term]` marks and
a body's own `[[wikilinks]]` share a bracket, a pair is a mark only when it
holds exactly one of the query's terms. Every link the site emits was moved to
be relative to the document that carries it, because a page under `types/` has
to reach `../attention.html` for the output to survive being moved. The `serve`
command is registered in the CLI table and runs until stopped.

## Task 4:

Status: Done

`build`. Static site generation into an output directory, with an explicit,
predictable structure, no absolute paths that break when moved, and no dependency
on a running server.

It closed with `internal/build`: `Write` renders the KB through the same
`Documents()` the server answers from and writes each document at its URL, so the
site on disk is the site in the browser less the two things only a running server
can provide — the search page and the reload helper. A URL is decoded on the way
to disk, because a page with a space in its name is served at `two%20words.html`
and stored as `two words.html`; that decode is `render.FilePath`, shared with
`serve`, which had been 404ing exactly those pages. The directory is reconciled
rather than replaced: the manifest of what a build wrote is read back on the next
one, so a renamed page leaves no stale file behind and a file the author put
there by hand is never removed (`P4`). The default output is `<kb>/.stemma/site`,
inside the generated directory that is already gitignored and re-derivable, and a
relative `--out` is relative to the KB root rather than to the working directory.
`stemma build` is registered in the CLI table, so help and the generated
reference picked it up with no further change.

One thing was found rather than planned: writing to disk is what surfaced the
`serve` 404 on escaped names, which is why the decode is one exported function
both entry points read rather than a private detail of the writer.

## Task 5:

Status: Done

The graph view. A small graph on every page, showing that page and the pages it
is connected to: its backlinks and its outgoing links, one hop. And a whole-KB
graph page, `graph.html`, drawing every page and every wikilink. Both have the
same baseline: a server-rendered SVG, so the graph is present and navigable with
JavaScript off — its nodes are real links, not a picture. A script may replace
either with its own interactive layout, reading the graph data embedded in the
page as inert JSON.

The static graph is the feature and the script is the upgrade, as with the theme
toggle: nothing about the page depends on the script running, and the no-JS gate
in the next task checks that the graph is present and usable without it. Go
computes both layouts and emits `<svg>` with `<a>` nodes, and the embedded JSON
carries the same nodes and edges the SVG draws, so a script that redraws the
graph cannot show what the baseline did not.

It closed with one script drawing both shapes. The local graph is a two-column
star; the whole-KB graph is a ring of every page with the links as chords. There
a node is sized by degree — backlinks plus resolved outgoing links — and tinted
from the Everglade accents, both chosen in Go so the static SVG and the script
agree and a node keeps them between visits; the local graph stays uniform,
because its shape already says what connects to what. The script jitters its
seed positions and gives each spring its own rest length, so the web settles
lopsided rather than in a ring, and its hover tip names the page and its
backlinks, so the size explains itself.

## Task 6:

Status: Done

No-JS conformance gate. Build the site, then assert in a test that the entry
document, per-type indexes, backlink lists, page navigation, and reference lists
are all reachable and readable with JavaScript disabled. Fail the build if any of
them only work with JS. This task is the enforcement mechanism for `P9`; without
it the requirement erodes on the first convenient exception.

It closed as `TestNoJSConformance`, an assertion over a *built* site rather than
over the templates, because the output is what a reader is handed. It builds the
example wiki and reads every document back: each address in the document is
followed to the file it names, so a link that does not resolve fails the gate
wherever it sits — navigation, a type index, a backlink list, a source page, or a
graph node; the five navigation addresses must be reachable from every page;
every script must be either a static asset or inert JSON; the theme toggle must
be hidden until a script shows it; and no page may carry a marked link, which
from a reader's side is a page that is not there. Backlink and reference sections
are checked to hold links and text rather than a placeholder, and the fixture is
checked to produce both, so the gate cannot pass by checking nothing. It was run
against four deliberate breaks — a dropped navigation link, an inline script, an
unhidden toggle, and a link to a page that does not exist — and failed all four.

The gate is what made the static site part of `make check`: the new `site` target
builds the example wiki with the binary the same run just produced, so parsing,
rendering and writing are exercised end to end by CI, not only by the tests.

## Task 7:

Status: Not started

`export json`. Pages, links, citations, and bibliography in a documented shape with
a stable top-level structure. Decide and document whether output is one file or
JSONL, and include enough metadata to reconstruct the graph without reading the
source tree.

## Task 8:

Status: Not started

`export page`, scoped extraction. Depth control, citation closure, link pruning with
retained anchor text, the default summary versus `--report-pruned` detail, and the
KB-root output form. Include the pruned-link report in `--json` for machine use.

## Task 9:

Status: Not started

Extract validation gate. Assert that every extract lints clean under `--strict`,
resolves every internal link it kept, and resolves every citation it materialised.
Round-trip a KB, extract a page from it, and verify the extract stands alone. This
is the test that makes `D30` true rather than aspirational.

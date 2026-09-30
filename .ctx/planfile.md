---
title: Stemma
date: 2026-09-26
---

This document is the top-level planfile. The project is multi-tiered: each task
below is a delegate plan in `.ctx/delegates/`, and completing a delegate completes
the corresponding task here. Delegate documents follow this same template and are
marked as delegates.

Task 8 is the exception. The MCP server outgrew a single delegate and now has its
own plan in `plans/mcp/planfile.md`; this task tracks that plan's completion.

# Plan details

## What Stemma is

A harness-agnostic toolset for authoring and maintaining a contiguous,
citation-bearing knowledge base (KB) on disk. Pages are markdown with minimal
YAML frontmatter and `[[wikilinks]]`; sources live in BibTeX and are cited inline
as `[@key]`; drafts live in an inbox outside the page tree; nothing generated is
ever committed; a CLI is the universal surface and MCP is the native-tool path.
The project's own documentation is the example wiki, built by the tool.

## Read this first

Every decision below is recorded with its rationale in `.ctx/design/decisions.md`
as `D<n>`, and every principle as `P<n>`. The full interview is in
`.ctx/design/qa-session.md`. Those two documents are the specification of intent;
this planfile and its delegates are the specification of work. If a task appears
to contradict a decision, stop and raise it rather than working around it.

## Settled constraints that shape every task

Language is Go, with Python permitted only as a document-extraction shim. There is
no `package.json`, no npm, and no JS build step anywhere. Harness-specific
extensions are forbidden; native tooling is exposed over MCP, and skills follow
the open Agent Skills standard. Git is optional: every command must work in a
plain directory and must not require a repository.

Dependencies are a small pinned allowlist: `gopkg.in/yaml.v3` for frontmatter,
`github.com/BurntSushi/toml` for the manifest, `github.com/russross/blackfriday/v2`
for markdown rendering, and `modernc.org/sqlite` for the Tier-1 index. All four are
pinned in `go.mod` and present in the module cache, so a build works offline.
`yaml.v3` and `toml` are in real use; `blackfriday/v2` and `sqlite` are held in
`go.mod` by a no-op import behind a build tag in `internal/deps` until Tasks 5 and
6 need them. CLI dispatch is hand-rolled on the standard library so that `--json`
and exit-code handling stay uniform across roughly twenty verbs.

Blackfriday is a deliberate choice despite gogs, gitea, and Hugo having migrated
to goldmark: the project needs a custom renderer to turn `[[wikilinks]]` and
`[@key]` citations into real links, and blackfriday's `Renderer` interface is more
direct for that than goldmark's layered extension model. Do not re-litigate this.

## Cross-cutting rules

Every command supports `--json`. Exit codes are `0` clean, `1` validation
findings, `2` operational error. Tools are lenient by default and `--strict` turns
warnings into errors. Tools must preserve unknown fields verbatim and must never
destroy content they do not understand — this is the single most important
invariant in the project, and it applies to every writer.

Nothing generated is committed. Generated artifacts live in a gitignored
directory. The only committed entry point is a hand-written document.

## Ordering

Tasks are ordered by dependency, not by importance. Foundation blocks everything.
Sources and bibliography unblocks both extraction and rendering. Skills, docs, and
MCP come last because they consume the finished surface.

The foundation delegate is closed, so the format, the page model, and the core CLI
shape are settled rather than in flight. `internal/kb` and `internal/cli` are a
library the remaining delegates build on: changing them is normal, but an interface
change that another delegate depends on is a decision to raise rather than a step
to take quietly.

## Definition of done for the project

A person who is not the author can clone the repository, run the tool against the
in-tree example wiki, lint it clean under `--strict`, build the static site from
it, extract a scoped subset of it, and hand that subset to someone else without
any link or citation dangling.

# Tasks

## Task 1:

Status: Done

Foundation. Format specification, page model, frontmatter handling, wikilink
resolution, citation-key resolution, lint, and the create/read/rename core of the
CLI. Also the de-risking spike that decides whether `rename` can reliably rewrite
inbound links, which is the load-bearing assumption behind `[[wikilink]]`.
See `delegates/foundation.md`.

It closed with `U5` resolved in favour of title-based links, a golden corpus that
holds the preservation guarantee over both a crafted set of awkward files and the
example wiki itself, and `lint --strict` wired over that wiki as a build gate.
Everything after this task builds on the two packages it produced.

## Task 2:

Status: Done

CLI surface. Dispatch, KB discovery, manifest loading, uniform `--json` and exit
codes, and the remaining non-source commands including `status`, `archive`,
`promote`, and `env`.
See `delegates/cli-surface.md`.

It closed with the whole surface described once, in a command table that
dispatch, help and the generated CLI reference are all derived from, and with
one `--json` envelope used by every verb. It also added `tags` to the format, a
decision recorded in the register as `D63`.

## Task 3:

Status: Done

Sources and bibliography. BibTeX parse and serialise, identifier resolution for
DOI, arXiv, and ISBN, inline citation resolution, the `cite` family, built-in
formatters, virtual source pages, the large-KB bibliography directory form, and
validated BibTeX and CSL-JSON export.
See `delegates/sources-bibliography.md`.

## Task 4:

Status: Done

Source extraction. The Python shim contract, PDF and URL extraction, the `fetch`
command and its scratch area, opt-in vendoring, and graceful degradation when
Python is unavailable.
See `delegates/source-extraction.md`.

It closed with the script embedded in the binary and written into every KB by
`init`, a contract that says exactly what the two halves promise each other, PDF
and HTML reading behind it, `fetch` and `cite vendor` on top of that, and the
no-Python promise kept literally: the whole Go test suite passes on a machine with
no interpreter on `PATH` at all. Two substitutions were recorded as they happened
rather than after the fact — PyMuPDF alone instead of a pypdf fallback, and the
size limit passed by Go rather than frozen in the contract (`D66`).

## Task 5:

Status: Done

Index and retrieval. The Tier-1 SQLite FTS5 store, incremental updates,
transparent Tier-0 fallback, the benchmark that answers how far Tier 0 scales,
and the `search` and `graph` commands.
See `delegates/index-retrieval.md`.

It closed with `internal/index` as the index half of the core: a versioned
`.stemma/index.sqlite` rebuilt by deletion, an incremental `Refresh` stamped by
content hash, one Go tokenizer and one Go BM25 scorer shared by both tiers, and
a `Source` interface both tiers implement so search and graph are single
functions over it. `stemma index`, `stemma search` and `stemma graph` are the
surface; `status` and `env` report the real index state. `U1` was measured
rather than guessed — Tier 0 is comfortable to roughly a thousand pages, and the
index saves about a third of a search and pays for itself after roughly twenty —
and the finding is in `FORMAT.md` and the new `README.md`. The choices are
recorded as `D67` and `D68`.

## Task 6:

Status: Done

Render and export. The blackfriday renderer with custom link and citation
handling, `serve`, `build`, the graph view, the no-JS conformance gate, the
JSON dump, and scoped extraction with depth control, citation closure, and link
pruning. See `delegates/render-export.md`.

It closed with the whole surface the delegate set out to build. `internal/render`
resolves wikilinks and citations through the same scan lint reads, so rendering
and validation cannot disagree by construction rather than by convention, and it
emits a dozen documents per KB: every page, the generated home page, the Index,
the type and tag lists, a graph of the whole KB, and a virtual source page for
every key that is cited. `serve` answers those documents with server-side search
and live reload. `build` writes them to a directory that needs no server at all.
`stemma export json` dumps pages, links, citations and the bibliography for a
program; `stemma export page` writes one page, what it links to and the sources
they cite as a KB root in the same format, so every tool reads the result.

Four decisions came out of it and every one was put to the author rather than
taken quietly, because each is expensive to change once something depends on it:
`D69`, what a link and a citation become when they render; `D70`, that a dump
other systems read is one JSON document and carries page bodies so it stands
alone; `D71`, that an extract renames its root to the entry document, prunes what
it cannot keep into anchor text, and closes its references; and `D72`, that the
home page is generated from what the KB holds, wrapping the entry document rather
than replacing it. Two of those started as contradictions rather than choices — an
extract's root has no inbound links, so the criterion that an extract lints clean
could not have held — and the answers changed the format's rules rather than
working around them.

The delegate also changed code the earlier tasks wrote, and every change is in
`design/render-export-changes.md`: one scan shared with lint, a per-page finding
reader, a citation group returned in pieces, a body rewrite, a manifest writer,
and three commands added to the CLI table. It leaves behind the project's
definition of done minus its last clause, and `make check` runs all of it: the
example wiki lints clean under `--strict`, builds a site that works with
JavaScript off, produces an extract that is itself a KB and also lints clean, and
none of that is asserted by hand.

## Task 7:

Status: Done

The skill suite. The umbrella skill and the five workflow skills, with descriptions
reviewed as a set for disjointness, then exercised against the example wiki.
See `delegates/skills.md`.

It closed with `skills/` at the repository root: the umbrella carrying the format
rules, the query-first discipline, and the tool-versus-judgement boundary, and
five workflows -- research, author, maintain, query, publish -- whose
descriptions were reviewed as a set and disclaim one another. The umbrella's two
references are generated by `make skills` rather than committed, and
`make check-skills`, wired into `make check`, validates every `SKILL.md` against
the open Agent Skills standard. The five workflows were run through isolated
sub-agents, and the friction came back as stronger instructions rather than new
skills. The generated CLI reference was the one gap the exercise surfaced in
earlier work -- it named the `cite` and `export` families' members without their
flags -- so the generator was extended and the change recorded in
`design/skills-changes.md`.

## Task 8:

Status: Deferred

The MCP server. Deferred by the author on 2026-09-29 in favour of finishing Task 9.
The work is planned in its own plan, `plans/mcp/planfile.md`, because it needs an
enabling refactor of how the surfaces build their output and settles protocol and
packaging questions of its own. This task tracks that plan.

## Task 9:

Status: In progress

Documentation and CI. The example wiki as the project's own documentation,
`FORMAT.md` restructured as a wiki document, the README, the four CI gates, and the
residual unknowns.
See `delegates/docs.md`.

Scope that already exists: foundation Task 10 seeded the example wiki with a first
slice and wired gate 1 (`lint --strict` over it) into `make check`. This task grows
that wiki rather than creating it, and adds three gates rather than four.

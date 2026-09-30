---
title: Stemma
date: 2026-09-26
---

This document is the top-level planfile. The project is multi-tiered: each task
below is a delegate plan in `.ctx/delegates/`, and completing a delegate completes
the corresponding task here. Delegate documents follow this same template and are
marked as delegates.

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

Status: Not started

Render and export. The blackfriday renderer with custom link and citation
handling, `serve`, `build`, the graph view, the no-JS conformance gate, the
JSON dump, and scoped extraction with depth control, citation closure, and link
pruning. See `delegates/render-export.md`.

## Task 7:

Status: Not started

The skill suite. The umbrella skill and the five workflow skills, with descriptions
reviewed as a set for disjointness, then exercised against the example wiki.
See `delegates/skills.md`.

## Task 8:

Status: Not started

The MCP server. A curated subset of about ten tools over the core library — not a
CLI wrapper — with schemas shared with the CLI and identical validation and error
semantics.
See `delegates/mcp.md`.

## Task 9:

Status: Not started

Documentation and CI. The example wiki as the project's own documentation,
`FORMAT.md` restructured as a wiki document, the README, the four CI gates, and the
residual unknowns.
See `delegates/docs.md`.

Scope that already exists: foundation Task 10 seeded the example wiki with a first
slice and wired gate 1 (`lint --strict` over it) into `make check`. This task grows
that wiki rather than creating it, and adds three gates rather than four.

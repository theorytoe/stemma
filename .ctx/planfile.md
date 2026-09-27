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
for markdown rendering, and `modernc.org/sqlite` for the Tier-1 index. Only
`yaml.v3` is in the local module cache; the others fetch on first build. CLI
dispatch is hand-rolled on the standard library so that `--json` and exit-code
handling stay uniform across roughly twenty verbs.

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
MCP come last because they consume the finished surface. Schema and CLI shapes may
move between delegates during foundation; treat interface churn before the
foundation delegate closes as expected rather than as rework.

## Definition of done for the project

A person who is not the author can clone the repository, run the tool against the
in-tree example wiki, lint it clean under `--strict`, build the static site from
it, extract a scoped subset of it, and hand that subset to someone else without
any link or citation dangling.

# Tasks

## Task 1:

Status: Not started

Foundation. Format specification, page model, frontmatter handling, wikilink
resolution, citation-key resolution, lint, and the create/read/rename core of the
CLI. Also the de-risking spike that decides whether `rename` can reliably rewrite
inbound links, which is the load-bearing assumption behind `[[wikilink]]`.
See `delegates/foundation.md`.

## Task 2:

Status: Not started

CLI surface. Dispatch, KB discovery, manifest loading, uniform `--json` and exit
codes, and the remaining non-source commands including `status`, `archive`,
`promote`, and `doctor`.
See `delegates/cli-surface.md`.

## Task 3:

Status: Not started

Sources and bibliography. BibTeX parse and serialise, identifier resolution for
DOI, arXiv, and ISBN, inline citation resolution, the `cite` family, built-in
formatters, virtual source pages, and the large-KB bibliography directory form.
See `delegates/sources-bibliography.md`.

## Task 4:

Status: Not started

Source extraction. The Python shim contract, PDF and URL extraction, the `fetch`
command and its scratch area, opt-in vendoring, and graceful degradation when
Python is unavailable.
See `delegates/source-extraction.md`.

## Task 5:

Status: Not started

Index and retrieval. The Tier-1 SQLite FTS5 store, incremental updates,
transparent Tier-0 fallback, the benchmark that answers how far Tier 0 scales,
and the `search` and `graph` commands.
See `delegates/index-retrieval.md`.

## Task 6:

Status: Not started

Render and export. The blackfriday renderer with custom link and citation
handling, `serve`, `build`, the no-JS conformance gate, the JSON dump, and scoped
extraction with depth control, citation closure, and link pruning.
See `delegates/render-export.md`.

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

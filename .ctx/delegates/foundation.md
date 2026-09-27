---
title: Stemma — Foundation
date: 2026-09-26
---

This is a delegate plan. It covers Task 1 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Deliver the core library and the smallest CLI that can create and read a KB. This
delegate exists to make the format real and to answer the project's riskiest open
question before anything is built on top of it.

## What it delivers

A KB root on disk that lints clean under `--strict`, created and inspected by
`stemma init`, `new`, `list`, `show`, `rename`, `move`, and `archive`. Nothing
else is in scope: no bibliography beyond key resolution, no index, no rendering,
no MCP.

## The load-bearing risk

`rename` must rewrite every inbound link, because links are title-based (`D19`).
This is assumption `U5` in the decision register, and it is the reason this
delegate exists first. If `rename` cannot be made reliable, `D19` is wrong and the
format needs revisiting — which is far cheaper now than after the format has
calcified. Task 9 is therefore a gate, not a feature.

## Format decisions this delegate implements

Frontmatter is YAML with a deliberately small mandatory set: `title` and `type`
only. Everything else is optional (`D14`). Types default to `topic`, `concept`,
`note` and are extended by configuration; `source` and `index` are tool-owned and
not assignable by authors (`D46`, `D55`). `status` is `active` or `archived`;
drafts are not a status, they are inbox membership (`D47`).

Links are `[[wikilinks]]` resolved by title, slug, or alias, and an ambiguous
title is a hard error (`D19`). Directories inside `pages/` are organisational
only and confer no scope; there are no path-qualified links (`D50`, `P10`).
Drafts live in `inbox/` inside the KB root but outside `pages/`, and are excluded
from indexing, search, and export, and validated only leniently (`D49`).

Leniency is the default and `--strict` escalates warnings to errors (`D26`). The
hard invariant is that no writer may drop or rewrite content it does not
understand (`P4`). Round-trip fidelity is a tested property, not an aspiration.

## Dependencies

None. This delegate blocks all others. It pins the module path
`github.com/theorytoe/stemma` and the dependency allowlist.

## Interface churn

Task 2 will reshape command plumbing, and Tasks 3 to 6 will add fields and types.
Schema and CLI shapes may move while this delegate is open. Prefer clean
interfaces over stability here; stability starts when Task 1 closes.

# Tasks

## Task 1:

Status: Done

Repository scaffolding. `git init`, module path `github.com/theorytoe/stemma`,
layout of `cmd/stemma` plus `internal/`, dependency pins for the allowlist, build
and test entry points, a CI workflow running build, vet, and test, and a
`.gitignore` covering the generated-artifact directory.

## Task 2:

Status: Done

Write `FORMAT.md`. This is the format specification: page anatomy, the frontmatter
field table with mandatory versus optional, the type vocabulary and how
configuration extends it, wikilink syntax and resolution rules including
ambiguity, citation syntax, the `pages/` and `inbox/` conventions, the manifest,
reserved types, and the leniency and preservation rules. It is prose first; it
becomes a KB document later, in Task 7.

## Task 3:

Status: Done

Page model and frontmatter parsing on `yaml.v3`. Parse, validate, and serialise
without losing unknown fields or field order where avoidable. Round-trip fidelity
test: parse then serialise a page with unknown fields, nested structures, and
unusual scalars, and assert byte-equivalence of the parts the tool does not own.

## Task 4:

Status: Done

Wikilink resolver. Extract links from a page body, resolve by title then slug then
alias, and raise a hard error on ambiguity. Build the incrementally-updatable
backlink map and a Tier-0 in-memory graph over the page set. No index, no
database, no cache — this must be fast enough on a few thousand pages to make
Tier 0 real (`P1`).

## Task 5:

Status: Done

Citation key resolution, minimal scope. Read a bibliography file far enough to know
which keys exist, extract `[@key]` from page bodies, and report keys that are cited
but absent and keys that exist but are never cited. Full BibTeX handling belongs to
`delegates/sources-bibliography.md`; only what lint needs lives here.

## Task 6:

Status: Done

`lint`. Implement every invariant: required frontmatter fields, unknown types,
reserved-type misuse, unresolved and ambiguous links, orphan pages, citation-key
problems, and inbox exclusion. Support `--strict` and `--json`, and return exit
code `1` on findings and `2` on operational failure.

## Task 7:

Status: Not started

Core CLI verbs: `init`, `new`, `list`, `show`, `move`, `archive`. `init` scaffolds
a KB root with manifest, entry document, `pages/`, `inbox/`, and a bibliography
file. `new` creates a page or an inbox draft. `show` renders one page with links
and citations resolved, and supports `--path` and `--raw` for scripts.

## Task 8:

Status: Not started

`rename`. Retitle a page and rewrite every inbound link, then verify by re-running
resolution. Handle the pathological cases explicitly: a page renamed to a title
that already exists, a page whose old title collides with an existing alias, a page
with no inbound links, a page referenced by alias rather than title, and a partial
rename interrupted midway. Make it idempotent where possible and covered by
tests for each case.

## Task 9:

Status: Not started

Gate. Prove or disprove `U5`. State plainly whether `rename` rewriting inbound
links is reliable enough to justify `D19`. If it is not, stop and raise it before
the format spreads into other delegates — the alternative is path-qualified links
or a link-rewriting pass on every operation, and both have costs that should be
decided deliberately rather than discovered.

## Task 10:

Status: Not started

Golden corpus. Seed the example wiki with a first slice as a test fixture, wire
`lint --strict` over it into CI, and add golden-file tests for parsing and
serialisation. This corpus grows in later delegates; establish the harness here.

---
title: Stemma — CLI surface
date: 2026-09-26
---

This is a delegate plan. It covers Task 2 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Make the command surface complete and uniform. Task 1 proved the format and
shipped a handful of verbs; this delegate makes the whole tool feel like one tool,
and adds the commands that are neither about sources nor about rendering.

## What it delivers

Dispatch and help for every verb, KB discovery, manifest handling, uniform
machine-readable output and exit codes, and the commands `status`, `archive`,
`promote`, `list` filters, `show` modes, and `doctor`. It also generates the CLI
reference that the skill suite consumes in Task 7.

## Discovery and manifest

One KB per invocation (`D41`). Resolution order is explicit argument, then
environment variable, then walk-up from the working directory looking for
`stemma.toml` — the same ergonomics as git, which is the model agents already
have. No registry and no daemon: a registry is stateful and breaks the first time
a KB moves, and `P8` already established that sharing is just files.

`stemma.toml` is optional-with-defaults and is the discovery marker; `init` writes
one, but a hand-made directory must work without it (`D59`, `P1`). Unknown keys
are preserved untouched, because a future version's configuration has to survive a
round trip through an older binary.

## Machine contract

Every command supports `--json` (`D57`). Exit codes are `0` clean, `1` validation
findings, `2` operational error. This separates "the KB has problems" from "the
tool broke", which is the distinction both CI and agents need; without it a lint
failure and a missing file are indistinguishable.

## Dependencies

Task 1, which fixes the page model, the resolver, and the initial verbs. Task 7
consumes the generated CLI reference, so keep it accurate rather than handwritten.

# Tasks

## Task 1:

Status: Done

Dispatch and help. A hand-rolled dispatcher over the standard library, with a
uniform shape for verb, noun families (`cite`, `export`), flags, and usage text.
No CLI framework. Help must be good enough that an agent can discover the surface
without reading the source.

## Task 2:

Status: Done

KB discovery. Implement the three-step resolution order, make it available to every
command, and cover the failure modes: no KB found, several candidate roots on the
walk up, an explicit path that is not a KB, and a KB root that is read-only.

## Task 3:

Status: Done

Manifest load and save. Parse `stemma.toml` on `BurntSushi/toml`, apply defaults for
every absent key, and preserve unknown keys verbatim on write. Define the keys:
`title`, `description`, `types`, `default_type`, `citation_style`, `ignore`, and
`[export] default_depth`. Document that `citation_style` selects from the built-in
formatters only, and is not a CSL style identifier.

## Task 4:

Status: Done

`--json` and exit codes, applied uniformly across every verb from both delegates.
Define the envelope shape once and use it everywhere; do not let each command
invent its own. Audit Task 1's verbs and bring them into line.

## Task 5:

Status: Done

`status`. A health summary: page counts by type, orphans, uncited sources,
ambiguous titles, index freshness, and inbox size. Read-only and fast. It must be
useful on a KB large enough that reading files directly is slow, which means it
should use the index when one exists.

## Task 6:

Status: Done

`list` filters and `show` modes. `list` filters by type, tag, directory, and status,
and supports `--json`. `show` supports default rendering, `--path` for scripts, and
`--raw` for the source text. Refine what Task 1 shipped.

## Task 7:

Status: Done

`archive` and `promote`. `archive` sets `status: archived` and appends the reason
to the document; the tool never deletes a page (`D47`). `promote` is the single
inbox transition: validate against the required fields, require a type, resolve
links that pointed at the draft, move the file into `pages/`, and report what
changed. Everything the inbox permits must become an error at promotion.

## Task 8:

Status: Done

`doctor`. Detect and report the optional environment: Go runtime, the Python
interpreter, whether a PDF extractor is importable, and whether SQLite is
available. Go-primary with an optional Python shim means a missing extractor should
be diagnosable rather than mysterious. This command earns its place only if it
stays small; if it grows into a dependency manager, cut it (`U4`).

## Task 9:

Status: Done

Generate the CLI reference. Emit a single markdown document describing every verb,
flag, exit code, and JSON envelope, generated from the command definitions so it
cannot drift. Task 7 of the project copies this into the umbrella skill.

## Task 10:

Status: Done

Command-level tests. Table-driven tests for dispatch, discovery, manifest round
trips, exit codes, and every `--json` envelope. Assert that a KB in a plain
directory with no git repository and no manifest works end to end (`P5`).

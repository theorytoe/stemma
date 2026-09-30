---
title: Stemma — Documentation and CI
date: 2026-09-26
---

This is a delegate plan. It covers Task 9 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Write the project's documentation as a knowledge base, and make it the thing CI
exercises. The wiki, the specification, and the test fixture are the same artifact,
which is the point.

## The example wiki is the documentation

The in-repo example wiki is the project's own documentation, written as a KB and
built by the tool (`D54`). It does three jobs at once: it is the documentation, it
is the integration fixture, and it is live proof that the format holds up on real
content rather than on contrived samples.

The failure mode is deliberate and cheap to accept. If the format is bad, the
project's own documentation degrades first and degrades loudly — which means the
first person to suffer from a design mistake is the author, immediately, instead of
a stranger six months later.

## The specification becomes a document

`FORMAT.md`, drafted as prose in `delegates/foundation.md` Task 2, is restructured
here into a document inside the wiki. The specification then lives in the format it
specifies, so it is exercised by the same lint and the same renderer as everything
else. Concretely: the specification cannot drift, because a change to the format
that is not reflected in the specification leaves internal links dangling and fails
the build.

## CI gates

Four gates, each one enforcing a decision that would otherwise erode quietly:

1. `lint --strict` over the example wiki. If the project's own documentation cannot
   lint strict, the release is not shippable.
2. A full `build`, which exercises parsing, resolution, rendering, citations, and
   virtual source pages together.
3. The no-JS conformance check from `delegates/render-export.md`, which is the
   enforcement mechanism for `P9`.
4. The extract validation gate from the same delegate, which is what makes `D30`
   true rather than aspirational: an extract must be a valid KB root that lints
   clean.

The tier-parity tests from `delegates/index-retrieval.md` belong in the same run,
because the fallback path is the one a new user actually experiences and the one
most likely to rot unseen.

The gates live here rather than in `delegates/foundation.md` because three of the
four require surfaces that do not exist until late. The plumbing — a CI workflow
that builds and tests — is in foundation Task 1; the gates are added here.

## Dependencies

All prior delegates. This delegate adds no format behaviour. A need for new format
behaviour discovered while writing the documentation is a signal that an earlier
delegate closed too early, and should be raised rather than patched here.

# Tasks

## Task 1:

Status: Done

Structure the example wiki. Decide the page set and the type of each page —
rationale, format specification, command reference, and worked examples — then
place and link them. Per page, choose a type from the vocabulary, link it into the
graph, and keep the wiki lint-clean as it grows. This is also the first real
exercise of the authoring workflow, so record friction as it appears.

The wiki has grown to 17 pages, a manifest, and a bibliography, all reachable from
the index and lint-clean under `--strict`. The specification and the command
reference are present. What is missing is a dedicated worked-examples page; the
closest thing is the walkthrough in `lifecycle.md`.

Done 2026-09-29: `wiki/pages/worked-example.md` ("A first KB, end to end") walks
init, add, write, cite, check, inspect and build, with the real output of every
command, and `index.md` links it. Writing it found the family-name bug in
`--help` recorded in `design/docs-changes.md`.

## Task 2:

Status: Resolved — FORMAT.md stays canonical

Restructure `FORMAT.md` into a wiki document. Preserve the substance from
foundation Task 2 — page anatomy, the frontmatter field table, the type vocabulary
and its extension, wikilink resolution including ambiguity, citation syntax, the
`pages/` and `inbox/` conventions, the manifest, reserved types, leniency and
preservation — and place it in the graph so it is reachable and so its internal
links are checked.

Now: `wiki/pages/format.md` is a 32-line summary, while the root `FORMAT.md` is 728
lines, still declares itself the canonical specification, and is still copied into
the skill references by the Makefile. No wiki page references `FORMAT.md`. The
restructure has not happened.

Decided 2026-09-29: it is not restructured. `FORMAT.md` stays the canonical,
self-contained specification and the wiki documents it rather than containing it.
The premise — that the spec should live in the graph so lint exercises it — is
given up deliberately, because the spec is one normative artifact that the skill
suite ships as a single file, and its self-containment is worth keeping. The
wiki's `format.md` is the documentation's entry point to the format, and the
Makefile comment that promised the move is corrected.

## Task 3:

Status: Done

The README. What the project is, how to install from source, and the two-minute
path to a first KB. State two things plainly so neither is a surprise later: that
the name collides with an existing `stemma` project and why that was accepted, and
what the dependency allowlist contains and why it is small.

Now: the README states what the project is and how to build and install it. It is
missing the two-minute first-KB path, the name-collision note, and the dependency
allowlist note.

Done 2026-09-29: the README gained an install section, a first-KB quickstart that
points at the worked example, and a dependency section naming the four pinned
modules and why the list is small. The name-collision note the task asked for was
dropped on the author's instruction — the collision does not matter in practice —
so `U6` closed as no confusion rather than as a caveat to publish.

## Task 4:

Status: Done

CI gates. Add the four gates above plus the tier-parity tests to the workflow
established in foundation Task 1, and confirm each one actually fails when it
should — a gate that cannot fail is decoration. Verify the failure output is
actionable, since a lint failure on the documentation is the first thing a
contributor will see.

All four gates are now wired. `make check` runs `lint-wiki` (gate 1), `site` (gate
2), `test`, which includes `internal/build`'s no-JS conformance test (gate 3), and
`extract`, which is backed by `internal/export`'s gate tests (gate 4). The
tier-parity tests run under `test` as well. `ci.yml` runs `make check`.

Each gate was confirmed to fail when it should. Gate 1 fails on an unresolved link
and on an unknown type. Gate 2 fails on a page whose frontmatter cannot be parsed
and passes once it is fixed (checked 2026-09-29). The no-JS gate was run against
four deliberate breaks — a dropped navigation link, an inline script, an unhidden
toggle, and a link to a missing page — and failed all four, as recorded in
`delegates/render-export.md`. The extract gate has negative runs: keeping a link
that leaves the slice fails it, and so does a root that keeps its own path.

## Task 5:

Status: Done

Close out the residual unknowns and record the outcomes in
`.ctx/design/decisions.md`. `U3`: which citation styles the built-in formatter
actually needs, decided by looking at the real sources in the example wiki. `U4`:
whether `env` and `fetch` earn their place, decided by use. `U6`: whether the
accepted `stemma` name collision has caused any real confusion.

Closed 2026-09-29. `U3`: both styles are kept and **numeric becomes the default**,
which changed code and is recorded in `design/docs-changes.md`. `U4`: both earn
their place — `fetch` is used by the research skill, `env` diagnoses a failed
fetch. `U6`: no confusion, so the README drops the external tool's name. All three
rows in the register now carry their answer.

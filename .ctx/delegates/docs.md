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

Status: Partly done by foundation Task 10

Structure the example wiki. Decide the page set and the type of each page —
rationale, format specification, command reference, and worked examples — then
place and link them. Per page, choose a type from the vocabulary, link it into the
graph, and keep the wiki lint-clean as it grows. This is also the first real
exercise of the authoring workflow, so record friction as it appears.

Already in place at `wiki/`: seven pages (an index, a rationale page using a
manifest-added type, two concept pages, a note, a drafts page, and an archived page
with an unknown nested field), a manifest, and a bibliography with one citation.
It lints clean under `--strict`. What is missing is the specification, the command
reference, and the worked examples.

## Task 2:

Status: Not started

Restructure `FORMAT.md` into a wiki document. Preserve the substance from
foundation Task 2 — page anatomy, the frontmatter field table, the type vocabulary
and its extension, wikilink resolution including ambiguity, citation syntax, the
`pages/` and `inbox/` conventions, the manifest, reserved types, leniency and
preservation — and place it in the graph so it is reachable and so its internal
links are checked.

## Task 3:

Status: Not started

The README. What the project is, how to install from source, and the two-minute
path to a first KB. State two things plainly so neither is a surprise later: that
the name collides with an existing `stemma` project and why that was accepted, and
what the dependency allowlist contains and why it is small.

## Task 4:

Status: Gate 1 already in place

CI gates. Add the four gates above plus the tier-parity tests to the workflow
established in foundation Task 1, and confirm each one actually fails when it
should — a gate that cannot fail is decoration. Verify the failure output is
actionable, since a lint failure on the documentation is the first thing a
contributor will see.

Gate 1 is done: `make lint-wiki` lints `wiki/` under `--strict` and `make check`
depends on it, so CI runs it. It was confirmed to fail on both an unresolved link
and an unknown type. Gates 2, 3, and 4 need surfaces that do not exist yet.

## Task 5:

Status: Not started

Close out the residual unknowns and record the outcomes in
`.ctx/design/decisions.md`. `U3`: which citation styles the built-in formatter
actually needs, decided by looking at the real sources in the example wiki. `U4`:
whether `doctor` and `fetch` earn their place, decided by use. `U6`: whether the
accepted `stemma` name collision has caused any real confusion.

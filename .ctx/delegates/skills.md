---
title: Stemma — Skill suite
date: 2026-09-26
---

This is a delegate plan. It covers Task 7 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Make the finished tool usable by an agent. This delegate produces agent-facing
instructions only — no format behaviour, no code paths in the core library. The MCP
server is a separate delegate; the project's own documentation is separate again.

## Six skills, and why the count is what it is

One umbrella and five workflows (`D61`). Skill selection is driven by description
matching, so **overlapping descriptions cause the wrong skill to load**, and a skill
that tries to cover everything fires on everything and becomes useless. Descriptions
therefore have to be reviewed as a set, not one at a time, and disjointness is a
correctness property rather than a style preference.

The umbrella `stemma` carries the invariants and the references: the format rules,
the query-first discipline, the tool-versus-judgement boundary, and where things
live. Its references carry the format specification (owned by `delegates/docs.md`)
and the generated CLI reference (owned by `delegates/cli-surface.md`).

The five workflows are `stemma-research`, `stemma-author`, `stemma-maintain`,
`stemma-query`, and `stemma-publish`. Research and maintain carry the most
instruction, because authoring/ingest and maintenance are the two value centres
that distinguish this project from a folder of markdown (`D2`). Query must stay
thin — it is mostly search, follow, cite — because bloat there is paid on every
retrieval.

## How skills invoke the tool

Skills state the operation and give the **CLI form as canonical**, noting the
equivalent MCP tool where one exists (`D62`). A skill that names MCP tools only
works where MCP happens to be wired up, which quietly reintroduces the harness
coupling that `D7` exists to prevent. The CLI is the surface that exists
everywhere: in a container, in CI, in a harness with no MCP, and at a human's
prompt.

## Two things that belong in the umbrella

Both are failure modes that make a knowledge base worthless in practice.

**Query-first.** Read the KB before answering from memory or from raw files. The
characteristic failure is an agent answering from its own recollection while a
correct, cited page sits unread on disk.

**The boundary.** Tools own anything with an invariant; the agent owns anything
requiring judgement. The agent never computes a path, a citation key, or an index
by hand — it asks the tool (`D60`). Every guessed filename or invented BibTeX key
is arithmetic on the KB's structure, which is exactly where language models are
confidently wrong.

## Conformance

Skills follow the open Agent Skills standard, in `.agents/skills/`, with no
per-harness adapters (`D11`, `D5`). No harness-specific tool names, no harness
configuration, and no assumptions about which agent is reading.

## Dependencies

Tasks 2 through 6 of the project, since the skills describe the finished surface.
Task 3 references the example wiki from `delegates/docs.md`; if that wiki is not
yet written, exercise against a scratch KB and revisit.

# Tasks

## Task 1:

Status: Not started

The umbrella skill. Format rules, the query-first discipline, the tool-versus-
judgement boundary, and where things live. References point at the format
specification and the generated CLI reference rather than restating them, so there
is one copy of each. Validate against the Agent Skills standard: name, description
length, and frontmatter.

## Task 2:

Status: Not started

The five workflow skills. One per workflow, each stating the operation and giving
the CLI form as canonical. Write all five descriptions together and review them as
a set, confirming each fires only on its own workflow and that no two overlap.
Keep `stemma-query` deliberately thin.

## Task 3:

Status: Not started

Exercise and validate. Assert the suite loads and validates under the standard,
then run each skill against the example wiki and record where the instructions were
insufficient. The expected failure is an agent reaching for a raw file or a
fabricated path instead of a command; where that happens, the fix is to strengthen
the relevant instruction rather than to add a new skill.

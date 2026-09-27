---
title: Stemma — MCP server
date: 2026-09-26
---

This is a delegate plan. It covers Task 8 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Expose the knowledge base to agents as native tools rather than as shell commands,
without coupling the project to any one harness. MCP is the harness-agnostic path
(`D8`): it is understood by Pi, Claude Code, Cursor, and others, and a server can be
written in Go, which keeps the technology decision intact (`D6`).

## Why this delegate exists at all

Without it, the only way an agent reaches the KB is by shelling out to the CLI.
That works everywhere and remains the universal fallback, but it costs a process
spawn per operation, returns text for the model to parse, and offers no schema for
the model to reason about. A native tool surface fixes all three. The CLI does not
go away; it stays canonical for instructions (`D62`) and remains the only surface
in environments with no MCP.

## A curated subset, not a mirror

Roughly ten tools, not twenty (`D58`). Every MCP tool schema is loaded into the
model's context, so a full mirror is a permanent tax paid on every conversation for
capabilities that are rare. The subset is the read-and-query core plus explicit
mutations:

Reads — resolution, listing, showing, search, graph traversal, citation lookup.
Mutations — `new`, `promote`, `cite add`, and `lint`.

Everything else stays on the CLI. If a tool is added here later, something else
should be considered for removal, because the budget is the context window and not
the tool count.

## Built on the core library, not on the CLI

The server wraps the core library directly. Shelling out to the CLI would
re-implement process handling, lose typed errors, and make every call pay for
startup. Sharing the library also guarantees that validation, resolution, and
leniency behave identically on both surfaces — a divergence between the CLI and
MCP would be a correctness bug, not an inconsistency.

## Contracts to keep identical

Output payloads match the `--json` shapes from `delegates/cli-surface.md` (`D57`),
and error semantics match the exit-code convention: validation findings are
distinguishable from operational failure. Transport is stdio, which is the
lowest-common-denominator MCP transport and the one that needs no port, no daemon,
and no discovery (`D41`).

## Dependencies

Tasks 1, 2, 3, 5, and 6 of the project — the server exposes their commands and
inherits their schemas. Task 9 of the project documents it; this delegate supplies
the tool schemas that documentation is generated from.

# Tasks

## Task 1:

Status: Not started

Tool curation and schema design. Choose the subset, and for each tool define the
input schema, the output payload, and the error shapes, reusing the CLI's types
rather than defining parallel ones. Document explicitly which commands are
deliberately absent and why, so the omission reads as a decision rather than an
oversight.

## Task 2:

Status: Not started

The stdio server, built on the core library. Lazy KB resolution so a server can be
configured with one root or discover one per call, structured errors that map onto
the CLI's exit-code semantics, and no daemon, no port, and no global state (`D41`).

## Task 3:

Status: Not started

Parity and integration tests. Assert that each exposed tool returns the same result
as the corresponding CLI command on the same corpus, and that both surfaces agree
on validation failures. Then exercise the server from at least two different
harnesses to confirm the absence of harness-specific assumptions (`D7`, `P2`).

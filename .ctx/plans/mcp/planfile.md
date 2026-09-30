---
title: Stemma MCP server
date: 2026-09-29
---

# Plan details

Expose the knowledge base to agents as native MCP tools over stdio, built on the
core library rather than by shelling out, with payloads that equal the CLI's
`--json` output.

This began as Task 8 of `.ctx/planfile.md`. It was moved to its own plan because
it is not a leaf task: it needs an enabling refactor of how the surfaces build
their output, and it settles protocol and packaging questions of its own. The
top-level Task 8 now points here and tracks this plan's completion.

Deferred by the author on 2026-09-29, in favour of finishing the main plan's
Task 9 first. No task here has started.

## Settled by the author, 2026-09-29

- Own plan, same module. The server lives in `internal/mcp` and ships as a second
  binary at `cmd/stemma-mcp`. No new repository, no nested module.
- Transport is hand-rolled JSON-RPC 2.0 over newline-delimited stdio, on the
  standard library alone. This keeps the four-package dependency allowlist and the
  hand-rolled dispatch posture of the CLI. The price is that the project tracks
  protocol versions itself.
- Reuse comes from exporting what already exists in `internal/…`, not from a shim
  or a new shared package. `internal/mcp` will import `internal/cli` for the
  envelope and the payload builders. That makes a surface depend on a surface,
  which is a mild departure from `D10`; it is accepted because it makes the `D57`
  payload parity structural rather than a thing two copies must agree on.
- About ten tools, not a mirror (`D58`): the read core plus the four mutations
  `new`, `promote`, `cite add` and `lint`. Everything else stays on the CLI, and
  the omissions are documented as decisions.
- Contracts are unchanged. Payloads equal the CLI `--json` shapes (`D57`), errors
  map onto the exit-code semantics (clean, validation findings, operational
  failure), and the server is stdio with no daemon, no port and no global state
  (`D41`).

## The enabling refactor

The envelope, the finding projection, fourteen payload structs and roughly
fifteen builders currently live unexported in `internal/cli`. Task 1 exports
them. Most already have usable signatures; only the few that take the unexported
`*output` need reshaping. `gopls` does the export and rename sweep.

## Dependencies

The main project's Tasks 1, 2, 3, 5 and 6 supply the commands and schemas this
server exposes; all are complete. The server's own documentation is Task 7 here.
The main plan's Task 9 documents the project as a whole and does not describe the
MCP surface; where it needs the tool set it consumes the schema artifact this plan
produces.

## Open risks

The MCP specification was materially rewritten for the 2026-07-28 version:
`server/discover` replaces the `initialize` handshake, the model is stateless with
per-request `_meta`, and multi-round-trip requests replace server-initiated ones.
Hand-rolling means choosing the protocol version to support and tracking it
deliberately rather than inheriting the tracking from a dependency.

The two-harness check the delegate requires is non-deterministic and stays out of
CI, as it did for the skill suite in Task 7.

# Tasks

## Task 1:

Status: Not started

Export the shared surface payloads. Move the envelope, the finding projection, the
payload structs and the builders the tools need out of the realm of unexported
`internal/cli` symbols, reshaping only what depends on the unexported `*output`.
No behaviour change: every existing CLI test must pass untouched. Use `gopls` for
the sweep and record each change to the earlier tasks' code as it is made.

## Task 2:

Status: Not started

Tool curation and schema design. Choose the subset of about ten tools and define,
for each, the input schema, the output payload and the error shapes, reusing the
CLI's types rather than parallel ones. Document explicitly which commands are
absent and why. Emit the schema set through a command or a make target so the
documentation task consumes a generated artifact.

## Task 3:

Status: Not started

The stdio server, hand-rolled on the standard library. JSON-RPC 2.0 framing over
newline-delimited stdio, protocol-version handling, `initialize` and
`initialized`, `tools/list` and `tools/call`. Stdout carries protocol traffic
only; logs go to stderr. KB resolution is lazy, so a call may name a root or
discover one. No daemon, port or global state.

## Task 4:

Status: Not started

The tool handlers. Wire each curated tool to the shared builders over the core
library, so validation, resolution and leniency behave identically on both
surfaces. Map validation findings and operational failures onto MCP results with
the same distinction the exit codes make.

## Task 5:

Status: Not started

The `stemma-mcp` binary and the build gate. A second command under `cmd/`, built
and installed alongside `stemma`, and a make gate that keeps it built, vetted and
tested with the rest.

## Task 6:

Status: Not started

Parity and integration tests. Assert that each tool returns the same payload as
the corresponding CLI command under `--json` on one corpus, and that both surfaces
agree on validation failures. Exercise the server from at least two harnesses to
confirm the absence of harness-specific assumptions; that check stays out of CI.

## Task 7:

Status: Not started

Documentation. Describe the server where it can be generated rather than written:
the tool reference is emitted from the schema set Task 2 produces, so a schema and
its documentation cannot drift. Cover the curated subset and the commands
deliberately left on the CLI, the stdio invocation and how a harness is configured
to run it, and the payload and error contracts that match the CLI's. Keep the
harness instructions to the standard configuration file each client reads, with no
harness-specific extension. Add a gate that regenerates the reference and fails
when it is stale, the way the skill suite's references are handled.

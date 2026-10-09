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
Task 9 first. All seven tasks are done as of 2026-10-08.

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

Status: Done

Export the shared surface payloads. Move the envelope, the finding projection, the
payload structs and the builders the tools need out of the realm of unexported
`internal/cli` symbols, reshaping only what depends on the unexported `*output`.
No behaviour change: every existing CLI test must pass untouched. Use `gopls` for
the sweep and record each change to the earlier tasks' code as it is made.

Done 2026-10-07. The gopls rename sweep exported the envelope (`Response`,
`JSONFinding`, `JSONFindingOf`), the payloads (`LintSummary`, `ShowLink`,
`ShowCitation`, `ShowReport`, `ListEntry`, `StatusReport`, `IndexState`,
`PromoteReport`, `CiteAddReport`) and the builders (`SummariseLint`, `Describe`,
`Resolve`, `ResolveDraft`, `LinksThatWillResolve`, `SearchSource`, `UnderDir`).
The three reshapes: `list`'s filter loop became `ListEntries`, which never
returns nil, so the JSON nil-guard moved into the builder; `status`'s survey
became `GatherStatus`, returning `DraftPaths`' error; `cite add`'s `writeEntry`
split into the pure `ApplyEntry` and the unexported `emitCiteAdd`. Left
unexported on purpose: `discover` and KB loading, which Task 3's lazy KB
resolution shapes, and the `*output` emitters of commands outside the curated
subset (`graphListings`, `emitVendor`, `printExtract`, `unfinished`). No test
changed and the CLI's `--json` output is byte-identical.

## Task 2:

Status: Done

Tool curation and schema design. Choose the subset of about ten tools and define,
for each, the input schema, the output payload and the error shapes, reusing the
CLI's types rather than parallel ones. Document explicitly which commands are
absent and why. Emit the schema set through a command or a make target so the
documentation task consumes a generated artifact.

Done 2026-10-07. The ten tools: status, list, search, show, graph, new, promote,
cite add, cite show, and lint; cite show joined the read core at this task, and
the set is recorded as `D80`. The registry is internal/mcp/tools.go: input
schemas written by hand, payload shapes walked off the CLI's structs by
internal/mcp/shape.go so they cannot drift, and an omission with a reason for
every verb left behind. The generator internal/mcp/gen writes
docs/mcp-tools.json through the Makefile's mcp-schema target, now part of
check; the artifact carries the envelope, each tool's input and result, the
error outcomes, and the omission table. Drift tests hold every tool and every
omission against the command table, so a verb added to the CLI must land in
one of the two lists.

Changes to earlier tasks' code: the four payloads that were anonymous maps
became exported structs — ListReport, SearchReport, GraphReport, NewReport —
so a schema could name them (CLI --json output parsed-shape identical); the
cite-show payload followed Task 1's sweep (CiteShowReport, CiteShowField,
CiteFields); and CommandNames exposes the command table for the drift tests.

## Task 3:

Status: Done

The stdio server, hand-rolled on the standard library. JSON-RPC 2.0 framing over
newline-delimited stdio, protocol-version handling, `initialize` and
`initialized`, `tools/list` and `tools/call`. Stdout carries protocol traffic
only; logs go to stderr. KB resolution is lazy, so a call may name a root or
discover one. No daemon, port or global state.

Done 2026-10-07, with the protocol choice the plan's open risks called for:
the server speaks protocol 2026-07-28 and nothing else, recorded as `D81`.
The task's wording named `initialize` and `initialized`; the 2026-07-28
revision retired the handshake, so the server answers `server/discover`,
`tools/list`, `tools/call` and `ping`, refuses batches as invalid requests,
and answers a legacy `initialize` with the supported versions in the error's
data. Every request names its version in `_meta`; tools/call holds arguments
to the registry's input schema (internal/mcp/args.go) before a handler runs,
and everything after that gate is an envelope result with `isError` mirroring
`ok`. A call names a KB root in `_meta` as `stemma/kb`, else `cli.Discover`
runs per call — the export Task 1 deferred for exactly this. The handlers map
is a parameter of `Serve`; Task 4 wires the ten, and the tests prove the
machinery with handlers of their own.

## Task 4:

Status: Done

The tool handlers. Wire each curated tool to the shared builders over the core
library, so validation, resolution and leniency behave identically on both
surfaces. Map validation findings and operational failures onto MCP results
with the same distinction the exit codes make.

Done 2026-10-07. `ToolHandlers` (internal/mcp/handlers.go) wires all ten: each
handler parses its arguments, loads the KB, and calls the builder the command
calls — `GatherStatus`, `ListEntries`, `SearchSource` with `index.Search`,
`Resolve` with `Describe`, the index walk, `CreatePage`, `Promote`,
`source.New` with `ApplyEntry`, `CiteShowReport` from the bibliography, and
`SummariseLint` with the findings. The cite-add resolver needed no export:
`cli.newResolver` was already an alias for `source.New`.

Changes to earlier tasks' code: `new` and `promote` carried their logic in the
run closures, so the shared builders had to exist — `cli.CreatePage(k, title,
pageType, draft, mode)` and `cli.Promote(k, name, pageType)` were extracted
byte-identically, and the closures now call them; `cli.NonNil` was exported
for the graph walk's neighbours; and `new`'s input schema gained `strict`, the
only other verb besides lint where the flag changes behaviour, so the artifact
was regenerated. One message was adapted rather than copied: cite add's
refusal of an identifier beside identifier fields names the tool's arguments,
not the CLI's flags. Findings and operational failures map exactly as the
exit codes do: a finding is a result with `ok` false (exit 1), an
operational failure an envelope error (exit 2), and `isError` mirrors `ok`.

The tool handlers. Wire each curated tool to the shared builders over the core
library, so validation, resolution and leniency behave identically on both
surfaces. Map validation findings and operational failures onto MCP results with
the same distinction the exit codes make.

## Task 5:

Status: Done

The `stemma-mcp` binary and the build gate. A second command under `cmd/`, built
and installed alongside `stemma`, and a make gate that keeps it built, vetted
and tested with the rest.

Done 2026-10-07. `cmd/stemma-mcp` is main-only: flags, the server wiring, and
an exit code — 0 when the session ends, 2 when the server itself fails;
findings stay inside tool results. `--kb` pins the KB the way `stemma`'s does,
which put a launch-time root between the per-call `_meta` name and the
environment, so `Serve` became a `Server` with fields (Task 3's tests moved
with it) and `D81` records the amended precedence. The build and install
targets build and install both binaries under the same version stamp, and
`make check` gained `check-mcp`, which holds a discover-and-list conversation
with the shipped binary and fails when the answers stop matching.

The `stemma-mcp` binary and the build gate. A second command under `cmd/`, built
and installed alongside `stemma`, and a make gate that keeps it built, vetted and
tested with the rest.

## Task 6:

Status: Done

Parity and integration tests. Assert that each tool returns the same payload as
the corresponding CLI command under `--json` on one corpus, and that both
surfaces agree on validation failures. Exercise the server from at least two
harnesses to confirm the absence of harness-specific assumptions; that check
stays out of CI.

Done 2026-10-07. The CI half is internal/mcp/parity_test.go: eleven cases run
each surface over one corpus — a KB with resolved, unresolved, and ambiguous-adjacent links, a defined and a dangling citation, tags, and two drafts — and
hold the envelopes to DeepEqual, with the CLI's exit code asserted against its
twin each time; seven more cases assert the surfaces refuse identically
(promote's draft-validation refusal is the exit-1 twin, the rest exit 2), and
the one refusal whose words differ by design is held to "both refuse with a
message". internal/mcp/binary_test.go drives the built binary as a subprocess
over real pipes — discover, list, call, clean exit — and skips when the binary
has not been built, the way the shim tests skip without python3.

The out-of-CI half: `make mcp-probe` runs internal/mcp/probe, a raw stdio
client that walks the curated set — discover, list, new, status, promote,
status, strict lint, cite add dry run then written, cite show — against its
own scratch KB and reports each turn. It found two real answers while being
written: a promoted draft's orphan counts under strict lint, and a dry run
writes nothing, so the record must be entered before it can be shown. The
second harness is a real client and stays manual: point any MCP client at
`bin/stemma-mcp --kb <root>` through its own configuration file, confirm the
ten tools are offered, and run one call per family. What to look for is the
absence of harness-specific assumptions: nothing in the protocol exchange
names a client, and the payloads are the CLI's own.

Parity and integration tests. Assert that each tool returns the same payload as
the corresponding CLI command under `--json` on one corpus, and that both surfaces
agree on validation failures. Exercise the server from at least two harnesses to
confirm the absence of harness-specific assumptions; that check stays out of CI.

## Task 7:

Status: Done

Documentation. Describe the server where it can be generated rather than written:
the tool reference is emitted from the schema set Task 2 produces, so a schema and
its documentation cannot drift. Cover the curated subset and the commands
deliberately left on the CLI, the stdio invocation and how a harness is configured
to run it, and the payload and error contracts that match the CLI's. Keep the
harness instructions to the standard configuration file each client reads, with no
harness-specific extension. Add a gate that regenerates the reference and fails
when it is stale, the way the skill suite's references are handled.

Done 2026-10-08. The renderer moved out of the generator and into
internal/mcp/doc.go, so `BuildDoc`, `JSONBytes`, and `ReferenceBytes` are one
thing the generator writes and the tests can render; the reference is rendered
through a round trip of the schema set's own bytes, so it describes the
artifact, not the registry's intentions. `make mcp-docs` (renamed from
`mcp-schema`, which now writes both files) runs before the tests in `check`,
and TestGeneratedDocsAreFresh holds both files to an in-memory rerender,
skipping on a tree that has never generated them — proved to bite by editing
the artifact and watching the suite fail. The committed prose is README's MCP
section (what the server is, the standard `mcpServers` configuration shape,
the `_meta` root), AGENTS.md's build line, workflow, target list, and
generated-files list, and the reference itself carries the curated subset,
the envelope and its outcomes, and the omission table. The main plan's Task 9
consumes the JSON artifact for the project-level documentation, as the
dependencies note planned.

Documentation. Describe the server where it can be generated rather than written:
the tool reference is emitted from the schema set Task 2 produces, so a schema and
its documentation cannot drift. Cover the curated subset and the commands
deliberately left on the CLI, the stdio invocation and how a harness is configured
to run it, and the payload and error contracts that match the CLI's. Keep the
harness instructions to the standard configuration file each client reads, with no
harness-specific extension. Add a gate that regenerates the reference and fails
when it is stale, the way the skill suite's references are handled.

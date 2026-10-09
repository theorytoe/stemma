---
title: The MCP server
type: concept
aliases:
  - mcp
tags: [mcp, cli]
---
Stemma speaks to harnesses as well as to people. `stemma-mcp` serves a
knowledge base over the Model Context Protocol [@mcp2026]: a stateless stdio
server with no port, no daemon and no session — the same one invocation, one
KB shape the rest of the tool holds to.

The surface is a curated subset of [[The command surface]], not a mirror. Ten
tools: `status`, `list`, `search`, `show`, `graph` and `cite show` to read;
`new`, `promote` and `cite add` to write; and `lint` as the guard. Every
result is the matching verb's `--json` envelope — the same `command`, `ok`,
`data`, `findings` and `error` a script would parse — so an agent learns one
result shape, and the exit codes arrive inside it as `ok`. The commands
without a tool are named, with their reasons, in the generated reference.

A harness points at the binary through its own configuration file, one
`mcpServers` entry per server; the invocation and that shape are in the
README. A call may name the KB it wants in `_meta` as `stemma/kb`, and
`--kb` pins one at launch; with neither, the KB is found the way
[[Invoking the tool]] describes — `STEMMA_KB`, then a walk up from the
working directory.

The tool reference is generated, not written: `make mcp-docs` renders it from
the tool registry into `docs/mcp-tools.md`, and the tests fail when it is
stale. That is the same commitment [[Design principles]] makes about
harness-specific extensions — there is nothing hand-maintained for a harness
to drift from.

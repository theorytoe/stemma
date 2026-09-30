---
title: Design principles
type: principle
aliases:
  - philosophy
tags: [design]
---
A decision in this tool is made once and then inherited. When a choice is
unclear, a small set of commitments decides it, and these are those commitments.

**Machinery is additive, never prerequisite.** A knowledge base (KB) is a
directory of text. The tool works on one with no setup at all: no index, no
server, no database, no repository. The Tier-1 index, the rendered site, the
extraction shim and git history are each optional and each degrade to something
simpler. Nothing has to be started for a page to be read.

**One core, thin surfaces.** Parsing, link and citation resolution, indexing and
the invariants live in one place, and a surface is a thin layer over it rather
than a second implementation. A rule is not reimplemented per surface, because
two implementations of one rule are two rules waiting to disagree — which is why
the renderer and lint read the same scan. The surfaces are standard ones — the
CLI, a curated MCP tool set, open-standard skills — with no harness-specific
extension anywhere.

**The tool owns invariants; the author owns judgement.** The tool resolves,
checks, rewrites links and reports. It never writes prose, and an agent working
through it never computes a path, a citation key or an index by hand — it asks.
Anything with a right answer belongs to the tool; anything that is a choice is
yours.

**Reasoning lives in the document.** Git records what changed and when, and git
is optional. A page carries the reason for its own state in its own text, which
is why a retired page is kept rather than deleted. See [[The old sharing model]].

**Nothing is quadratic.** Resolution is constant-time per link and indexing is
incremental, because scale is unbounded in principle even when a KB is small
today.

**Sharing is not a subsystem.** A KB distributes as itself, by `git clone` or
`tar`. An extract is a generated derivative rather than a new artifact with its
own rules.

**Directories carry no meaning.** No directory confers scope, partitioning or
link behaviour. See [[Structure]].

**Lenient by default, strict on request.** Almost everything wrong is a warning
until `--strict` makes it an error, so ordinary work is not blocked by writing
that is not finished. The one refusal is a link that cannot resolve to exactly
one page, because there is no honest answer to guess at. See [[The format]].

**No-JS is a functional requirement.** Every web surface reads with JavaScript
off, and a script is polish. See [[Serving the site]].

**Never destroy what you do not understand.** The promise the whole tool rests
on: an unknown field survives every write. See [[The format]].

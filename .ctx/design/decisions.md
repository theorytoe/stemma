# Stemma — Decision Register

Distilled state of the design. Source transcript: [`qa-session.md`](qa-session.md).

**Legend** — All decisions below are settled by the author. Nothing is provisional.

- Name: **`stemma`**, accepted despite the existing `stemma-sh/stemma` collision (see naming log).
- Manifest: `stemma.toml`. Binary: `stemma`.

---

## Design principles

| #   | Principle                                                                                                                      | Origin      |
| --- | ------------------------------------------------------------------------------------------------------------------------------ | ----------- |
| P1  | **Graceful degradation.** Simple tooling works on a small KB with no setup; heavier machinery is additive, never prerequisite. | Q2 addendum |
| P2  | **Harness- and agent-agnostic.** No harness-specific extensions or config. Portable standard interfaces only.                  | Q4, Q5(c)   |
| P3  | **Layered core.** One core library owns parsing, linking, indexing, and invariants. Surfaces are thin.                         | Q6          |
| P4  | **Never destroy what you don't understand.** Tools preserve unknown fields verbatim.                                           | Q20         |
| P5  | **Git is optional.** The tool works in any directory; history features activate when a repo is present.                        | Q9          |
| P6  | **Reasoning lives in the document.** Git records what and when; the document records why.                                      | Q9          |
| P7  | **Scale headroom.** Nothing O(n²) in link resolution; incremental indexing must be possible.                                   | Q7          |
| P8  | **Sharing is not a subsystem.** Distribution is `git clone` or `tar`. Every shared artifact is a generated derivative.         | Q19         |
| P9  | **No-JS is a functional requirement.** Every web surface must work with JavaScript disabled. JS is polish only.                | Q25         |
| P10 | **Directories are not semantics.** No directory confers scope, partitioning, or link-resolution behaviour.                     | Q35         |

---

## Settled decisions

### Project scope

| ID  | Decision                                                                                                                                     | Rationale |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D1  | The repo contains **tooling for authoring and maintaining wikis**. A small example wiki lives in-repo. The author's real KB lives elsewhere. | Q1        |
| D2  | The agent works **in tandem with a user**. Authoring/ingest, maintenance, **and** retrieval all matter.                                      | Q2        |
| D3  | Audience is the author first, but anything crossing to another person must be self-contained.                                                | Q3(b)     |
| D4  | **Retrieval is first-class**, expected to work well at scale.                                                                                | Q2        |
| D5  | Scale is **unbounded in principle**.                                                                                                         | Q7        |
| D54 | The in-repo example wiki **is the project's own documentation**, built with the tool itself.                                                 | Q39       |
| D53 | **Importing existing wikis is out of scope** — not a v1 feature and not a near-term one.                                                     | Q38       |

### Technology

| ID  | Decision                                                                                   | Rationale  |
| --- | ------------------------------------------------------------------------------------------ | ---------- |
| D6  | **Go is primary.** Python where the document ecosystem demands it. JS only for web polish. | Q4         |
| D7  | **No harness-specific extensions.**                                                        | Q4, Q5     |
| D8  | **Native tools surface via MCP.** CLI is the universal fallback.                           | Q4, Q5, Q8 |
| D9  | **No `package.json`.** Install via git or local path. No npm, no bundler.                  | Q8(a)      |
| D10 | **One core library** owns format, links, index, invariants; CLI and MCP are thin surfaces. | Q6         |
| D11 | Skills conform to the **open Agent Skills standard**.                                      | Q5(c)      |

### Format

| ID  | Decision                                                                                                                            | Rationale |
| --- | ----------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D12 | **Git is optional.** Documents record the reasoning for meaning changes in their own text.                                          | Q9        |
| D13 | Tool-free raw readability is **not required**; preview/export is the human interface.                                               | Q10       |
| D14 | Frontmatter mandatory set is **small**. `type` is **closed but extensible by config**.                                              | Q11       |
| D15 | **Sources are cited, not compiled.** Pages are the origin. Original research is first-class.                                        | Q12       |
| D16 | A **robust bibliography subsystem** tracks documents as cited.                                                                      | Q12       |
| D17 | The KB is **contiguous — no topic boundaries, no sub-wikis**.                                                                       | Q13       |
| D18 | The format is **documented but not strictly versioned or gated**.                                                                   | Q14       |
| D19 | Links are **`[[wikilinks]]`**, resolved by title/slug/alias, exported to portable markdown. Ambiguous titles are a hard lint error. | Q15(b)    |
| D20 | Citations use **pandoc-style `[@key]`**.                                                                                            | Q16(b)    |
| D46 | Default types are **`topic`, `concept`, `note`**; `stemma.toml` **extends** the set.                                                | Q34       |
| D52 | The name is **`stemma`**, accepting an existing name collision.                                                                     | Q32, Q37  |
| D63 | Pages may carry free-form **`tags`**, compared by normalised form. Tags label; they never name, and never resolve a link.           | review    |

### Layout

| ID  | Decision                                                                                                                                                                                                                    | Rationale |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D48 | A single **`pages/`** directory holds all authored pages, with **author-chosen structure inside it**. The tool imposes no layout.                                                                                           | Q30       |
| D50 | Directories inside `pages/` are **organisational only** — no scoping, no per-directory index or export, no effect on link resolution. **No path-qualified links**; identity is the title, collisions resolved by `aliases`. | Q35       |
| D49 | `inbox/` sits in the **KB root but outside `pages/`**. Excluded from the knowledge proper: not indexed, not searched, not exported, only leniently validated.                                                               | Q24, Q31  |

### Citations and bibliography

| ID  | Decision                                                                                                                                                                    | Rationale |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D21 | Bibliography source of truth is **BibTeX**.                                                                                                                                 | Q17       |
| D22 | "Robust bibliography" = reverse citation lookup, orphan detection, stable keys with retrieval date and content hash, duplicate detection, validated BibTeX/CSL-JSON export. | Q22       |
| D34 | **Single `.bib` by default, per-source `bibliography/` directory for large KBs** — both first-class.                                                                        | Q22       |
| D35 | **Small built-in formatter**, a few common styles. **No CSL conformance** — full CSL implies `citeproc-js`, ruled out by D9.                                                | Q22       |
| D36 | Sources **do** get metadata pages, auto-generated from the BibTeX entry.                                                                                                    | Q22       |
| D45 | Source pages are **virtual**: materialised only at export/build time, never committed. Resolvable by citation key. Excluded from orphan and title-collision checks.         | Q28       |

### Ingest

| ID  | Decision                                                                                                                                              | Rationale |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D37 | **Deterministic capture, agentic synthesis.** Tools resolve identifiers and extract text; they never write prose. The agent never hand-writes BibTeX. | Q23       |
| D43 | Identifiers: **DOI, arXiv ID, URL, local PDF, ISBN**.                                                                                                 | Q23       |
| D38 | Source text is **not vendored by default** — pointer, content hash, retrieval date; opt-in snapshots.                                                 | Q23       |

### Tooling tiers, leniency, exports

| ID  | Decision                                                                                                                                                              | Rationale |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D23 | **Two tiers with transparent fallback.** Tier 0 computes on demand with no setup. Tier 1 is an opt-in gitignored index (SQLite FTS5). Nothing generated is committed. | Q18       |
| D24 | Use **KB** (corpus) and **KB root** (directory). Everything shared is a generated **export**.                                                                         | Q19       |
| D25 | **No dedicated sharing tooling.** Distribution is `git clone` or `tar`.                                                                                               | Q19       |
| D26 | **Lenient by default, `--strict` for CI.** No behaviour-changing version field; breaking changes ship as migrations.                                                  | Q20       |
| D27 | **Scoped extraction** exports a root document plus the documents it links to.                                                                                         | Q19       |
| D28 | **Context packets** are deferred, not dropped.                                                                                                                        | Q19       |
| D29 | Extract depth is **configurable**, default one hop, `--depth all` for closure.                                                                                        | Q21       |
| D30 | Extract outputs **a KB root in the same format**.                                                                                                                     | Q21       |
| D31 | **Citations are extracted and materialised into the exported tree**, so references never dangle.                                                                      | Q21       |
| D44 | Out-of-scope links are **pruned** from the extract.                                                                                                                   | Q21       |
| D51 | Pruned links **retain their anchor text** as plain text. A summary is reported by default; a flag emits the full list.                                                | Q36       |
| D33 | A **JSON/JSONL machine-readable dump** of pages, links, and citations is included.                                                                                    | Q21       |

### Platform

| ID  | Decision                                                                                                               | Rationale |
| --- | ---------------------------------------------------------------------------------------------------------------------- | --------- |
| D40 | One renderer, invoked as **`serve`** and **`build`**.                                                                  | Q25       |
| D41 | **One KB per invocation.** Discovery: explicit path → environment variable → walk-up from cwd. No registry, no daemon. | Q26       |

---

### Agent interface

| ID  | Decision                                                                                                                                                                                                                                                   | Rationale |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D57 | Every command supports **`--json`**. Exit codes: `0` clean, `1` validation findings, `2` operational error.                                                                                                                                                | Q42       |
| D58 | The MCP surface is a **curated subset** (~10 tools), not a full CLI mirror. Full surface stays on the CLI.                                                                                                                                                 | Q42       |
| D59 | `stemma.toml` is **optional-with-defaults** and is the discovery marker. Unknown keys preserved. Holds title, type extensions, citation style, ignore paths, default type. `citation_style` selects from built-in formatters only — **not CSL style IDs**. | Q43       |
| D60 | **Tools own anything with an invariant; the agent owns anything requiring judgement.** The corollary written into the skills: **the agent never computes a path, citation key, or index by hand** — it asks the tool.                                      | Q44       |

---

### Page types and lifecycle

| ID  | Decision                                                                                                                                     | Rationale |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D55 | `source` and `index` are **tool-owned reserved types**, not assignable by authors. Unknown types warn by default and error under `--strict`. | Q40       |
| D47 | `status` is `active` (default) or `archived`. `draft` is replaced by inbox membership. `archive` records a reason; the tool never deletes.   | Q40       |

### Command surface and skill suite

| ID  | Decision                                                                                                                                               | Rationale |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | --------- |
| D56 | Command surface: about twenty top-level verbs plus the `cite` and `export` families, spanning setup, authoring, sources, graph, retrieval, and output. | Q41       |
| D61 | Skill suite: **one umbrella skill plus five workflow skills** — research, author, maintain, query, publish.                                            | Q45       |
| D62 | Skills give the **CLI form** as canonical and note equivalent MCP tools. The umbrella carries the format reference and the query-first discipline.     | Q46       |

---

## Open questions at close

**None.** Every branch of the design tree is visited. The items below are not decisions but facts to
be discovered during implementation, recorded so they are not mistaken for settled ones.

| #  | Unknown                                                                                           | Becomes known by                                                                 |
| --- | ------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| U1 | The page count at which Tier 0 stops being fast enough.                                           | Benchmarking the on-demand resolver.                                             |
| U2 | Whether global title uniqueness becomes a real nuisance given arbitrary `pages/` structure (D50). | Real use. Path-qualified links are the escape hatch, deliberately not taken now. |
| U3 | Which citation styles the built-in formatter should cover.                                        | Examining the author's actual sources.                                           |
| U4 | Whether `env` and `fetch` earn their place in the surface.                                     | Use.                                                                             |
| U5 | Whether `rename` rewriting inbound links is reliable enough to justify D19.                       | Building it. This is the load-bearing assumption of the whole link design.       |
| U6 | `PATH` shadowing or confusion from the accepted `stemma` name collision.                          | Installing both tools.                                                           |

---

## Resolved during implementation

### Tags — a labelling axis, not a naming one. `D63` added.

**Decision.** A page may carry `tags`, a list of free-form labels. There is no
vocabulary of tags and no manifest key that declares one. Tags are compared by
their normalised form, so `Machine-Learning`, `machine learning` and
`machine_learning` are one tag; the spelling the author wrote is kept. Tags never
take part in link resolution: a page is identified by its title and aliases
alone, and a link never resolves to a tag.

**Why.** `type` is a closed, manifest-extensible classification with exactly one
value per page. It answers "what kind of page is this", and it cannot answer
"what is this page about" across types, which is what a tag filter, a per-tag
index and a later `search --tag` need. The alternative considered was a
manifest-declared tag vocabulary mirroring `types`, and it was rejected: a tag is
a judgement and the author owns judgement (`D60`), so a second closed set is
maintenance the tool would be imposing rather than a rule the format needs.

**Provenance.** The cli-surface delegate assumed a tag filter; `FORMAT.md` had
settled that there is no `tags` field. On review the tool author chose to add the
field rather than drop the filter. This section records that the settled line in
`FORMAT.md` was deliberately reopened, and `FORMAT.md` now documents the field.

### `description` and `[export] default_depth` manifest keys

The cli-surface delegate defined two keys `FORMAT.md` did not list. `description`
is free text that nothing but a person reads. `[export] default_depth` is the
default hop count for scoped extraction (`D29`), which the render/export delegate
reads. Both are parsed with defaults, and both are now in `FORMAT.md`.

The manifest is still never re-emitted: it keeps its raw bytes, and unknown keys
survive because the file is not rewritten. "Preserve unknown keys verbatim on
write" is therefore a property held by never writing, not by a writer that knows
how to splice. If a command ever needs to change a manifest key, that command
adds the writer and the tests it needs.

### U5 — rewriting inbound links. `D19` stands.

**Verdict: rewriting inbound links is reliable enough to justify title-based links.** No
fallback to path-qualified links is needed, and no link-rewriting pass belongs on every
operation.

The reason it holds is not that the string matching is careful. It is that the set of links
to rewrite is *defined by resolution*: a link is rewritten when it currently resolves to the
page in question, by a name derived from that page's title. Everything hard follows from that
definition rather than from a rule about text. A link by an alias resolves too, but through a
name the rename does not change, so it is excluded by construction. A link whose target was
ambiguous never resolved to this page, so rewriting it is excluded for the same reason. There
is no list of special cases to keep in step with the resolver, because there is no second
notion of what a link means.

The cost is small and paid once. Measured on a KB of 1000 referrers and 2000 links, a rename
rewrites all of them in **92 ms**, leaving the KB clean under `--strict` on both sides. The
command also checks itself: it collects the links that resolved to nothing before the rename,
reloads afterwards, and fails if that set grew, so a rewrite that broke a link is an error
rather than a silent corruption.

The pathological cases the delegate named are all covered by tests, not by argument: renaming
to a title another page holds or answers to as an alias is refused; a page with no inbound
links is the trivial case; links by alias are left alone; a link whose target was ambiguous is
left alone and reported, because such a link did not mean this page and must not be quietly
retargeted at it; and an interrupted run resumes. The last of these works because the steps
are ordered links-first and each step is skipped when already done, so the same command
finishes the job instead of needing hand repair. Resumption is verified from both points a
run can stop.

**Where the guarantee stops.** These are bounded and known, which is the difference between a
cost and a defect:

- Content outside the KB as the format defines it — inbox drafts, and anything a manifest
  ignores — is not read, so its links are not rewritten. Such a link is not lost and not
  silent: promoting the draft makes it a page, and lint reports the stale link at once. The
  limit is a delay, not a corruption.
- A link inside an *indented* code block is treated as prose, by lint and by rename alike,
  because the scanner decides code by fences and inline spans. The two agree, which is what
  matters for consistency; the disagreement is with a markdown renderer, and it is the known
  approximation recorded in the scanner's own tests.
- Repairing a collision needs a path rather than a name, since while two pages claim one name
  no name identifies either. Paths are accepted wherever names are for exactly this reason.

**Why the alternatives were not taken.** Path-qualified links do not avoid the rewrite, they
relocate it: every link would break on `move` instead of on `rename`, and moving files is far
more frequent than retitling. Move would then need the same machinery this delegate built, so
the cost is not saved, only moved — and it is moved onto the operation that happens
constantly, while giving up the property that reorganising `pages/` is free. A rewriting pass
on every operation is worse still: it pays this cost on every command rather than once, for
the same result.

## Naming collision log

**Chosen:** `stemma` — accepted despite collision. `stemma-sh/stemma` exists (CLI `stemma` on crates.io, plus an agent-facing MCP server). Accepted because D9 means git/local install, so the only costs are `PATH` shadowing and search noise.

**Taken and rejected before the decision to ignore collisions:**

| Name       | Collision                                                                                                        |
| ---------- | ---------------------------------------------------------------------------------------------------------------- |
| `weft`     | Six GitHub projects, several AI-agent CLIs                                                                       |
| `basalt`   | 1.2k-star TUI for Obsidian vaults — directly adjacent                                                            |
| `nomi`     | Multiple AI agent/context-engine projects                                                                        |
| `pinax`    | Long-standing Django starter framework                                                                           |
| `lemma`    | Rust CLI/language plus lemma.work agent platform                                                                 |
| `siglum`   | `@siglum` WebAssembly LaTeX compiler                                                                             |
| `colophon` | Includes a near-identical agent + citations knowledge-graph writing tool                                         |
| `koine`    | DSL compiler and a Claude Code HTTP gateway                                                                      |
| `variorum` | LLNL power/performance library; also an AI artifact editor                                                       |
| `urtext`   | An entire ecosystem in this exact domain: plaintext markup + Python library for writing and knowledge management |
| `apograph` | PyPI package (HTML→PPTX) and a LaTeX template collection                                                         |

**Rejected on taste:** `memex`, `codex`, `athenaeum`, `compendium`, `grimoire`.

**Observation:** every rare scholarly word in the manuscript/textual-criticism register is already claimed, because projects in this domain converge on the same vocabulary. Future naming should either accept collisions (as decided) or coin.

---

## Explicitly excluded

- `llm-wiki-okf` — author states its end goals do not align. Not a design input.
- `firwiki` — an older, incomplete attempt. Not a useful design input.

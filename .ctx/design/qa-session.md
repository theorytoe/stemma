# Agentic Wiki — Design Interview Transcript

Chronological record of a question-and-answer design session.

- Status of the session: **in progress** — round 5 is outstanding.
- Distilled outcome of this transcript: see [`decisions.md`](decisions.md).
- Question numbering is stable and never reused across rounds.

---

## Environment facts

Established before round 1, by inspection.

| Tool                            | Version / status                                                                        |
| ------------------------------- | --------------------------------------------------------------------------------------- |
| Go                              | 1.27.0                                                                                  |
| Rust                            | 1.97.1                                                                                  |
| Python                          | 3.14.7                                                                                  |
| Python pip                      | 26.2.1, system pip is PEP-668 `EXTERNALLY-MANAGED`                                      |
| `uv`, `pipx`                    | not installed                                                                           |
| Python libs present system-wide | `yaml`, `markdown`, `mistune`, `jinja2`, `httpx`, `requests`, `bs4`, `pypdf`, `pymupdf` |
| Also present                    | `sqlite3`, Deno, Node, Ruby, Lua, LuaJIT, Perl, PHP, `jq`, `rg`, `fd`, `pytest`, `tmux` |
| Pi                              | 0.83.0                                                                                  |

Delivery mechanics relevant to the project:

- Pi packages install from npm, git, or a local path, and are discovered via a `package.json` `pi`
  manifest **or** conventional directories (`skills/`, `extensions/`, `prompts/`, `themes/`).
- Pi native tools (extensions) load only `.ts`/`.js`, so they are incompatible with a JS-free,
  harness-agnostic project.
- MCP is available via the separately installed `pi-mcp-adapter`, and MCP servers may be written
  in any language. This is the harness-agnostic native-tool path.

Prior art excluded by explicit instruction: `llm-wiki-okf` and `firwiki`. Neither is a design input.

---

## Round 1 — root questions

**Q1 — What is the artifact?**
Options: (a) repo is the toolkit, KB lives elsewhere; (b) repo is a KB; (c) both.
Recommendation: **(a)**, plus a small example wiki in-repo as docs and integration fixture.

**Q2 — Where does the LLM add value?**
Candidates: (1) authoring/ingest, (2) maintenance, (3) query/retrieval.
Recommendation: 1 and 2 are differentiators; 3 is table stakes.

**Q3 — Who is this for?**
(a) personal only; (b) personal, but output must be handable to a stranger; (c) general-purpose public tool.
Recommendation: **(b)**.

**Q4 — Primary runtime.**
Recommendation: TypeScript on Node.

**Q5 — Which harnesses must the skills work in?**
(a) Pi only; (b) Pi-first, usable from any agent and a plain shell; (c) harness-agnostic by design.
Recommendation: **(b)**.

**Q6 — Layered core, or a pile of scripts?**
Recommendation: **(b)** one core library with thin surfaces.

**Q7 — Scale the design must survive.**
Recommendation: hundreds to a few thousand docs, files plus regenerable index, no database.

### Answers received at this point

Only Q4 was answered:

> Tools should avoid excessive use of Node/JS/TS — they are not languages the author writes in,
> and they are prone to various issues.

Consequence: the Q4 recommendation was withdrawn and the question re-opened in round 2.

---

## Round 2 — re-asked root questions plus language

**Q1, Q2, Q3, Q5, Q6, Q7** re-asked compactly with the same recommendations.

**Q4 (revised) — Core language.**
Candidates: Python, Go, Rust. Recommendation was Python 3.14, on the grounds that the document
ecosystem is Python's and the machine already has the required libraries installed.

**Q8 — How strict is "no JS/TS"?**
(a) zero JS/TS artifacts anywhere, install by git/local path, native tools via MCP, `package.json`
never exists; (b) `package.json` tolerated as a discovery manifest only; (c) occasional TS shims allowed.
Recommendation: **(a)**.

### Answers received

**Q1** — The repo contains the tooling for authoring and maintaining wikis. A small example wiki in
the repo would be useful.

**Q2** — The agent works in tandem with a user to create KB content, whether from external sources or
from original research. The LLM is also useful for information retrieval, especially in large KBs.
Therefore **all three** value centers matter.

*Follow-up addendum:* simpler tooling must work for small KBs, while tools must also exist to
facilitate larger KBs when needed.

**Q3** — (b) sums up the important points wanted for shareability.

**Q4** — Keep the toolset harness-agnostic, so harness-specific extensions are a no-go. Primary
amount of code should be **Go**, **Python as needed**, and **JS sparingly** (probably only for web
tools that require interactivity or fancier features).

**Q5** — (c) is correct: agent- and harness-agnostic.

**Q6** — (b).

**Q7** — Not sure how large the scale could be, but it is in the toolset's interest to compensate for
the possibility that larger KBs may be authored. Tooling that accounts for larger KBs would be a
good addition.

**Q8** — Stick with (a); loops back with Q4.

### Consequences

- Retrieval is upgraded from "table stakes" to a first-class deliverable.
- Grep-and-regenerate is ruled out as the indexing strategy; an incremental index is required.
- Native tool surface is an **MCP server in Go**. No `package.json` anywhere.
- JS is permitted only for interactive web surfaces, and this sits in tension with Q8(a) until the
  preview surface is specified.

---

## Round 3 — the knowledge base format

**Q9 — Is a KB a git repository by design?**
Recommendation: git assumed but optional, plus per-page change notes for meaning changes.

**Q10 — Must the KB be fully usable with no tooling at all?**
Recommendation: yes, as a hard constraint.

**Q11 — Page anatomy and typing.**
Strawman frontmatter: `title`, `type`, `tags`, `status`, `updated`, `sources`.
Recommendation: mandatory set is `title` and `type` only; `type` closed but extensible by config;
links untyped.

**Q12 — Are sources compiled from, or cited?**
Recommendation: pages are the origin, sources are cited evidence; pointer-by-default provenance
with opt-in vendoring.

**Q13 — What is a bundle, and what happens to links when sharing part of one?**
Recommendation: bundle = directory with a manifest; explicit mount directory for nesting; export
computes link closure and declares external references rather than emitting broken links.

**Q14 — Does the project define a named, versioned format?**
Recommendation: yes, lightweight — a `format: 1` manifest field and a `FORMAT.md`.

### Answers received

**Q2 (addendum)** — Simpler tooling for smaller KBs; tools present to facilitate larger KBs when
needed. (Restated as the *graceful degradation* principle.)

**Q9** — Git should be optional, not required. Documents themselves should ideally track reasoning
for changes in their text.

**Q10** — This is **not** a hard requirement. Having tools to preview/export a wiki to a readable
format (for example a website with the wiki content) is the better long-term choice anyway.

**Q11** — Keep the mandatory set small. Type should be closed, but extensible by config.

**Q12** — Sources are cited in wiki documents. There should be a **robust bibliography system** for
the wiki, to keep track of documents as they are cited throughout the wiki.

**Q13** — Not sure "bundle" is the correct term. The knowledge base as a whole should be
**contiguous**; there should be **no topic boundaries**. This aspect should be brainstormed further;
the vision is not clear yet.

**Q14** — A lightweight format specification might be useful, but requiring strict compliance and
versioning is foreseen to become a footgun later.

### Consequences

- Q10's relaxation withdraws two earlier recommendations: non-standard link syntax is viable again,
  and generated-only navigation is acceptable.
- Q14 changes the recommendation from a versioned gate to a leniency/preservation policy.
- A new subsystem is created: the bibliography.
- The sharing model is explicitly unresolved and needs brainstorming.

---

## Round 4 — link syntax, bibliography, generated artifacts, sharing, leniency

**Status: ANSWERED.**

**Q15 — Link syntax.**
(a) standard markdown paths, root-relative; (b) `[[wikilinks]]` resolved by title/slug/alias;
(c) canonical markdown with `[[...]]` as input sugar.
Recommendation: **(b)** stored canonically, exported to portable markdown. Lint ambiguous titles as
hard errors; exporter emits standard relative links.

**Q16 — Citation syntax inside page bodies.**
(a) markdown footnotes; (b) pandoc-style `[@key]`; (c) wikilink-style `[[src:key]]`.
Recommendation: **(b)**.

**Q17 — Bibliography store: format, and what "robust" means.**
Candidates: BibTeX/BibLaTeX, CSL-JSON, custom YAML/TOML.
Constraint: CSL rendering normally means `citeproc-js`, which Q8(a) rules out.
Recommendation: **BibTeX** as source of truth, single file by default with a `bibliography/`
directory form for large KBs; **small built-in formatter**, no CSL conformance. "Robust" is defined
as: reverse citation lookup, orphan detection, stable keys with retrieval date and content hash,
duplicate-key dedup, and validated BibTeX/CSL-JSON export.

**Q18 — What is generated, when, and does anything require an index?**
Recommendation: **two tiers with transparent fallback and nothing generated committed.**
Tier 0 reads files directly and computes the graph on demand; Tier 1 is an opt-in `index` command
writing a gitignored cache (SQLite FTS5, backlinks, citation maps, incremental updates). Commands
use the index when fresh and silently fall back otherwise. The only committed entry point is a
hand-written root document.

**Q19 — Sharing model and vocabulary.**
Proposal: retire "bundle" for topic-cutting. Use **knowledge base (KB)** for the corpus and
**KB root** for the directory. Everything shared is a generated **export**. Five scenarios:
1. source handoff (git clone or tarball),
2. static publication (render to a standalone site),
3. scoped extract (generated link-closed or query-defined subset),
4. context packet (one page expanded with transitively referenced context),
5. machine-readable dump (JSON/JSONL for other agent/RAG systems).
Recommendation: prioritize 1, 2, 5 early; sequence 3 and 4 after the format is stable.

**Q20 — Leniency policy.**
Proposal: lenient by default, `--strict` for CI; unknown fields preserved verbatim and never
rewritten; unknown types warn; no format version field that changes tool behavior; breaking changes
ship as migration commands. Load-bearing rule: **never destroy what you don't understand.**
Recommendation: adopt as stated.

### Answers received

**Q15** — (b) seems like the best choice.

**Q16** — (b) is the best choice.

**Q17** — BibTeX.

**Q18** — The process outlined looks like what is wanted. Use the proposed design.

**Q19** — Scenarios 1 and 2 are not really something handled by a tool as such. Sharing the entire
KB is simple, whether in a git repo or archived into a tar, so tooling for those functions is
redundant. Scenario 3, scoped extraction, is a good choice: it would fundamentally export a
document and all its children — that is, documents the top-level document links to. Scenario 4
might be interesting down the line, but focusing on it now is not a priority. Scenario 5 was not
addressed.

**Q20** — Agreed, adopt as stated.

### Consequences

- **No sharing/bundling subsystem.** Whole-KB distribution is `git clone` or `tar`, which needs no
  tooling. The export family collapses to two things: the render/publication path (needed anyway for
  preview) and scoped extraction.
- Scoped extraction now has a stated definition: a root document plus its linked children.
- Context packets (scenario 4) are deferred, not dropped.
- Scene 5, the machine-readable dump, remains unanswered and is re-opened in round 5.
- Bibliography semantics beyond "the format is BibTeX" were not confirmed and are reopened in Q22.

---

## Round 5 — citations, extraction, ingest, preview, platform

**Status: ANSWERED.**

**Q21 — Export family: scoped extract semantics and the machine-readable dump.**
Depth (one hop vs closure vs configurable); output artifact form (mini KB root vs rendered site);
citation closure; handling of inbound and excluded links; and whether the JSON/JSONL dump of
scenario 5 is wanted.
Recommendation: configurable depth defaulting to 1, output as a KB root in the same format,
citation closure always included, excluded links declared in a manifest, and the dump included as
nearly-free leverage.

**Q22 — Bibliography completion.**
Confirm the "robust" semantics; the file layout for large KBs; whether any CSL rendering is
attempted; and whether bibliography entries also become pages in the KB.
Recommendation: confirm the semantics; single file plus a directory form; a small built-in
formatter and no CSL; no auto-generated source pages.

**Q23 — Ingest pipeline and the tool/judgment boundary.**
Which inputs are supported, and what the deterministic tools do versus what the agent writes.
Recommendation: deterministic capture, agentic synthesis — tools resolve identifiers into
bibliography entries and extract text, and never write prose; the agent never hand-writes BibTeX.

**Q24 — Capturing original research and in-tandem authoring.**
Recommendation: use `status: draft` in place rather than a separate inbox directory, excluded from
export and from search by default.

**Q25 — Preview surface, and the JS decision.**
Recommendation: one renderer invoked two ways, `serve` and `build`; progressive enhancement so the
site is fully readable with JS disabled; optional vanilla JS with no build step.

**Q26 — KB discovery, and single versus multiple KBs.**
Recommendation: explicit path, then environment variable, then walk-up discovery; named roots in
config; no global registry, no daemon.

**Q27 — Naming.**
Project, binary, manifest filename, and whether the format gets its own name.
Recommendation: one name for tool and format; decide now, since it appears in the manifest
filename and binary.

### Answers received

**Q21** — Mostly agree, with two changes: citations should be **extracted then added to the exported
KB tree**. Links not covered by the export should have **the links pruned**. Depth should be
configurable. Otherwise agrees with all proposals.

**Q22** — Agrees with all choices. Adds: mechanically there might be use in adding **metadata pages
for a source**, which could be **auto-generated from the BibTeX entry**.

**Q23** — Mostly agree, but **ISBN support** should be added.

**Q24** — Option **(b), the inbox**, is preferred. Drafted documents have a separate place compared
to live documents.

**Q25** — Any web components must be **usable without JS**. Its inclusion might prove useful to make
web-interface tools look good, so **be sparing** in its usage.

**Q26** — One KB per invocation. Discovery methods accepted as solid.

**Q27** — Not feeling any of the proposed names. Come up with more; they do not have to be themed to
the project's goals.

### Consequences

- D31 is confirmed with an explicit mechanism: citation entries are materialized into the exported
tree.
- Pruning replaces the earlier "declare in an extract manifest" proposal. Whether the anchor text
  survives pruning is reopened as Q33.
- Source metadata pages are now wanted, which contradicts the round-5 recommendation D36. This
  creates a live tension with D23 (nothing generated is committed) and is reopened as Q28.
- ISBN is added to the ingest input list.
- A separate inbox directory replaces the `status: draft` proposal, which reopens where a promoted
  draft is filed and therefore what the KB's directory layout actually is (Q30).
- No-JS moves from a preference to a functional requirement (P9).
- All five round-5 name candidates were rejected by the author. Collision checks then eliminated a
  further batch; see the collision log in [`decisions.md`](decisions.md).

---

## Round 6 — source pages, types, layout, inbox, naming

**Status: ANSWERED.**

**Q28 — Source metadata pages: where do they live and when do they exist?**
Generated-into-export only, generated-once-and-committed, or fully virtual; whether wikilinks
resolve to them; and which type they carry.
Recommendation: virtual pages derived from BibTeX, materialized only at export/build time, never
committed, resolvable by citation key, carrying a reserved `source` type, and excluded from orphan
checks.

**Q29 — Default type vocabulary, and the status field.**
Recommendation: `topic`, `concept`, `entity`, `note`, `guide`, plus reserved structural types
`source` and `index`; `status` limited to `active` and `archived`, with `draft` replaced by inbox
membership.

**Q30 — Physical layout of the KB root.**
Flat `pages/`, by-type directories, or author-chosen namespaces.
Recommendation: by type, noting that D19 makes links title-based and therefore layout is not
load-bearing; disambiguation is by alias rather than by path.

**Q31 — Inbox mechanics.**
Where the inbox sits, what it is excluded from, how strictly it is validated, and what promotion
does.
Recommendation: inside the KB root, excluded from index/search/export by default, leniently
validated, and `promote` validates then moves the draft into its type directory.

**Q32 — Naming.**
Verification results so far: `weft`, `basalt`, `nomi`, `pinax`, `lemma`, `siglum`, `colophon`, and
`koine` are all taken. `lexeme`, `sememe`, and `sestina` appear clear; `stemma` is unverified.

**Q33 — Pruned-link semantics.**
Recommendation: the anchor text is retained as plain text; pruning is reported on stderr at extract
time rather than embedded in the output.

### Answers received

**Q28** — Virtual.

**Q29** — Pushback: did we not already agree that types would be configurable, minus the types owned
by the tool? (Correct — D14 already says closed but extensible by config.)

**Q30** — A `pages/` directory with arbitrary structure inside it is the best approach.

**Q31** — The inbox is in the KB root but excluded from "the knowledge", and is kept outside of
`pages/`.

**Q32** — `stemma` is liked. *(Subsequently found to be taken; see below.)*

**Q33** — Pruned links should be reported, with an option to report what links were pruned.

### Consequences

- D45 settled: source pages are virtual.
- Q29 was a correction, not an answer. The question of whether config **extends** or **replaces** the
  default vocabulary is still unanswered, as is the fate of the `status` field. Reopened as Q34.
- D48 changed materially: arbitrary structure inside `pages/` replaces the recommended by-type
  layout. This collides with D17 unless directory structure is declared semantically inert, and it
  makes path-qualified links a live question. Reopened as Q35.
- D49 settled with an explicit detail: `inbox/` is outside `pages/`, not merely separate from it.
- Anchor-text retention was **not** answered — only the reporting behaviour. Reopened as Q36.
- `stemma` was verified and is taken: `stemma-sh/stemma` is a live project (v0.5.2, Aug 2026)
  publishing a `stemma` CLI (`stemma-cli` on crates.io) **and an agent-facing MCP server**
  (`stemma-mcp`). `variorum`, `urtext`, and `apograph` were also checked and are taken; `urtext`
  is taken inside this exact problem domain.

---

## Round 7 — types, layout semantics, pruning, naming strategy, import, example wiki

**Status: ANSWERED.**

**Q34 — Type vocabulary config, and the status field.**
Does config extend the shipped defaults or replace them? And does `status` survive the inbox, with
what values?
Recommendation: config extends; `source` and `index` are tool-owned and reserved; unknown types
warn by default and error under `--strict`; `status` is `active` or `archived` only, with `archive`
recording a reason and the tool never deleting.

**Q35 — What arbitrary structure inside `pages/` means.**
Are directories semantic or organisational? Are path-qualified wikilinks allowed?
Recommendation: organisational only — no scoping, no per-directory index or export, no effect on
resolution. No path-qualified links; identity is the title, collisions resolved by alias.

**Q36 — Pruned links.**
Is the anchor text retained? What is reported by default versus behind a flag?
Recommendation: anchor text retained as plain text; a summary by default; a flag for the full list.

**Q37 — Naming strategy.**
Every evocative single word checked so far is taken. The question is the uniqueness bar and the
flavour.
Recommendation: coined names, which cannot collide by construction; or accept a rare real word
with search noise.

**Q38 — Importing existing wikis.**
Obsidian vaults, plain markdown collections.
Recommendation: out of v1 scope, explicitly designed for, with Obsidian as the first target.

**Q39 — The in-repo example wiki.**
Toy fixture, the project's own documentation, or a real subject-matter showcase.
Recommendation: the project's own documentation, built with the tool itself, so the docs are a
live test of the format.

### Answers received

**Q32 (addendum) / Q37** — The author does not care that the name is already taken: **`stemma`** is
the name. Chosen as option (c).

**Q34** — Config extends, and defaults exist. Author suggests **`topic`, `concept`, `note`** as the
sensible defaults.

**Q35** — Stick with (b): directories are organisational only, no path-qualified links. Author
agrees this is the correct call.

**Q36** — Anchor text is retained.

**Q38** — Out of scope.

**Q39** — Author had (b) in mind already, since it doubles as documentation and a sample wiki.

### Consequences

- Naming closed: `stemma`, `stemma.toml`, binary `stemma`. The collision with `stemma-sh/stemma` is
  explicitly accepted because D9 means git/local install, so only `PATH` shadowing and search noise
  remain as costs.
- Default type list is trimmed from five to three: `topic`, `concept`, `note`. `entity` and `guide`
  drop out.
- `archive`/`status` behaviour, reserved tool-owned types, and unknown-type handling were not
  addressed. Reopened as Q40.
- Import is out of scope outright, not merely deferred.

---

## Round 8 — types plumbing, command surface, machine contract, manifest, agent boundary

**Status: ANSWERED (Q40 and Q41 outstanding).**

**Q40 — Reserved types and the status field.**
Are `source` and `index` tool-owned and non-assignable? Does `status` survive the inbox, with which
values?
Recommendation: `source` and `index` reserved; `status` is `active` or `archived` only, with `draft`
replaced by inbox membership; unknown types warn by default and error under `--strict`.

**Q41 — The command inventory.**
A concrete list of roughly twenty top-level verbs across setup, authoring, sources, graph, retrieval,
and output, plus two noun families (`cite`, `export`).
Recommendation: as listed, with `backlinks` folded into `graph --in` and `resolve` folded into
`show --path`.

**Q42 — The machine-facing contract.**
`--json` on every command; exit-code convention; and whether the MCP surface mirrors the CLI or is a
curated subset.
Recommendation: `--json` universally; exit `0` clean, `1` validation findings, `2` operational
error; MCP curated to about ten tools rather than a full mirror.

**Q43 — The `stemma.toml` schema.**
Which keys exist, and is the manifest required?
Recommendation: optional with defaults, and the discovery marker; unknown keys preserved; holds
title, type extensions, citation style, ignore paths, default type.

**Q44 — The agent/judgment boundary.**
Which workflows are deterministic tools, and which are the agent's prose responsibility?
Recommendation: tools own anything with an invariant; the agent owns anything requiring judgement;
the agent never computes a path, key, or index by hand.

### Answers received

**Q40** — *Not answered.*

**Q41** — *Not answered.*

**Q42** — Agreed: there should be JSON for agents.

**Q43** — Agreed.

**Q44** — Agreed with the proposal.

### Consequences

- D57, D58, D59, D60 settled.
- Q40 and Q41 stand unanswered and are re-asked in round 9. Q41 is the blocker for the skill
decomposition, so it is carried forward verbatim rather than assumed.

---

## Round 9 — types plumbing, command surface, skill decomposition

**Status: ANSWERED. Session closed — frontier empty.**

**Q40 (re-asked) — Reserved types and the status field.**
Are `source` and `index` tool-owned and non-assignable? Does `status` survive the inbox, with which
values?
Recommendation: `source` and `index` reserved; `status` is `active` or `archived` only, with `draft`
replaced by inbox membership; unknown types warn by default and error under `--strict`.

**Q41 (re-asked) — The command inventory.**
The concrete list of roughly twenty top-level verbs plus two noun families (`cite`, `export`),
spanning setup, authoring, sources, graph, retrieval, and output.
Recommendation: adopt as listed, with `backlinks` folded into `graph --in` and `resolve` folded into
`show --path`. Author is invited to cut anything unused.

**Q45 — Skill decomposition.**
How many skills, and what each covers.
Recommendation: one umbrella skill carrying the invariants and format reference, plus five workflow
skills: research, author, maintain, query, publish. Descriptions must be disjoint, because skill
selection is driven by description matching.

**Q46 — How skills invoke the tool, and what the umbrella contains.**
Recommendation: skills state the operation and give the CLI form as canonical, noting the equivalent
MCP tool; the umbrella carries the format reference and the query-first discipline.

### Answers received

**Q40 (addendum)** — Author concurs with the proposal.

**Q41** — This set of commands works well.

**Q45** — This looks good.

**Q46** — Yes, a good solution.

### Consequences

Design closed. D47, D55, D56, D61, D62 settled. No provisional decisions remain and no questions
are unanswered.

### Session closing note

The frontier is empty: every branch of the design tree was visited and settled by the author, and
nothing is left silently assumed. The remaining unknowns are recorded in
[`decisions.md`](decisions.md) under "Open questions at close" as facts to be discovered during
implementation — most importantly U5, whether `rename` can reliably rewrite inbound links, which is
the load-bearing assumption behind the `[[wikilink]]` decision (D19).

The most valuable thing this session produced was not the answers but the collisions and
reversals: Q4's runtime recommendation was withdrawn outright, the "bundle/share" pillar of the
original brief dissolved into `git clone` plus one extract command, the source-page decision
contradicted the generated-artifact policy and forced Q28, and the inbox decision exposed that the
directory layout had never been asked about at all.

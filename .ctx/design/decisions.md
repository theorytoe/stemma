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
| D8  | **Native tools surface via MCP** (planned, not yet built). CLI is the universal fallback.  | Q4, Q5, Q8 |
| D9  | **No `package.json`.** Install via git or local path. No npm, no bundler.                  | Q8(a)      |
| D10 | **One core library** owns format, links, index, invariants; CLI and MCP are thin surfaces. | Q6         |
| D11 | Skills conform to the **open Agent Skills standard**.                                      | Q5(c)      |

### Format

| ID  | Decision                                                                                                                                                                        | Rationale |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D12 | **Git is optional.** Documents record the reasoning for meaning changes in their own text.                                                                                      | Q9        |
| D13 | Tool-free raw readability is **not required**; preview/export is the human interface.                                                                                           | Q10       |
| D14 | Frontmatter mandatory set is **small**. `type` is **closed but extensible by config**.                                                                                          | Q11       |
| D15 | **Sources are cited, not compiled.** Pages are the origin. Original research is first-class.                                                                                    | Q12       |
| D16 | A **robust bibliography subsystem** tracks documents as cited.                                                                                                                  | Q12       |
| D17 | The KB is **contiguous — no topic boundaries, no sub-wikis**.                                                                                                                   | Q13       |
| D18 | The format is **documented but not strictly versioned or gated**.                                                                                                               | Q14       |
| D19 | Links are **`[[wikilinks]]`**, resolved by title/slug/alias, exported to portable markdown. Ambiguous titles are a hard lint error. The markdown export is deferred; see below. | Q15(b)    |
| D20 | Citations use **pandoc-style `[@key]`**.                                                                                                                                        | Q16(b)    |
| D46 | Default types are **`topic`, `concept`, `note`**; `stemma.toml` **extends** the set.                                                                                            | Q34       |
| D52 | The name is **`stemma`**, accepting an existing name collision.                                                                                                                 | Q32, Q37  |
| D63 | Pages may carry free-form **`tags`**, compared by normalised form. Tags label; they never name, and never resolve a link.                                                       | review    |

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
| D64 | Export is **`cite export --format bibtex|csl-json`**, whole bibliography by default, `--cited` to narrow it. BibTeX is **self-contained** (macros resolved); CSL-JSON maps known fields and passes the rest through. Output is checked by an independent reader. | Q22       |
| D65 | A source pointer is compared by **what it names, not how it is typed**, and there is **one normalisation rule per identifier in `kb`**, shared by duplicate detection and by resolution so the two cannot disagree. DOI folds case and drops the URL wrapper, query and fragment; ISBN keeps digits only; arXiv **keeps the version to fetch and drops it to identify**. Names are read by **one shared reader** for the same reason. | review    |

### Ingest

| ID  | Decision                                                                                                                                                                     | Rationale |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D37 | **Deterministic capture, agentic synthesis.** Tools resolve identifiers and extract text; they never write prose. The agent never hand-writes BibTeX.                        | Q23       |
| D43 | Identifiers: **DOI, arXiv ID, URL, local PDF, ISBN**.                                                                                                                        | Q23       |
| D38 | Source text is **not vendored by default** — pointer, content hash, retrieval date; opt-in snapshots.                                                                        | Q23       |
| D66 | The Python shim is a **tracked script embedded in the binary** and written into `<kb>/.stemma/shim/` by `init`. Go supplies the text limit; the shim prints one JSON object. | review    |

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
| D70 | The dump is **one JSON document**, not JSONL, carrying the **page bodies** as well as the metadata so it stands alone, with a **schema version of its own** and a shape documented in `FORMAT.md`. It is written to **`<kb>/.stemma/export.json`** by default, and `--out -` writes it to standard output. | review    |
| D71 | A **scoped extract** writes its root page as the **entry document**, **prunes** every construct it cannot keep into its anchor text, and **closes** references to the keys it still cites. The **entry document is exempt from the orphan check**, because nothing links to the page every other page is reachable from. | review    |
| D67 | The Tier-1 index is **one file, `<kb>/.stemma/index.sqlite`**, versioned by SQLite's `user_version`; any other version, or a file that is not a database, is deleted and rebuilt. Tokens are defined by **one Go tokenizer** shared by both tiers; FTS5 is an external-content index over a `tokens` column those tokens fill, and **BM25 ranking and snippets are Go code**. | review    |
| D68 | Retrieval is one **data-source interface** (`index.Source`) that both tiers implement, not an interface of queries. Search, graph traversal and the orphan and dead-end listings are single functions over it, so the tiers agree by construction. The interface includes **batch forms** (`Docs`, `LinksAll`, `BacklinksAll`), because a rule over the whole corpus must not become one query per page. `NewSource` prefers a fresh index and **silently falls back to the KB**, reporting the downgrade through one optional callback. | review    |

### Platform

| ID  | Decision                                                                                                               | Rationale |
| --- | ---------------------------------------------------------------------------------------------------------------------- | --------- |
| D40 | One renderer, invoked as **`serve`** and **`build`**.                                                                  | Q25       |
| D41 | **One KB per invocation.** Discovery: explicit path → environment variable → walk-up from cwd. No registry, no daemon. | Q26       |
| D69 | A resolved wikilink renders as an anchor showing the **page's own title**; an unresolved or ambiguous one renders as the written text inside a marked span; a citation links each key to that key's virtual source page. | review    |
| D72 | The site's home page is **generated** from what the KB holds — counts, browse by type and tag, the newest sources, and what needs attention — with the **entry document's body as its opening** when the KB has one. `pages/index.md` is **optional** and is not rendered as a page of its own. | review    |
| D73 | The site's type is **Work Sans**, falling back to **Roboto** and then the reader's system UI face. The fonts are **named, not shipped**: the stylesheet declares the stack, the repo carries no font files, and a built site stays self-contained (`D3`). The body's serif stack is gone — everything but code reads in this face. | review    |

---

### Agent interface

| ID  | Decision                                                                                                                                                                                                                                                   | Rationale |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| D57 | Every command supports **`--json`**. Exit codes: `0` clean, `1` validation findings, `2` operational error.                                                                                                                                                | Q42       |
| D58 | The MCP surface, when built, is a **curated subset** (~10 tools), not a full CLI mirror. Full surface stays on the CLI.                                                                                                                                    | Q42       |
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
| U1 | **Answered.** Tier 0 is comfortable to roughly 1,000 pages. The index saves about a third of a search at 1,000 and at 10,000 pages, so one build (19 ms / 0.19 s / 2.7 s) pays for itself after roughly twenty searches: it is worth it under repeated queries, not above a particular size. Measured by `make bench` over the whole `search` and `graph` commands, not candidate matching alone; the table is in `FORMAT.md`. | Benchmarking the on-demand resolver.                                             |
| U2 | Whether global title uniqueness becomes a real nuisance given arbitrary `pages/` structure (D50). | Real use. Path-qualified links are the escape hatch, deliberately not taken now. |
| U3 | **Answered.** Both `author-date` and `numeric` are kept, and **`numeric` is the default**: `kb.DefaultCitationStyle` and `citestyle.Parse("")` return it, `FORMAT.md` records it, and the example wiki no longer names a style so it exercises the default. CSL stays out of scope. | Examining the author's actual sources.                                           |
| U4 | **Answered.** Both earn their place. `fetch` is used by the research skill, and `env` is the diagnostic for a failed fetch — it reports python3, the extraction shim, its libraries, and whether sqlite is linked. Neither is required for the core to work. | Use.                                                                             |
| U5 | Whether `rename` rewriting inbound links is reliable enough to justify D19.                       | Building it. This is the load-bearing assumption of the whole link design.       |
| U6 | **Answered.** No confusion; the collision does not matter in practice. The tool keeps the name and the README does not mention it. | Installing both tools.                                                           |

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

### The extraction shim — delivery, the limit, and one PDF library. `D66` added.

**Decision.** The extraction script is a tracked file in the repository,
`internal/extract/extract.py`, embedded in the binary. `init` writes it into
`<kb>/.stemma/shim/extract.py` when the KB is created, and every extraction
compares that copy against the embedded bytes and rewrites it when they differ.
The text limit is an argument Go supplies, not a number written into the contract.

**Why.** The delegate allowed either embedding the script or shipping it as a file
found at run time, and the first answer was a repository file resolved beside the
executable. That was wrong for a reason worth recording: the binary is not
self-contained either way, because the interpreter and the reading libraries live
outside it, so embedding buys version-lock rather than independence. What it locks
is worth locking, because the two halves share a JSON contract that must not
drift. Writing the copy at `init` answers the objection that a file should not
appear the first time someone reads a document, and the byte comparison is what
makes a cleared `.stemma/`, an upgrade, and a KB received by clone one case rather
than three.

**Provenance.** The first draft of the contract made the 8 MiB limit a constant of
the document. The number was invented rather than required, and pinning it there
meant editing a design document to tune a policy, so the limit became an argument.
`probe` had no place in that draft's success shape either: it describes the
machine rather than a document, so its object carries `python` and `libs` in place
of `text`.

**Changed during implementation.** The delegate asked for pymupdf with a pypdf
fallback, and the fallback was built first. It was then dropped, because
implementing it showed what it costs: the two libraries do not report the same
things. The note about a page that looks two-column needs block geometry that only
pymupdf offers, so the tool's answers would have depended on which library a
machine happened to have. Requiring pymupdf makes the machine stop being a
variable. The bytes guard stayed, for the reason that does not depend on
installations at all: PyMuPDF also reads EPUB, XPS and plain text files, so without
the guard a document would be read by a pipeline this contract does not describe
and reported as a PDF, with EPUB supported by accident rather than by design.

### The Tier-1 index — one file, one version marker, one tokenizer. `D67` added.

**Decision.** The cache is a single SQLite file at `<kb>/.stemma/index.sqlite`. Its
schema version lives in SQLite's `user_version`; a file carrying any other value,
or one that is not a database at all, is deleted and rebuilt from the KB. Words
are defined by one Go tokenizer that both tiers call. FTS5 is an external-content
inverted index over a `tokens` column that tokenizer fills, kept in step with
`pages` by triggers, with a two-, three- and four-character prefix index; BM25
ranking and snippet extraction are Go code in `internal/index` as well.

**Why delete rather than migrate.** The cache is derived and nothing in it is a
source of truth, so the only state a version mismatch can lose is state that the
next build recomputes. Deleting the file is the one recovery that works whatever
is on disk — a wrong version, a half-created schema, a file that is not a database
— and it removes the need for a migration path that would itself have to be
tested. A partial schema at the *right* version is still rebuilt, because a header
is not evidence that the objects behind it exist.

**Why the tokenizer and the ranker are Go, not FTS5.** Tier 0 is defined as
reading files and computing in memory (`Q18`, `D23`), so it has no FTS5 to
tokenize or rank with. If Tier 1 used FTS5's tokenizer and Tier 0 a Go one, the
two would return different pages whenever the tokenizers disagreed about a word —
the exact failure the tier-parity test exists to catch. One Go tokenizer and one
Go BM25 scorer, called by both tiers, make the answers identical by construction;
FTS5 decides nothing about a match, it only finds candidate rows quickly. Accent
folding therefore lives in a small Latin fold table rather than in `unicode61`.
The fold is partial — the precomposed accents that occur in practice — because
Go's standard library has no normaliser and the pinned allowlist has no room for
`golang.org/x/text`. Snippets are Go code for the same reason: a contentless FTS
table has no text to quote, and one snippet function keeps both tiers alike.

**Changed during implementation.** The first version of this decision put the
tokenizer in FTS5 (`unicode61 remove_diacritics 2`, external content, FTS5
`bm25` and `snippet`). Working the parity requirement through showed that an
in-memory Tier 0 cannot reproduce FTS5's Unicode folding in Go, so a shared
engine was the only way to keep FTS5 authoritative — and building a database per
Tier-0 query contradicts Tier 0 being files plus in-memory. The tokenizer and
ranker moved into Go instead, and the text table became a contentless index over
Go's tokens. The schema version went from 1 to 2 with the change, and to 3 when
incremental refresh showed that a contentless table cannot cheaply delete one
document: the tokens moved into a `tokens` column on `pages` and the text table
became external-content over it, so the triggers remove a page's entry when the
page goes.

**Why `internal/index` and not `internal/kb`.** `D10` puts the index in the core
library rather than in a surface, and `internal/index` is that core library's
index half: the CLI calls it and writes no SQL of its own. It is a separate
package because it is the one part of the core that links `modernc.org/sqlite`;
keeping it apart leaves the page model, which every command imports, free of the
database and of a build tag. `internal/kb` stays the pure on-disk format.

**Provenance.** The delegate fixed the tiers, the storage engine and the fact
that the cache must be deletable, and left the file name and the tokenizer to
implementation. Both are recorded here because both are expensive to change once
an index exists on disk; the tokenizer's move into Go was the tool author's
choice, made after the parity cost of the first answer was shown.

### Retrieval — one source interface, two tiers. `D68` added.

**Decision.** Retrieval is one interface, `index.Source`, that both tiers
implement: each answers what the corpus holds — pages, tokenised documents,
candidate terms, names, links, backlinks, citations. Search and graph traversal
are functions written once against that interface, not methods on it, and so are
the orphan and dead-end listings. The interface carries the batch forms of the
text and link relations (`Docs`, `LinksAll`, `BacklinksAll`) for the same reason:
a rule over every page stays one query rather than one per page, which is what
keeps the single implementation affordable. `NewSource` returns the index when it
is fresh and the KB otherwise, and reports the downgrade through one optional
callback, never per query.

**Why a data interface, not a query interface.** An interface whose methods were
"search" and "graph" would leave each tier free to rank or traverse differently,
and the tier-parity requirement would be a promise two implementations have to
keep rather than a property of the code. Making the interface the data half means
there is one ranking implementation and one traversal implementation; the two
tiers supply different rows, never different answers. It is the same reasoning
that moved the tokenizer and ranker into Go (`D67`): a candidate the index finds
must be a candidate the KB scan would find.

**The fallback is not an error.** A missing, stale or unreadable index is a
reason to answer from the KB, not to fail, because every command must work with
no index (`D23`, `P1`). The downgrade is silent; a caller that wants a diagnostic
passes a callback and gets one line per invocation.

### Render and export — what a link and a citation become. `D69` added.

**Decision.** A resolved wikilink renders as an anchor to its page and shows that
page's title, whatever spelling of the name was written. A link that does not
resolve, or resolves to more than one page, renders as the text that was written
inside a span carrying a class (`stemma-unresolved`, `stemma-ambiguous`) rather
than as a link to nothing. A citation renders as the built-in formatter's label,
and every key the bibliography defines is a link to that key's virtual source
page; a key it does not define stays text and is reported as a finding.

**Why the title rather than the spelling.** `FORMAT.md` calls the slug "a form
that is forgiving to type", and `D13` makes the rendered site the human
interface. A forgiving spelling is for typing and the title is for reading; an
alias names the same page, so it reads as the title too.

**Why a marked span for a broken link.** An unresolved link is a warning by
default (`D26`), so dropping it would hide from a reader what lint only complains
about, and an anchor with no destination is worse than text that looks like text.
The class is the hook a stylesheet uses; the page stays readable with no
stylesheet at all.

**Why citations link per key.** `FORMAT.md` says a citation resolves to the
entry's virtual source page (`D36`, `D45`), so a group of keys is a group of
destinations. One anchor around a multi-key group would leave every key but one
unreachable from the body.

**Provenance.** Chosen with the tool author before the renderer core was written.
The format deliberately leaves the HTML shape to the presentation layer, so these
are the site's choices rather than the format's, and they are recorded the way the
index choices were: because they are expensive to change once pages are published.

### The JSON dump — one document, bodies included. `D70` added.

**Decision.** `stemma export json` writes one JSON document, by default to
`<kb>/.stemma/export.json`, holding the pages, every wikilink with how it
resolved, every citation with whether the bibliography defines it, and the
bibliography itself. Page bodies are in it. The document carries a `version` of
its own — `1` — every list is present even when empty, and the shape is
documented in `FORMAT.md`.

**Why one document and not JSONL.** `D33` left the form open. A KB is a graph,
and a graph is not a stream: the whole answer is one object, and a consumer that
wants records iterates an array. JSONL would make the record order and a per-line
discriminator part of the contract — surface to keep stable for no gain at KB
scale, where thousands of pages is a few megabytes. The records are the same
records, so a JSONL form can be added later without changing this one.

**Why the bodies.** `D33` names pages alongside links and citations, and the
purpose is consumption by other agent and RAG systems. A dump without the prose
sends the consumer back to the source tree, which is the one thing the dump
exists to avoid. The body is the markdown as it is on disk, so the dump is a view
of the KB rather than a second rendering of it.

**Why a version of its own.** `D26` refuses a behaviour-changing version field in
the KB format, and this is not that. It is the schema version of one generated
artifact, whose only reader is a program that has nothing else to tell it that
the shape moved.

**Why a file by default.** An artifact belongs in a file the way the built site
does, and it keeps the universal `--json` envelope carrying a summary rather than
the whole document re-encoded inside another JSON object. `--out -` writes the
document to standard output when a pipe was what was wanted.

**What is deliberately absent.** A page's fields are the ones the format defines;
a citation's group, prefix and narrative form are how it should be rendered
rather than what it is; and drafts are absent because `inbox/` is not part of the
knowledge.

**Provenance.** Put to the tool author with the alternatives before the package
existed, and answered: one document, bodies included, a file by default.

### The scoped extract — a KB, not a rendering. `D71` added.

**Decision.** `stemma export page PAGE` writes a KB root holding one page, the
pages it links to, and the sources they cite, in the same format as any other KB
(`D30`). The root page is written as the entry document, `pages/index.md`. A link
that resolves to a page inside the slice is left as written; a link out of the
slice becomes the title of the page it named, and a link that named nothing or
named more than one page becomes the text the author wrote. A citation group is
kept only when every key in it is defined, and otherwise becomes its own text
with the `@`s taken out. `bibliography.bib` holds exactly the keys the extract
still cites. The entry document is exempt from the orphan check. Depth is hops,
defaulting to one, with `--depth all` for the whole reachable set.

**Why the root becomes the entry document.** An extract is *of* a page, so that
page is the page a reader should arrive at; the format already gives the entry
document that meaning, and `homeURL` already treats it as the site's home. It
costs nothing, because a file name carries no meaning in this format — links
resolve by title and alias (`D19`, `U5`) — so the page keeps its name, its type
and every field, and only its path moves. A page that would land on the entry
document, meaning the source's own entry document when the slice holds it, moves
aside to `pages/index-2.md`.

**Why the entry document has to be exempt from the orphan check.** It is the
page every other page is reachable from, so nothing links to it by construction;
an orphan finding on it says nothing an author could act on. `init` gives the
entry document `type: index`, which is why the finding never appeared for a KB
the tool made itself, but that is an accident of what `init` chooses rather than
a property of being the entry document — and an extract's root is exactly the
case where the two come apart. Raising it was necessary rather than optional:
without it no extract could satisfy the criterion that an extract lints clean
under `--strict`.

**Why prune rather than refuse or carry.** Carrying a construct that cannot be
kept would leave the extract with the dangling link that `D31`'s closure exists
to prevent; refusing would make one unresolved link anywhere in the closure block
an otherwise useful export. Pruning keeps the extract's promise — it stands alone
and it is clean — whatever state the source is in. The text used is the anchor
text a reader of the source would have seen, which is `D69`'s rule applied one
layer out, and it is what `D44` and `D51` already ask for.

**Why the citation group is the unit.** `kb.Inline` calls a group "one
parenthetical ... read and rewritten as one unit", and that is the honest unit:
rebuilding `[@a; @b]` out of the keys that survived would be writing markdown the
author did not write. A group with one undefined key is therefore pruned whole,
and the report says so.

**Why the extract records what it wrote.** A second extraction into the same
directory would otherwise accumulate the pages of the first, and a stale page
becomes an orphan or a name collision — the extract would get *less* clean each
time it was refreshed. The record lives in the extract's own `.stemma/`, which is
where a KB keeps what it derives, and it is the only thing a write is allowed to
remove (`P4`). A directory that is neither empty nor an extract is refused
rather than written into.

**Provenance.** Put to the tool author as a contradiction rather than worked
around: an extract's root has no inbound links, so the criterion "an extract
lints clean under `--strict`" could not hold. Renaming the root and exempting the
entry document was chosen over generating a cover page, retyping the root as an
index, or softening the criterion; pruning and reporting was chosen over carrying
broken constructs or refusing to extract.

### The home page — generated, with the entry document as its opening. `D72` added.

**Decision.** `index.html` is a landing page the tool writes: the KB's title and
description, a line of counts, then blocks counted from what the KB holds —
browse by type and tag, the newest sources, and the things lint would like fixed
— and, when the KB has an entry document, that page's body as the opening. The
entry document is `pages/index.md`, `init` writes one, it is optional, and it is
not rendered as a page of its own: its address is the home page's, and it is the
same document.

**Why generated.** What a KB knows about itself is exactly the material a front
page wants, and all of it is a consequence of the content rather than of anybody's
choice: how many pages there are, which types and tags exist, which sources were
read most recently, and what lint is unhappy about. Wikipedia's Main Page is the
model and the warning at once — its blocks are generated from its content, and
its "featured article" and "did you know" are the two a person chose. This page
takes only the first kind.

**Why the entry document stays, and stays the opening.** It is the one part of
the page a program cannot write: a list of pages with a reason to read each is
judgement, and the example wiki's entry document is exactly that list. Keeping it
as the opening is also what makes the change free — that page's address is already
`index.html`, so the page an author wrote and the page a reader arrives at become
one document rather than two wanting one URL. No address changes, no links move,
and there is nothing to migrate.

**Why it is optional.** A KB with no entry document gets the generated page and
no opening, so deleting `pages/index.md` is a supported choice rather than a
broken KB. That is what makes it a page rather than a fixture: `init` writes one
because a new KB deserves a worked example of the format, not because the format
requires one. What cannot be dropped is its other role — the page nothing is
expected to link to, which is what lets an extract's root lint clean under
`--strict` (`D71`) — and an extract renames its root into that place for exactly
that reason.

**What is deliberately absent.** No "recently changed" list: the format records no
dates for pages, and mtimes are worthless the moment a KB is cloned or untarred,
when every file arrives with the same timestamp — so recency comes from the
bibliography, where `cite add` and `fetch` write the day a source was read. No
page of the day or random article, because date-seeded output would make a build
irreproducible and break the golden tests. No drafts, because `Load` deliberately
never reads `inbox/`. No images, because the format has no such concept.

**Provenance.** Put to the tool author as a proposal with the alternatives, after
reading what Wikipedia's Main Page is actually made of, and answered: a landing
page in that shape, wrapping the entry document rather than replacing it.

### The site's type. `D73` added.

**Decision.** Text reads in Work Sans, falling back to Roboto and then the
reader's own system UI face. The fonts are named in the stylesheet, not shipped:
no font files live in the repo, so a built site stays self-contained (`D3`) and
costs no bytes. Everything but code reads in that face — headings and long-form
prose alike — so the body's serif stack is gone.

**Why named and not shipped.** The site's promise is that it is self-contained:
one stylesheet, no network, no build step. Bundling the files would keep that
promise and make the font render everywhere, but it puts binary assets in a
repository that is otherwise all text and grows every built site by them. Loading
the font from a CDN would break the promise outright. Naming it is the middle: a
reader who has Work Sans sees it, and everyone else gets the system stack the
site used before, so the change is never worse than what it replaces.

**Why the whole page.** The body was serif and only the chrome was sans, so a
font named for the chrome would have touched a fraction of what a reader reads.
Asking for a primary face means the prose reads in it too, and the stylesheet
loses the serif stack rather than keeping two.

**Provenance.** Asked of the tool author as a direct request — Work Sans as the
primary face, Roboto as the fallback — with delivery, scope and fallback put as
separate questions. The answers: name-only, the whole page, Roboto named but not
shipped.

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

## Deferred during implementation

### Portable-markdown export. The second clause of `D19` is deferred.

`D19` says wikilinks are "exported to portable markdown", following Q15(b)'s
"exporter emits standard relative links". No command does this. A scoped extract
is a KB root in the same format, so it keeps `[[wikilinks]]` (`D30`, `D71`);
`export json` is machine JSON (`D70`); and the renderer emits relative links in
HTML, not markdown (`D69`). The relative-link intent is met only by the site.

The promise is deferred, not dropped. Taken up, it is a markdown export whose
wikilinks become standard relative paths. It is separate from the scoped extract,
which must stay a KB root for the tool to read. Deferred by the author on
2026-09-29.

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
- Input formats other than PDF and HTML. EPUB is a zip container with its own
  structure and metadata; a scanned page needs OCR, which is a different class of
  tool; a page that only exists after JavaScript has run needs a browser. The shim
  refuses each by name — `unsupported` for the EPUB, `empty` for the other two —
  rather than returning an empty document, and the shim contract is the seam any of
  them would arrive through if one were ever wanted (`D66`).

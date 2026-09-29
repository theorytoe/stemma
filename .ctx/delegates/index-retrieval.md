---
title: Stemma — Index and retrieval
date: 2026-09-26
---

This is a delegate plan. It covers Task 5 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Make retrieval work at a scale nobody has committed to yet. `D5` says scale is
unbounded in principle and `D4` makes retrieval first-class, so the design must
hold at ten thousand pages without ever costing the small case anything.

## The two tiers, and the rule that makes them real

Tier 0 is the default and requires no setup: read files directly, resolve the graph
in memory, answer. Tier 1 is an explicit `index` command that writes a cache
enabling fast search, precomputed backlinks, and citation maps. Every command uses
the index when it exists and is fresh, and silently falls back to Tier 0 when it
does not exist, is stale, or was never built (`D23`).

The rule that makes this real rather than decorative: **no command may require the
index.** If any command fails or degrades noticeably without it, the tier is a
dependency and `P1` has been violated. The fallback path deserves as much testing
as the fast path, and it is the path a new user actually experiences.

**Where the index state is reported today.** `status` and `env` each print an
`index` line, and neither can be right yet: nothing builds an index. The check
they used was the newest modification anywhere under `.stemma/`, which stopped
meaning anything as soon as that directory also held the extraction script and the
text `fetch` reads — it reported a fresh index on a KB that had just been created.
Both now report the index as absent, which is the truth until this delegate lands.
Naming the index file and looking for it belongs here.

Nothing generated is committed. The index lives inside the KB root in a gitignored
directory, alongside the scratch area from `delegates/source-extraction.md`.

## Storage

SQLite is available through `modernc.org/sqlite`, which is cgo-free and — verified,
not assumed — compiles FTS5 in by default, so no extension loading is needed and
the binary stays statically linkable. Use FTS5 for text search with BM25 ranking.

The index is a cache, never a source of truth. Design it so it can be deleted at
any time and rebuilt with no loss and no manual repair. That property is what keeps
the cache honest: any state that exists only in the index is a bug.

## Open question this delegate answers

`U1` — the page count at which Tier 0 stops being fast enough. Nobody knows, and
the answer decides whether Tier 1 is a convenience or a necessity. Task 5 exists
to measure it rather than guess, and the number belongs in the documentation so
users can tell which tier they are in.

## Dependencies

Task 1 for the resolver and the Tier-0 graph, which this delegate accelerates
rather than replaces. Task 7 of the project consumes `search` and `graph`.

# Tasks

## Task 1:

Status: Done

Index store design. Schema, on-disk location inside the generated-artifact
directory, and a version marker so an incompatible schema is rebuilt rather than
misread. Record what is stored: pages, frontmatter fields used for filtering, the
link graph, citation keys, and the FTS table. Nothing that cannot be regenerated.

Landed as `internal/index`: `<kb>/.stemma/index.sqlite`, schema version in
`user_version`, rebuilt by deletion on any mismatch or on a file that is not a
database. Tables for pages (with type, status, tags), claimed names, links,
citations and tags, plus a contentless FTS5 table over Go-produced tokens.
The choices are recorded as `D67`.

## Task 2:

Status: Done

FTS5 population and ranking. Build the full-text table over page bodies plus the
searchable frontmatter fields, use BM25 with sane weights, and choose a tokenizer
with the consequences understood for code, identifiers, and non-English text.
Expose snippets so results are useful without opening every page.

`internal/index` gained `Tokenize` (one Go tokenizer shared by both tiers),
`Score`/`Stats` (BM25 with title, tag and body weights), `Populate` (rebuilds the
contents from a loaded KB) and `Snippet` (plain-text excerpt with the match
marked). FTS5 holds Go-produced tokens rather than raw prose, so it cannot decide
what a match is; the tokenizer and ranker moved out of FTS5 for exactly that
reason, amending `D67`.

## Task 3:

Status: Done

Incremental updates. Stamp each page by content hash rather than modification time,
because files get touched without changing, and clock skew across machines makes
mtimes unreliable in a synced directory. Support a full rebuild, an incremental
refresh, and the detection of a KB that changed underneath the index.

`internal/index` gained `Refresh` (rewrites only pages whose content hash moved),
`Populate` (full rebuild), `Report` (pages/added/updated/removed), `State` with
`Check`/`CheckKB`/`Synced` for changed-underneath detection, and `KBHashes`.
`kb.Hashes` hashes page bytes without parsing and shares the page walk with
`Load`, so the two cannot disagree about which pages exist. The `stemma index`
command builds or refreshes, `--rebuild` discards first, and `status`/`env` now
report the real state. The text table moved to external content (schema version
3) so one page can be removed without rebuilding the whole index.

## Task 4:

Status: Done

Transparent fallback. A single internal query interface that both tiers implement,
with freshness checked before use and a silent downgrade to Tier 0. Log the
downgrade at most once per invocation; silence is the requirement, noise is not.

Landed as `index.Source`, a *data* interface both tiers implement (pages,
tokenized documents, candidate matching, document frequencies, names, links,
backlinks, citations, plus the batch forms of the text and link relations a rule
over the whole corpus reads), so search and graph are single functions over it
and the tiers cannot disagree. `newKBSource` wraps a loaded KB; `newIndexSource` answers
from the store alone. `NewSource` checks freshness, returns the index when it is
fresh, and otherwise falls back to the KB, calling an optional callback once with
the reason. Recorded as `D68`.

## Task 5:

Status: Done

Benchmark and publish `U1`. Generate synthetic KBs across a range of page counts
and measure the commands that matter — resolution, lint, search, graph traversal —
on both tiers. Record the point where Tier 0 becomes uncomfortable, commit the
benchmark so it can be re-run, and write the finding into `FORMAT.md` and the
README. This is a measurement task, not an optimisation task.

`BenchmarkTiers` in `internal/index/bench_test.go` builds synthetic KBs of 100,
1,000 and 10,000 pages and measures the two commands end to end, `search` and
`graph`, on both tiers, alongside load, hashing, freshness checking,
tokenization, resolution, lint, candidate matching, backlink traversal and the
orphan listing; `make bench` runs it one iteration per size. An earlier draft
measured candidate matching rather than `Search` and reported the index as four
to five times faster; the review that compared the two showed the full command
was then about ten times slower on Tier 1, because ranking fetched three rows
per candidate, and `Source.Docs` now fetches them in batches. The corrected
finding: Tier 0 is comfortable to roughly 1,000 pages; the index saves about a
third of a search at 1,000 and at 10,000 pages, and one build pays for itself
after roughly twenty searches, so the index is worth it under repeated queries
rather than above a particular size. Recorded in `FORMAT.md` ("Tiers and
scale"), the new `README.md`, and `U1` in the register.

## Task 6:

Status: Done

`search`. Ranked full-text with filters for type, tag, and status, `--include-inbox`
to reach drafts, and `--json` output. Support scoping to a directory within
`pages/` for convenience, while never treating a directory as a semantic boundary
(`P10`).

`index.Search` ranks a `Source` plus optional extra documents (drafts) with the
shared BM25, then filters by type/status/tag/directory and caps the result.
`Source` gained `Docs`, the batch text fetch that ranking and snippets share, so
one candidate costs a place in a query rather than a round trip of its own; and
`NewSourceAt` lets the command answer from a fresh index without parsing any
page. The `stemma search` command reports
the tier in `--json` and stays silent in text; `--limit` caps results. Result
ranking and membership are identical on both tiers.

## Task 7:

Status: Done

`graph`. Neighbourhood traversal with `--in` for backlinks and `--out` for forward
links, bounded by `--depth`, and `--json` output. Backlinks are folded into this
command rather than getting a dedicated verb. Include a way to find orphans and
dead ends, since lint reports the same facts without the traversal view.

`index.Neighbors` walks a `Source` breadth-first in either direction, reporting
each page once at its first hop, dropping forward links that do not resolve to
exactly one page and treating `--depth 0` as the whole reachable set.
`index.ResolvePage` resolves a start by path or name through the source, so the
command never needs the KB object. `Orphans` and `DeadEnds` are functions over
the source's data (`Pages`, `LinksAll`, `BacklinksAll`), so lint's orphan rule
lives once rather than an SQL expression on the index and a loop on the KB. The
`stemma graph` command takes a page or
`--orphans`/`--dead-ends`, reports the tier in `--json`, and stays silent in text.

## Task 8:

Status: Done

Tier-parity tests. Assert that every query returns the same answer on both tiers
for the same corpus. This is the test that prevents the index from silently
becoming authoritative, and it should run against the example wiki as well as
synthetic corpora.

`parity_test.go` runs the whole retrieval surface against both tiers: pages,
tokenized docs, the batch text and link forms, links, backlinks, citations, a
battery of derived search requests, neighbourhood walks at three directions and
depths, orphans, dead ends, name resolution, citation maps and average length. It
runs on synthetic corpora of 1, 5 and 40 pages and on a temporary copy of the
project's own example wiki (the generated `.stemma/` is skipped). The synthetic
corpus carries multi-tag pages, which is where the tiers were found to differ
only in the order of a Doc's tags — invisible in a score, visible to the test.

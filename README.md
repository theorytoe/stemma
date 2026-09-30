# stemma

A tool for authoring and maintaining a citation-bearing knowledge base on disk.
Pages are markdown with a small frontmatter block, `[[wikilinks]]` and `[@key]`
citations; sources live in BibTeX. The format is specified in
[FORMAT.md](FORMAT.md).

## Build and check

    make build     # build bin/stemma
    make install   # put stemma on PATH
    make check     # build, vet, test, the shim's tests, and lint the example wiki
    make bench     # measure the two retrieval tiers (below)

## Tiers and scale

Retrieval runs in two tiers. Tier 0 reads the KB directly and needs no setup.
Tier 1 is an index under `.stemma/`, built by `stemma index`; every command uses
it when it is fresh and falls back to Tier 0 otherwise, and none requires it.

The index is stamped by content hash, so it is checked by content rather than by
timestamp: a file touched without changing does not invalidate it, and a file
changed on a machine with a wrong clock does. That check reads every page, so the
index saves parsing and tokenizing, not reading.

The page count at which Tier 0 stops being comfortable (`U1`) was measured with
`make bench`. On a 2020 six-core laptop, a Tier-0 search — load, tokenize, rank —
takes about 3 ms at 100 pages, 35 ms at 1,000, and 0.35 s at 10,000; the indexed
path takes 2 ms, 26 ms, and 0.23 s, and building the index costs 19 ms, 0.19 s,
and 2.7 s. Resolution is O(1) (about 10 µs at any size) and lint stays under
35 ms at 10,000 pages.

So Tier 0 is comfortable to roughly a thousand pages. The index saves about a
third of a search (the freshness check still reads every page, and ranking still
re-tokenizes each candidate), so one build pays for itself after roughly twenty
searches, at a thousand pages and at ten thousand alike — worth it when the KB is
queried many times between edits, not above a particular size. The numbers are
machine-specific; re-run `make bench` to get the ones that matter. The full table
and the reasoning are in [FORMAT.md](FORMAT.md#tiers-and-scale).

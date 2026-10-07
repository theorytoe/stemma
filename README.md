# stemma

**WARNING:** this project is a personal experiment in 'oneshot' style ""vibe coding""
I basically told the demon machine what to do and it did it, I had very little
intervention in the architectural design of the project. Therefore there is a chance
things are messy.

A tool for authoring and maintaining a citation-bearing knowledge base on disk.
Pages are markdown with a small frontmatter block, `[[wikilinks]]` and `[@key]`
citations; sources live in BibTeX. The format is specified in
[FORMAT.md](FORMAT.md); the example wiki under `wiki/` is the project's own
documentation and is built by the tool itself.

## Install

Go 1.27 or newer. PDF and HTML text extraction additionally needs a `python3`;
everything else works without it.

    git clone https://github.com/theorytoe/stemma
    cd stemma
    make install

`make install` builds with the version stamp and puts `stemma` on `PATH` (it uses
`go install`, so the binary lands in `GOBIN`, or in `GOPATH/bin` when that is
unset). `make build` writes `bin/stemma` instead, if you would rather not install
it. `make install-skills` copies the agent skills into `~/.agents/skills`, and
`make install-man` puts the manual pages where `man` looks for them.

## A first knowledge base

    stemma init demo --title "Demo KB"
    cd demo
    stemma new "Vector search" --type concept
    $EDITOR pages/vector-search.md    # write prose; link with [[...]], cite with [@...]
    stemma lint --strict
    stemma build                      # writes .stemma/site

[A first knowledge base, end to end](wiki/pages/worked-example.md) is that
walkthrough in full, with the real output of every command.

## Build and check

    make build          # build bin/stemma
    make install        # put stemma on PATH
    make install-skills # copy the agent skills into ~/.agents/skills
    make man            # write the manual pages to man/
    make install-man    # put the manual pages where man looks for them
    make check          # build, vet, test, the shim's tests, and lint the example wiki
    make bench          # measure the two retrieval tiers (below)

## Dependencies

Four direct dependencies, pinned in `go.mod` and present in the module cache, so
a build works offline:

- `gopkg.in/yaml.v3` — frontmatter.
- `github.com/BurntSushi/toml` — the manifest.
- `github.com/russross/blackfriday/v2` — markdown rendering.
- `modernc.org/sqlite` — the Tier-1 index.

The list is kept small deliberately. Command dispatch, link resolution, the
renderer's structure, and everything else are the standard library's, and these
four are what is left once nothing else is let through.

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

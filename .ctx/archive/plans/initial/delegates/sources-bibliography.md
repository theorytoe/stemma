---
title: Stemma — Sources and bibliography
date: 2026-09-26
---

This is a delegate plan. It covers Task 3 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Make sources first-class. Pages cite evidence; the bibliography is the single
authoritative record of that evidence, and the tool guarantees that a citation
either resolves or is reported as broken.

## The division of labour that matters

Tools resolve identifiers into BibTeX. Agents never hand-write BibTeX (`D37`).
Large language models produce confidently wrong bibliographic records — invented
page numbers, shifted years, plausible but non-existent venues — and a
bibliography exists precisely to prevent that corruption. Every task here assumes
the metadata comes from an authoritative resolver or from a human, never from a
model's memory.

## Format decisions this delegate implements

BibTeX is the source of truth (`D21`). Storage is a single `.bib` file by default,
with a `bibliography/` directory of per-source files supported as a first-class
alternative for large KBs (`D34`); the tool must treat both identically so the
choice can be deferred until it hurts.

Citations are written inline as `[@key]`, pandoc style, supporting several keys in
one bracket (`D20`). Rendering is a small built-in formatter covering a few common
styles selected by `citation_style` in the manifest. There is no CSL conformance:
full CSL implies `citeproc-js`, which `D9` rules out (`D35`). This limitation must
be documented in `FORMAT.md` so nobody expects a CSL style identifier to work.

"Robust bibliography" is defined concretely as reverse citation lookup, orphan
detection, stable keys with retrieval date and content hash, duplicate detection,
and validated BibTeX and CSL-JSON export (`D22`).

Source pages exist but are virtual: materialised from BibTeX at export and build
time, never committed (`D45`, `D36`). They carry the reserved `source` type, are
excluded from orphan and title-collision checks, and are resolvable through
`[[key]]`. Virtual is the point — there is only ever one copy, so a source page
cannot drift from its record.

## Where this meets the extraction delegate

`cite add <url>` produces a bibliographic record and belongs here. `fetch <url|pdf>`
produces text for an agent to read and belongs to
`delegates/source-extraction.md`. The same URL can flow through both, with
different outputs. Keep the boundary clean: this delegate never extracts document
text, and that delegate never writes bibliography entries.

## Provenance

A source is a pointer by default — identifier or URL, retrieval date, and content
hash — with vendored full text opt-in per source (`D38`). The hash is computed
over whichever representation was actually retrieved, which means a record may
legitimately carry a hash with no vendored copy, and may temporarily carry neither
until something is fetched. Record that state honestly rather than inventing a
hash.

## Dependencies

Task 1. Task 6 depends on the formatters here. Task 4 depends on the key and
provenance conventions established here.

# Tasks

## Task 1:

Status: Done

BibTeX parse and serialise. Support the entry types and fields that real exports
contain without normalising them away, preserve unknown fields and entry types
verbatim on write (`P4`), and round-trip a corpus of real records from Zotero,
Crossref, and arXiv without loss.

## Task 2:

Status: Done

Identifier resolution for DOI, arXiv, and ISBN, plus URL as a bibliographic
record. Define the network contract: timeouts, retries, user agent, and a clear
operational failure with exit code `2` when offline. Support manual entry as the
offline fallback, because a bibliography that cannot be edited without a network
is not usable.

It closed with resolution in a new `internal/source` package: DOI through
doi.org content negotiation, which reaches Crossref, DataCite and mEDRA without
agency sniffing; arXiv through its own BibTeX export; ISBN through Open Library,
following the edition's work for author names; and URL through the page's own
head metadata (Highwire, Dublin Core, JSON-LD, OpenGraph), delegating to the DOI
resolver when the page names one. The network contract is a single client: a 15s
timeout, two retries with exponential backoff and jitter, Retry-After honoured,
and a `stemma/<version>` user agent stamped by the Makefile. Failures carry a
kind, so the CLI can map a finding about the input to exit 1 and an operational
failure to exit 2, and an `Offline` mode refuses the network up front. `Manual`
builds the same kind of entry from supplied fields and adds no retrieval date,
because nothing was retrieved. The `--offline` flag itself lands with `cite add`,
the first command that can carry it.

## Task 3:

Status: Done

`cite add`. Create or update the entry, record retrieval date and content hash,
detect duplicates before appending, and write back through the preserving
serialiser. Duplicate detection is the interesting part: the same work arrives
under a fresh key more often than one expects, and silently accumulating two
copies defeats reverse lookup.

It closed with the first member of the `cite` family and the first command that
writes to the bibliography. `cite add <identifier>` resolves and writes;
`cite add --title ...` builds the record by hand, which is the offline fallback
and so carries no retrieval date. Both paths meet one duplicate check: the key
first, then a normalised DOI, arXiv identifier, ISBN or URL, then a normalised
title, year and first author, which is what sees the same work under a fresh key.
A match merges into the entry already there and keeps its key, never deleting a
field; `--force` appends a second copy deliberately. The write goes back through
the preserving `BibFile`, so comments and every entry the command did not touch
come back byte for byte, into whichever form the KB already uses. Provenance is
`stemma-retrieved` plus `stemma-content-hash`, a sha256 over the bytes the
resolver actually returned, written only for a fetched record. A resolution that
names nothing is exit 1; a network failure is exit 2; and `--offline` (or
`STEMMA_OFFLINE`) refuses the network up front. The command also corrected the
`--json` envelope to name a family member in full, as "cite add".

## Task 4:

Status: Done

Citation resolution in page bodies. Parse `[@key]`, support several keys in one
bracket, resolve to entries, and emit per-page reference lists. Report cited-but-
absent and present-but-uncited keys. This is the read path that lint and rendering
both consume.

It closed by growing the citation model past a key and a line to the whole
inline form — group, position within it, prefix, locator, author suppression and
narrative versus parenthetical — so the renderer reads the prose once rather
than twice. `Page.Citations` still returns the same keys in the same order, so
lint's findings, the graph's citation index and `cited-by` are unchanged. The
`Bibliography` now keeps the record each key defines, first definition winning,
which is what lets `Entry(key)` and `KB.References(path)` turn a page's
citations into its reference list (in order of first citation, deduplicated) and
a separate list of the keys nothing defines. Nothing is surfaced on the command
line yet: the `cite` family (subtask 5) and the renderer (subtask 6) consume
this read path.

## Task 5:

Status: Done

The `cite` family: `list` with filters and `--uncited`, `show` for one entry,
`check` for duplicates, malformed records, missing hashes and dates, and absent
keys, and `cited-by` for reverse lookup. All support `--json`.

It closed with four read verbs beside `add`. `list` filters by `--type`,
`--author`, `--year` and `--cited`/`--uncited` and orders by key, so its output
is a property of the data rather than of the filesystem. `show` prints the
record's own bytes, every field verbatim beside its decoded value, where the
record lives and which pages cite it. `cited-by` is the reverse lookup. `check`
reports the four facts the delegate names — duplicate keys, cited-but-absent
keys, missing retrieval dates and missing content hashes — and adds the same-work
duplicate subtask 3 made detectable plus validation of the provenance values'
form, reusing lint's codes for the two facts lint also reports and exiting 1 on
findings through the same renderer and envelope. `show` and `cited-by` on a key
nothing defines exit 2, because being asked for a missing thing is a failure to
complete rather than a finding about the KB. `Bibliography` gained `SortedKeys`
and `BibEntry` gained `Title`, `Year` and `Authors` for the listings.

## Task 6:

Status: Done

Built-in formatters. A small set covering a few common styles, selected by
`citation_style`. Keep it deliberately small and well tested; the failure mode to
avoid is growing a partial CSL implementation by accident. Document the supported
styles and the absence of CSL in `FORMAT.md`.

It closed with a new `internal/citestyle` package holding two styles,
`author-date` and `numeric`, each rendering both the in-text citation and the
reference-list entry so that the author label and the year cannot disagree
between the two. Both honour all five inline forms — locator, narrative,
suppression and a group — and `numeric` numbers a key by its first citation on a
page, which is why the formatter takes the page's reference list. `Parse`
refuses an unknown `citation_style` instead of falling back, so nothing renders
in a style nobody chose; the manifest is not validated at load, because a KB used
only for lint should not be unreadable over a style it never uses. `FORMAT.md`
now documents the two styles, their exact shapes, and the deliberate absence of
CSL. No command surface was added: the renderer in top-level Task 6 is the
consumer.

## Task 7:

Status: Done

Virtual source pages. Materialise one per cited entry at build and export time,
carrying the reserved `source` type and exposing the entry's fields, retrieval
date, hash, and the pages that cite it. Exclude them from orphan and
title-collision checks, and ensure `[[key]]` resolves to them without colliding
with a page whose title happens to equal a key.

It closed with `kb.SourcePage(entry, citedBy)` and `KB.SourcePages()`: one
virtual `*Page` per cited entry, carrying the reserved `source` type, the entry's
key in the `key` field, the record's title (or the key) as its title, and a
generated body that lists every field, the retrieval date, the content hash and
the citing pages as wikilinks. Only cited entries get a page, in key order.

The `[[key]]` clause was read against D45 and `FORMAT.md`, which both say a
citation resolves to a source page and a wikilink does not. The source page
therefore answers to no name in the graph — `claimedNames` returns nothing for
`type: source` — so a page whose title happens to equal a citation key cannot
collide with it, and it is never an orphan or a backlink target. That exemption
is enforced in `Graph.Add`, `Remove`, `Backlinks` and `Orphans`. Nothing is
user-visible yet: `build` and `export` (top-level Task 6) materialise the pages.

## Task 8:

Status: Done

The `bibliography/` directory form. Support one file per entry as an alternative to
a single `.bib`, with identical behaviour from every command, and define what
happens when both forms are present. Document when to switch.

Most of the form was already in place from earlier subtasks: `Load` reads
`bibliography.bib` and every `bibliography/*.bib` and merges them, so `list`,
`show`, `check`, `cited-by`, `lint` and `status` all see one bibliography
whichever form holds it. What this task added was the rule and the bug the
earlier work left open. A per-entry file is now named from the key and takes a
`-2`, `-3`, ... suffix when a different key already owns that name, so two keys
that normalise alike no longer share a file. A new entry is written to
`bibliography.bib` whenever that file exists, which makes both forms together a
defined migration state rather than an accident — and both-present is
deliberately not a `cite check` finding, because the format allows it. An
update always goes to the file the entry is already in. `FORMAT.md` now says
when the directory form is worth switching to and where a new entry lands. Tests
cover the form end to end: listing, `show` naming the defining file, `add`
creating and updating per-entry files, both-forms-present writing the single
file, and a duplicate key found across two files.

## Task 9:

Status: Done

Validated export. Emit the bibliography as BibTeX and as CSL-JSON, verify the
output parses with an independent reader, and expose it through the export surface
so a KB's evidence can leave the KB intact.

It landed as `cite export --format bibtex|csl-json`, alongside the rest of the
bibliography family rather than in a new `export` family, with `--cited` to
narrow the output to the records some page relies on and `-o` to write a file
instead of stdout. `kb.ExportBibTeX` re-renders every entry with its values
resolved, so a `@string` macro a source file defined is written out in full and
the result is self-contained; entry types, keys, field names, values and the
tool's own provenance fields all survive, and only the `@string`/`@preamble`
scaffolding is left behind. `kb.ExportCSLJSON` maps the BibTeX fields CSL names
(entry type to CSL type, `journal`/`booktitle` to `container-title`, `year` +
`month` + `day` to `issued`, `stemma-retrieved` to `accessed`), reads names as
`von Last, Jr, First` or `First Last` with a braced name as a literal, and
carries every field CSL has no home for through under its own name, so the
content hash survives the trip. Both orders are by key, so two exports of one
bibliography are the same bytes.

The verification is a reader other than the writer: CSL-JSON is decoded with the
stdlib `encoding/json` into a CSL schema written by hand for the test, and
BibTeX is read back by `bibtex` and `biber` when they are installed — those two
tests skip rather than fail when TeX is absent, so the suite has no new
dependency, and both pass on a machine that has them.

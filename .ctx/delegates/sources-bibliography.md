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

Status: Not started

BibTeX parse and serialise. Support the entry types and fields that real exports
contain without normalising them away, preserve unknown fields and entry types
verbatim on write (`P4`), and round-trip a corpus of real records from Zotero,
Crossref, and arXiv without loss.

## Task 2:

Status: Not started

Identifier resolution for DOI, arXiv, and ISBN, plus URL as a bibliographic
record. Define the network contract: timeouts, retries, user agent, and a clear
operational failure with exit code `2` when offline. Support manual entry as the
offline fallback, because a bibliography that cannot be edited without a network
is not usable.

## Task 3:

Status: Not started

`cite add`. Create or update the entry, record retrieval date and content hash,
detect duplicates before appending, and write back through the preserving
serialiser. Duplicate detection is the interesting part: the same work arrives
under a fresh key more often than one expects, and silently accumulating two
copies defeats reverse lookup.

## Task 4:

Status: Not started

Citation resolution in page bodies. Parse `[@key]`, support several keys in one
bracket, resolve to entries, and emit per-page reference lists. Report cited-but-
absent and present-but-uncited keys. This is the read path that lint and rendering
both consume.

## Task 5:

Status: Not started

The `cite` family: `list` with filters and `--uncited`, `show` for one entry,
`check` for duplicates, malformed records, missing hashes and dates, and absent
keys, and `cited-by` for reverse lookup. All support `--json`.

## Task 6:

Status: Not started

Built-in formatters. A small set covering a few common styles, selected by
`citation_style`. Keep it deliberately small and well tested; the failure mode to
avoid is growing a partial CSL implementation by accident. Document the supported
styles and the absence of CSL in `FORMAT.md`.

## Task 7:

Status: Not started

Virtual source pages. Materialise one per cited entry at build and export time,
carrying the reserved `source` type and exposing the entry's fields, retrieval
date, hash, and the pages that cite it. Exclude them from orphan and
title-collision checks, and ensure `[[key]]` resolves to them without colliding
with a page whose title happens to equal a key.

## Task 8:

Status: Not started

The `bibliography/` directory form. Support one file per entry as an alternative to
a single `.bib`, with identical behaviour from every command, and define what
happens when both forms are present. Document when to switch.

## Task 9:

Status: Not started

Validated export. Emit the bibliography as BibTeX and as CSL-JSON, verify the
output parses with an independent reader, and expose it through the export surface
so a KB's evidence can leave the KB intact.

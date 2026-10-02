---
name: stemma-research
description: >-
  Find, resolve, and record the outside sources a Stemma knowledge base cites.
  Use when a source is not yet in the bibliography: searching for papers or
  documents, resolving a DOI, arXiv ID, ISBN, or URL, adding or editing a BibTeX
  entry, fetching a PDF or web page's text, vendoring a snapshot, or checking
  the bibliography for duplicates and uncited entries. Operates `stemma cite`
  and `stemma fetch`. Reading pages that already exist is stemma-query, not
  this.
---

# Stemma research

Research is the whole path from "I need a source" to "the KB cites it correctly":
resolve the work, record it once, read it, and cite the key the tool minted.

This skill pairs with the `stemma` umbrella, which carries the format rules and
the tool-versus-judgement boundary. Read that first if it is not already loaded.

## The operation

**Tools capture; you synthesise.** The tool resolves identifiers and extracts
text, and it alone writes BibTeX. You never hand-write an entry and never invent
a citation key. Your judgement is in deciding what is worth recording.

## Steps

1. **Check the bibliography first.** A work already recorded under another key
   is the failure this step prevents. List what is there before you add:

   ```sh
   stemma cite list                # the whole bibliography
   stemma cite list --uncited      # entries no page cites
   stemma cite show KEY            # one entry
   stemma cite cited-by KEY        # which pages rely on it
   ```

2. **Resolve the identifier, or enter fields by hand.** `cite add` resolves DOI,
   arXiv ID, ISBN, and URL against an authoritative source, then mints a key.
   Prefer this over entering fields yourself.

   ```sh
   stemma cite add 10.1145/3290605.3300233
   stemma cite add --arxiv 2401.12345
   stemma cite add --isbn 9780262033848
   stemma cite add --url https://example.org/paper
   stemma cite add --path ./downloads/paper.pdf --title "A Local Paper"
   stemma cite add --title "A Work Entered By Hand" --year 2024 --container "A Journal"
   ```

   Useful flags: `--key` to name the entry, `--type` for the BibTeX entry type,
   `--dry-run` to see what would be written, `--offline` to refuse the network.
   A source that is a file on this machine takes `--path`, not `--url`: a path
   is not an address. A positional argument that names an existing file is read
   as a path too. Either way a local file still needs its title and year, because
   there is no resolver for a file. `cite add` updates an entry it already has
   rather than appending a second copy of the same work; `--force` is the
   deliberate way to keep a duplicate.

3. **Read the source.** `fetch` writes a document's text into the KB's scratch
   area so you can work from it. It does not add it to the KB.

   ```sh
   stemma fetch https://example.org/paper.pdf
   stemma fetch ./downloads/paper.pdf
   stemma fetch --clear            # empty the scratch area and stop
   ```

   Source text is **not vendored by default** -- the entry keeps a pointer, a
   retrieval date, and a content hash. Snapshot the full text only when the
   source may move or vanish:

   ```sh
   stemma cite vendor KEY
   ```

   A vendored capture is committed. When the source is a file on this machine,
   the original is copied into `sources/` beside the extracted text, and `build`
   and `serve` publish the capture for a reader: a PDF opens in the browser's
   viewer, markdown is rendered, and text is shown as text, with the original
   bytes always a click away.

4. **Cite the key in a page.** Write `[@key]` in the prose, using the key the
   tool minted. Do not paraphrase the key or derive one from the title. The
   bibliography entry and the page reference are checked against each other by
   `stemma lint`.

5. **Audit the bibliography when done.**

   ```sh
   stemma cite check               # duplicates, bad identifiers, uncited entries
   ```

## Boundary

- Reading or searching pages that already exist: `stemma-query`.
- Writing the prose that cites a source: `stemma-author`.
- This skill owns everything up to and including the bibliography entry.

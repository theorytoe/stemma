---
name: stemma-publish
description: >-
  Turn a Stemma knowledge base into output another person or program can use.
  Use when building the static site, serving the KB locally, exporting the whole
  KB as JSON, extracting one page and its closure as a self-contained KB, or
  exporting the bibliography as BibTeX or CSL-JSON. Operates `stemma serve`,
  `build`, `export json`, `export page`, and `cite export`. This is output and
  sharing, not authoring or source-gathering.
---

# Stemma publish

Publishing is turning a KB into something that leaves it: a site a person browses,
a JSON document a program reads, or a self-contained subset handed to someone
else.

This skill pairs with the `stemma` umbrella, which carries the format rules and
the tool-versus-judgement boundary. Read that first if it is not already loaded.

## The operation

**Output is generated, never committed.** Every form below writes a derivative.
It is produced on demand and lives under `.stemma/` or a directory you name. It
is never edited by hand and never committed to the KB's history.

## Steps

1. **Preview as a site.**

   ```sh
   stemma serve                       # http://127.0.0.1:8080, runs until stopped
   stemma serve --addr 127.0.0.1:9000
   ```

   The served site works with JavaScript off; the script is polish only.

2. **Write a static site.**

   ```sh
   stemma build                       # default .stemma/site in the KB
   stemma build --out public/site
   ```

   The output needs no server: every page, the home page, the index, the type
   and tag lists, the graph, a virtual page for every cited source, and the
   captured text of every source that was vendored.

3. **Hand the whole KB to a program.**

   ```sh
   stemma export json                 # default .stemma/export.json
   stemma export json --out -         # to standard output
   ```

   One JSON document carrying the pages and their bodies, the links, the
   citations, and the bibliography, with a schema version of its own. It stands
   alone: a reader needs nothing but the file.

4. **Hand someone a subset.**

   ```sh
   stemma export page "The Entry Page" --depth all --out .stemma/kit
   stemma export page "The Entry Page" --report-pruned
   ```

   The extract is itself a KB root in the same format, so every tool reads it.
   It writes the entry page as the root document, prunes every construct it
   cannot keep into its anchor text, and writes a `bibliography.bib` holding
   exactly the sources the kept pages cite, so the subset closes on its own.
   Default depth is one hop; `--depth all` is the closure.

   `--out` is relative to the KB root, and the default is
   `.stemma/extract/<page>`, which is already gitignored. Keep hand-off output
   under `.stemma/` unless you are deliberately writing to a directory you
   manage: a path elsewhere inside the KB is not ignored.

5. **Export the bibliography alone for a reference manager or citation
   processor.** The extract in step 4 already carries the entries its pages
   cite; this is for handing the bibliography to a different tool.

   ```sh
   stemma cite export                       # BibTeX to standard output
   stemma cite export --format csl-json
   stemma cite export --cited               # only what pages rely on
   stemma cite export -o refs.bib
   ```

   `cite export -o` is relative to your working directory, not the KB, unlike
   the `--out` used above.

## Before you ship

Run the KB's own checks over whatever you are about to hand over. A published
artifact that does not lint clean is the one thing a recipient notices first.

```sh
stemma lint --strict
stemma build          # for a site
```

An extract is a KB root, so lint it as one:

```sh
stemma lint --kb .stemma/kit --strict
```

## Boundary

- Writing pages: `stemma-author`.
- Finding or recording sources: `stemma-research`.
- This skill owns output and sharing, not content.

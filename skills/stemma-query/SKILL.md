---
name: stemma-query
description: >-
  Answer questions from a Stemma knowledge base. Use when asked what the KB says
  about a topic, to look something up, or to gather the pages, links, and
  citations around a subject already recorded on disk. Search with `stemma
  search`, read with `stemma show`, follow with `stemma graph`, and cite the
  pages and sources the answer rests on. Do not answer from memory while a
  relevant page is unread. For sources not yet in the bibliography, use
  stemma-research.
---

# Stemma query

Query is the shortest skill on purpose: search, read, follow, cite. Every extra
token here is paid on every retrieval.

This skill pairs with the `stemma` umbrella, which carries the format rules and
the tool-versus-judgement boundary. Read that first if it is not already loaded.

## The operation

**Answer from the KB, not from memory.** The characteristic failure is a
plausible answer given while a correct, cited page sits unread on disk. Search
first, every time.

## Steps

1. **Search.**

   ```sh
   stemma search "the question in a few content words"
   stemma search "term" --type concept --limit 10
   stemma search "term" --include-inbox        # drafts too, deliberately
   ```

2. **Read the pages it returns.**

   ```sh
   stemma show "The Page Title"
   ```

   `show` resolves the page's links and citations, so you read what the KB
   actually means rather than the raw file.

3. **Follow the graph for context.**

   ```sh
   stemma graph "The Page Title" --out --depth 1   # what it leads to
   stemma graph "The Page Title" --in --depth 1    # what leads to it
   ```

4. **Answer, and name the evidence.** Say which pages the answer comes from and
   which sources those pages cite. If the KB does not hold the answer, say so
   plainly rather than filling the gap from memory.

## Boundary

- The source is outside the KB and not yet recorded: `stemma-research`.
- The question is about the KB's own health, not its content: `stemma-maintain`.
- This skill reads; it does not write.

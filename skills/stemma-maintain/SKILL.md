---
name: stemma-maintain
description: >-
  Audit and repair a Stemma knowledge base that already exists. Use when
  linting, checking the KB's health, finding orphan or dead-end pages, resolving
  broken or ambiguous wikilinks, archiving a page with a reason, renaming a page
  so inbound links follow, or rebuilding the search index. Operates `stemma
  lint`, `status`, `archive`, `rename`, `move`, `index`, and `graph`. Writing new
  content is stemma-author.
---

# Stemma maintain

Maintenance is keeping a KB that already exists healthy: finding what is broken,
fixing it with the tool, and never destroying content the tool does not
understand.

This skill pairs with the `stemma` umbrella, which carries the format rules and
the tool-versus-judgement boundary. Read that first if it is not already loaded.

## The operation

**The tool finds and fixes; you decide what should change.** Invariants -- a
link resolving to exactly one page, a citation key existing, a file staying
where the format puts it -- belong to the tool. Whether a page should be
archived, retitled, or split is your judgement.

## Steps

1. **Survey before touching anything.**

   ```sh
   stemma status                     # counts, orphans, uncited sources, index state
   stemma lint                       # everything wrong with the KB
   stemma lint --strict              # warnings become errors; what CI runs
   ```

2. **Find structural problems.**

   ```sh
   stemma graph --orphans            # pages nothing links to
   stemma graph --dead-ends          # pages with no outgoing links
   stemma graph "A Title" --in --depth 2    # backlinks, two hops
   stemma cite check                 # bibliography health
   ```

3. **Fix with the tool, not the filesystem.**

   - Broken or ambiguous wikilinks: read what the tool resolved with
     `stemma show`, then fix the link text or retitle the target.
   - Retitle: `stemma rename "Old Title" "New Title"` rewrites every inbound
     link. Never edit a link target by hand.
   - Move: `stemma move "A Title" dir/` keeps the page's identity. Directories
     are organisational only; moving a page never changes what links to it.
   - Archive: `stemma archive "A Title" --reason "superseded by ..."` sets the
     status and appends the reason. **The tool never deletes a page.** An
     archived page stays readable but leaves the graph.

4. **Rebuild the search index when results look stale.** Tier 0 computes on
   demand, so this is a speed and consistency step, not a prerequisite.

   ```sh
   stemma index                      # incremental refresh
   stemma index --rebuild            # discard and build from scratch
   ```

5. **Confirm the repair.** Re-run `stemma lint --strict`. Cleaning one thing
   often exposes the next; the KB is healthy when the strict run is clean.

## Preservation

The single most important invariant is that the tool never destroys what it does
not understand. It preserves unknown frontmatter fields verbatim. If a
maintenance step seems to require rewriting a file by hand, that is a signal to
stop and check whether a command already does it.

## Boundary

- Adding or rewriting content on purpose: `stemma-author`.
- Gathering or recording a source: `stemma-research`.
- This skill owns the state of the KB, not its content.

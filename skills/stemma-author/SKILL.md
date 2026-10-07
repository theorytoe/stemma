---
name: stemma-author
description: >-
  Write and edit the content of a Stemma knowledge base. Use when adding a
  page, drafting material in the inbox, choosing a page type, linking pages with
  [[wikilinks]], citing sources with [@key], promoting a draft into the pages,
  retitling a page, or moving it between directories. Operates `stemma new`,
  `promote`, `rename`, `move`, and `show`. Repairing pages that already exist is
  stemma-maintain.
license: CC-BY-NC-SA-4.0
---

# Stemma author

Authoring is producing content: a new page, a draft in the inbox, the prose that
links pages and cites sources, and the transitions that turn a draft into a page
or change where a page lives.

This skill pairs with the `stemma` umbrella, which carries the format rules and
the tool-versus-judgement boundary. Read that first if it is not already loaded.

## The operation

**The tool places files; you write words.** Choosing a filename, a directory, a
link target, or a citation key is arithmetic on the KB's structure, and the tool
owns it. You own what the page says.

## Steps

1. **Decide page or draft.** A draft goes to `inbox/`, outside the knowledge
   proper: not indexed, not searched, not exported, only leniently checked. Use
   a draft for anything unfinished.

   ```sh
   stemma new "Working Title" --draft --type note    # a draft
   stemma new "A Settled Title" --type concept       # a page
   ```

   A title that begins with a dash is still a title: put the flags first, then
   `--`, then the title -- `stemma new --type note -- "--draft"`.

2. **Give it a type from the vocabulary.** `topic`, `concept`, and `note`, plus
   whatever the manifest adds. Never assign `source` or `index` -- the tool owns
   those. An unknown type warns by default and errors under `--strict`.

3. **Write the body.** `new` made the frontmatter; you write the prose beneath
   it. Link with `[[Title]]`; cite with `[@key]`. Never edit a filename, a
   link's target, or a citation key by hand: the tool owns those, and a hand
   edit leaves the graph broken. Change a title with `rename` and a status with
   `archive`, because those keep the links and the reason consistent. The
   `type`, `aliases` and `tags` fields have no command for an existing page; set
   them in the frontmatter yourself.

   - A link resolves by title, slug, or alias. If two pages resolve one link,
     that is an error; `stemma lint` reports it and `stemma show` shows what
     resolved. Do not guess which page a link means.
   - Every `[@key]` must exist in the bibliography. Get the key from the tool
     (see the `stemma-research` skill), not from the title.

   Check a page as you write it:

   ```sh
   stemma show "A Settled Title"     # the page as the tool resolves it
   stemma lint                       # the whole KB while you work
   ```

   A page nothing links to yet is an orphan, and `--strict` treats an orphan as
   an error. Link the new page from an existing one, then re-run
   `stemma lint --strict` before you call it finished.

4. **Promote a draft when it is ready.** Everything the inbox permits becomes an
   error at promotion, so promotion is where a draft is held to the real format.

   ```sh
   stemma promote "Working Title" --type note
   ```

   Promotion validates the required fields, requires a type, resolves links that
   pointed at the draft, and moves the file into `pages/`.

5. **Change a title or a location with the tool, never by hand.** The tool
   rewrites every inbound link, which is the reason `[[wikilinks]]` are safe.

   ```sh
   stemma rename "Old Title" "New Title"
   stemma move "A Title" notes/archive
   ```

   Editing a filename or a link target directly leaves the graph broken.

## Boundary

- Deciding what an existing page should say after an audit: `stemma-maintain`.
- Finding a source or recording it: `stemma-research`.
- This skill owns creating and changing content; it does not audit or repair.

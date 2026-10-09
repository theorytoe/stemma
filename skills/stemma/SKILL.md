---
name: stemma
description: >-
  Conventions for working in a Stemma knowledge base: a directory of markdown
  pages linked with [[wikilinks]] that cite sources as [@key] from a BibTeX
  bibliography. Load this for the rules that hold on every Stemma task: the page
  format, the query-first discipline, and the boundary between what the `stemma`
  tool decides and what the agent decides. Pair it with one workflow skill:
  stemma-research, stemma-author, stemma-maintain, stemma-query, or
  stemma-publish. Never compute a page path, link target, or citation key by
  hand; ask the tool.
license: CC-BY-NC-SA-4.0
---

# Stemma

Stemma is a tool for authoring and maintaining a knowledge base (KB): a
directory of markdown pages, linked with `[[wikilinks]]`, that cite sources as
`[@key]` from a BibTeX bibliography. The `stemma` command is the whole CLI
surface, and `stemma-mcp` exposes a curated subset of the same operations to
harnesses over the Model Context Protocol; the CLI is canonical and is what
this skill spells.

## The two rules that matter most

**Read the KB before you answer.** When a question could be answered from what
the KB holds, look it up first with `stemma search` and `stemma show`. The
failure this prevents is answering from memory while a correct, cited page sits
unread on disk. Do not resolve the KB by reading or grepping raw files: the tool
resolves links and citations, and a file read is not the KB's answer. Writing a
page's own prose is the one exception, and `stemma-author` owns it.

**Never compute the KB by hand.** You never construct a page path, choose a link
target, mint or guess a citation key, or work out where a file should live.
Those are invariants, and the tool owns them. Ask the tool instead:

- a page's path: `stemma show PAGE --path`
- a citation key: `stemma cite add` mints one
- where a file belongs: `stemma new` and `stemma move` place it
- what a link resolves to: `stemma show PAGE` and `stemma lint`

A guessed filename or an invented BibTeX entry is confidently wrong in exactly
the way the invariants exist to prevent.

## Running commands

- Form: `stemma <command> [flags]`. A family takes a member, as in
  `stemma cite show KEY`.
- One invocation works on one KB. The KB is found from `--kb PATH`, else
  `STEMMA_KB`, else by walking up from the working directory to the nearest
  `stemma.toml` or `pages/`. The walk goes up only, so when the KB is not above
  your working directory, pass `--kb PATH`.
- `--json` on every command. The output is one envelope: `command`, `ok`,
  `data`, and `findings` or `error` when they apply.
- Exit codes: `0` clean, `1` validation findings, `2` operational error. Treat
  `1` as a fact about the KB and `2` as a fact about the invocation.

## The format in one screen

- A page is markdown with YAML frontmatter. `title` and `type` are required;
  `tags`, `aliases`, `status`, and unknown fields are allowed and preserved.
- `type` comes from the vocabulary: `topic`, `concept`, `note`, plus anything
  the manifest adds. `source` and `index` are the tool's own; never assign them.
- `pages/` holds every authored page; directories inside it are organisational
  only. `inbox/` holds drafts and is outside the knowledge proper: not indexed,
  not searched, not exported.
- `[[Title]]` resolves by title, slug, or alias. Two pages resolving one link is
  an error, not a guess. You never decide which page a link means; `stemma lint`
  and `stemma show` do.
- `[@key]` names a bibliography entry. Every key must exist in the bibliography.
- Generated output lives under `.stemma/` in the KB. It is never committed and
  never edited by hand.

The full specification is in `references/format.md`. The full command reference,
every verb, flag, exit code, and envelope, is in `references/cli.md`. Read the
relevant one when a command's exact form matters. If the CLI reference is absent,
`stemma help --markdown` writes the same document.

## If the tool will not run

If you cannot invoke `stemma` -- no binary, no shell, a harness that gives you
only file tools -- stop and say so. Do not fall back to reading or grepping the
page tree to approximate it. A raw read does not resolve a wikilink or a
citation, and an answer pieced together from files is not the KB's answer. Ask
for the tool, or report the blocker.

## Which workflow skill to load

Load exactly one alongside this one:

| The task is                                                    | Load              |
| -------------------------------------------------------------- | ----------------- |
| find or record an outside source, resolve a DOI/arXiv/ISBN/URL | `stemma-research` |
| write or edit pages, draft, promote, retitle                   | `stemma-author`   |
| audit, lint, repair, archive, or reorganise existing pages     | `stemma-maintain` |
| answer a question from what the KB already holds               | `stemma-query`    |
| build, serve, export, or hand the KB to someone                | `stemma-publish`  |

Do not load several at once. If a task genuinely spans two -- record a source,
then write it up -- finish the first workflow, then load the second.

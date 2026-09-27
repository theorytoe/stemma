# The Stemma format

This document is the canonical specification of the Stemma on-disk format. It is
self-contained by design: it depends on no other document, and everything it
relies on is stated here. The tool that reads and writes the format is `stemma`,
and commands appear below in the form they are invoked.

The format is documented but deliberately not versioned and not gated. There is
no version field, no schema URL, and no compatibility matrix. Breaking changes
ship as migrations.

## Vocabulary

**KB** is the corpus, the whole body of documents. **KB root** is the directory
that holds them. A KB is contiguous: there are no topic boundaries and no
sub-wikis. One invocation of the tool operates on one KB.

## The KB root

```
my-kb/
├── stemma.toml        # the manifest. Optional. The discovery marker.
├── pages/             # every authored page
├── inbox/             # drafts, outside pages/
├── sources/           # vendored source text. Absent unless asked for.
├── bibliography.bib   # the bibliography, or bibliography/ for large KBs
└── .stemma/           # generated. Never committed.
```

Nothing outside `pages/` and `inbox/` is a page. Files elsewhere in the KB root
are not read, not linted, and not exported.

### `pages/`

A single directory holds every authored page. The structure inside it is the
author's, at any depth, and the tool imposes none.

One page is the entry document: `pages/index.md`, with `type: index`. `init`
writes it and the author owns its content from then on. The renderer roots the
site at it, so it is the page every other page is reachable from.

Directories inside `pages/` are organisational only. They confer no scope, no
partitioning, no per-directory index, and no effect whatsoever on link
resolution. Moving a file between directories cannot change what any link
resolves to, and there is no path-qualified link syntax.

### `inbox/`

Drafts live in `inbox/`, inside the KB root but outside `pages/`. An inbox file
is markdown, but it is not a page and the format's rules do not apply to it. It
is not indexed, not searched, not exported, and not checked for broken links or
missing fields. It is validated only leniently, enough that a half-written file
is never an error.

A draft becomes a page when it is promoted into `pages/`, at which point every
rule below applies to it at once.

`draft` is not a status. Draft-ness is inbox membership, and nothing else.

### `.stemma/`

Generated artifacts live in `.stemma/` in the KB root: the Tier-1 index, the
extraction scratch area, and anything else the tool derives. It is gitignored
and nothing in it is ever committed. It can be deleted at any time and rebuilt
with no loss, because it is a cache and never a source of truth.

No command may require it. A KB with no `.stemma/` directory is fully usable.

### The bibliography

The bibliography is BibTeX. A single `bibliography.bib` in the KB root is the
default form; a `bibliography/` directory of `.bib` files is the equivalent form
for a KB large enough to want one. Both are first-class, and citation keys are
unique across whichever form is in use. Two entries sharing a key is a finding;
the tool does not silently pick one. Both forms may be present at once and are
then read together, so a key defined in each of them is a duplicate like any
other.

The bibliography is the source of truth for sources. Pages are the origin of
their own content: a source is cited from a page, never the reverse, and a page
that cites nothing is still a complete page.

Every source is recorded by pointer -- a DOI, an arXiv identifier, a URL, an
ISBN, or a path to a local file -- together with the date it was retrieved and a
hash of what was retrieved. Both are carried as fields on the BibTeX entry, and
they have to be, because the bibliography is committed and is the only place a
source's record can live. A source page is generated and never committed, so it
cannot be the record of anything.

| Field                 | Meaning                                                 |
| --------------------- | ------------------------------------------------------- |
| `stemma-retrieved`    | the date the source was fetched, as `YYYY-MM-DD`        |
| `stemma-content-hash` | the hash of what was fetched, as `<algorithm>:<digest>` |

These exist so that drift is detectable. Re-fetching a source and finding a
different hash is a fact worth reporting, not a silent overwrite.

Three further questions must be answerable from the bibliography and the pages
together, because they are what make it useful rather than merely well-formed:
which pages cite a given key, which cited keys are missing, and which present
keys are never cited. The bibliography also exports to BibTeX and to CSL-JSON,
and the export is validated on the way out rather than trusted.

### `sources/`

The full text of a source is not stored by default. Vendoring it is opt-in, and
when it is asked for the text goes into `sources/` in the KB root.

A vendored file is a capture rather than a generated artifact, so unlike
`.stemma/` it is committed. The directory is absent in a KB that has vendored
nothing, which is the ordinary case, so the default KB root is unchanged by its
existence. Files in `sources/` are not pages, and no rule above applies to them
beyond their location.

## Pages

A page is a single file in `pages/` with the extension `.md`, encoded UTF-8.
Files in `pages/` with any other extension are not pages and are left alone.

### Anatomy

A page is a YAML frontmatter block followed by a markdown body. A `.md` file in
`pages/` with no frontmatter block is still a page; it is simply missing
everything the format requires of one.

```markdown
---
title: Attention Is All You Need
type: concept
aliases:
  - transformer paper
---

The [[Transformer]] is defined in [@vaswani2017].
```

Frontmatter opens with a line containing exactly `---` at the very start of the
file and closes with a line containing exactly `---`. The block between them is
a YAML mapping at the top level. Everything after the closing delimiter is the
body.

The title lives in frontmatter and nowhere else. A body may open with an H1, but
that is ordinary content: it is not required, not special, not stripped, and not
reconciled against `title`.

### Frontmatter fields

Two fields are mandatory. Everything else is optional.

| Field            | Required | Value                            | Owner                                   |
| ---------------- | -------- | -------------------------------- | --------------------------------------- |
| `title`          | yes      | string                           | author                                  |
| `type`           | yes      | a type from the vocabulary       | author                                  |
| `status`         | no       | `active` (default) or `archived` | author                                  |
| `aliases`        | no       | list of strings                  | author                                  |
| `archive_reason` | no       | string                           | `archive`                               |
| `key`            | no       | string                           | tool, and only on a virtual source page |

Any other field is an **unknown field**. Unknown fields are preserved verbatim
and never interpreted. They are not an error, not a warning, and not a reason to
refuse to write a file.

There is no `tags` field, no date field, and no identifier field. A page is
identified by its title.

### Types

Three types are built in: `topic`, `concept`, and `note`. `topic` is the default
applied by `new` when no type is given.

The set is closed but extensible by configuration. A manifest's `types` list
adds to it. A type that is neither built in nor configured is unknown: a warning
by default, an error under `--strict`.

Two types are reserved and tool-owned:

- `index` — a navigation page. `new` will not create one.
- `source` — a bibliography entry. A source page is **virtual**: it is
  materialised from its BibTeX entry at export and build time, and it is never
  committed.

Reserved types are recognised by lint rather than forbidden in the file, because
the tool has no way to tell an author's edit from its own. What the format
states is weaker and honest: the tool never assigns them on an author's behalf,
and a committed page declaring `type: source` is a warning by default and an
error under `--strict`, because it contradicts the rule that source pages are
virtual.

### Status and lifecycle

`status` is `active` or `archived`, and defaults to `active` when absent. Any
other value is a warning by default and an error under `--strict`.

`archive` sets `status` to `archived` and records the reason it was given in
`archive_reason`. The tool never deletes a page.

### Filenames

A page's filename is the author's business. It carries no meaning, and no rule
in this document depends on it.

`new` defaults the filename to the page's slug plus `.md`, so that nobody has to
choose one. Choosing a different one afterwards is allowed and changes nothing.

## Names and links

### Title, slug, and alias

A page has exactly one **title**, which is its identity.

A page's **slug** is derived from its title by normalisation:

1. lowercase;
2. every run of characters that is not a letter, a digit, `+` or `#` becomes a
   single `-`;
3. strip leading and trailing `-`.

So `Attention Is All You Need` has the slug `attention-is-all-you-need`, and
`C++` has the slug `c++`. Letters and digits are kept because they are the only
characters that are universally safe; `+` and `#` are kept because dropping them
would collapse distinct real titles such as `C` and `C++` into one name, which
the format has no way to disambiguate.

An **alias** is an additional name the author declares in `aliases`.

The slug is not a second identity. It is the same name as the title, spelled in
a form that is forgiving to type. Comparing a name means comparing its
normalisation, so resolving a link "by title" and resolving it "by slug" are the
same operation, differing in appearance and not in identity.

### Wikilinks

An internal link is written `[[name]]`, where `name` is a title, a slug, or an
alias. There is no other syntax: no path, no anchor, no transclusion, and no
embed.

Link syntax applies to prose and not to code. A `[[name]]` inside an inline code
span or a fenced block is text, not a link, which is what lets a page describe
the syntax without linking to a page called `name`. A link also does not span
lines: a `[[` with no matching `]]` on its line is ordinary text.

Resolution compares normalisations. `[[Attention Is All You Need]]`,
`[[attention is all you need]]`, and `[[attention-is-all-you-need]]` are the
same link and resolve to the same page. Aliases are compared the same way.

A link resolves to the set of pages whose normalised title or normalised alias
equals the link's normalisation:

- exactly one page — resolved;
- no page — **unresolved**: a warning by default, an error under `--strict`;
- two or more pages — **ambiguous**: a hard error, in both lenient and strict
  modes. The tool cannot guess, so it refuses rather than picking.

Ambiguity is a hard error wherever it is found, but a *latent* collision — two
pages claiming the same normalised name without any link actually using it — is
a warning by default and an error under `--strict`. The distinction matters: an
unusable link stops the tool, while a collision nobody has linked to yet is
something to fix before it does.

Because names must be unique, a title that collides with another page's alias is
a collision. Adding an alias can therefore break an existing link, and the tool
reports that rather than resolving it quietly.

### Citations

A citation is written in pandoc's inline syntax:

| Form            | Meaning                             |
| --------------- | ----------------------------------- |
| `[@key]`        | parenthetical citation              |
| `[@a; @b]`      | several citations in one group      |
| `[@key, p. 33]` | with a locator                      |
| `@key`          | narrative citation                  |
| `[-@key]`       | citation with the author suppressed |

`key` is a BibTeX citation key. Citations, like links, are read from prose and
not from code, so a citation written inside a code span or a fenced block means
nothing. A key that is cited but absent from the bibliography is a warning by
default and an error under `--strict`, and a key that exists but is never cited
is the same. A key that the bibliography defines twice is the same again.

Keys are matched exactly, including case. A citation key is an identifier rather
than prose, so unlike a page title it is not folded for comparison, and
`[@Vaswani2017]` does not find an entry called `vaswani2017`.

A citation resolves to the entry's virtual source page. Wikilinks do not: a
source page is not part of the authored name space, so `[[key]]` does not
resolve to one. This is what makes it safe for source pages to be exempt from
name-collision and orphan checks — they never compete for a name.

How a citation is rendered is a presentation concern, not a format one. The
available forms are named by `citation_style` in the manifest, and the format
requires only that rendering never changes the citation's meaning or which
source it points at.

## The manifest

`stemma.toml` is the manifest. It is optional-with-defaults, and it is the
discovery marker. Unknown keys are preserved verbatim.

| Key              | Type            | Default                      | Meaning                                   |
| ---------------- | --------------- | ---------------------------- | ----------------------------------------- |
| `title`          | string          | the KB root's directory name | the KB's title                            |
| `default_type`   | string          | `topic`                      | the type `new` applies when none is given |
| `types`          | list of strings | empty                        | types added to the built-in three         |
| `citation_style` | string          | `author-date`                | a built-in formatter                      |
| `ignore`         | list of strings | empty                        | path globs, relative to the KB root       |

```toml
title = "Stemma"
default_type = "concept"
types = ["person", "project", "question"]
citation_style = "author-date"
ignore = ["pages/attic/**"]
```

`citation_style` selects from the built-in formatters and nothing else. It is
not a CSL style ID, and no CSL processor is involved.

A path matching `ignore` is outside the KB: not read, not indexed, not linted,
and not exported. It is the general form of what `inbox/` does by convention.
Patterns are written with forward slashes. `*` matches within one path segment
and `**` matches across segments, so `pages/attic/**` covers everything under
that directory while `pages/attic` covers only that directory itself.

### Discovery

One KB per invocation, resolved in this order:

1. an explicit path given on the command line;
2. the `STEMMA_KB` environment variable;
3. walking up from the working directory looking for `stemma.toml`, then for a
   directory containing `pages/`.

A manifest is preferred over a directory that merely holds `pages/`, however
near that directory is: a KB configured by hand beats one that only looks like
one. A directory holding neither is not a KB root.

If none of these finds a KB root, the command fails with exit code `2`. There is
no registry and no daemon.

## Leniency and preservation

The tool is lenient by default and `--strict` turns warnings into errors. The
only non-negotiable failure is a link that cannot be resolved to exactly one
page.

| Finding                                       | Default        | `--strict`     |
| --------------------------------------------- | -------------- | -------------- |
| missing or empty `title` or `type`            | warning        | error          |
| a field holding the wrong shape for its value | warning        | error          |
| unknown type                                  | warning        | error          |
| invalid `status`                              | warning        | error          |
| latent name collision                         | warning        | error          |
| unresolved wikilink                           | warning        | error          |
| cited key absent from the bibliography        | warning        | error          |
| duplicate citation key in the bibliography    | warning        | error          |
| bibliography key never cited                  | warning        | error          |
| orphan page, with no inbound links            | warning        | error          |
| committed page declaring `type: source`       | warning        | error          |
| `key` on a page that is not a source page     | warning        | error          |
| **ambiguous wikilink**                        | **error**      | **error**      |
| unreadable or non-UTF-8 file                  | error (exit 2) | error (exit 2) |

`index` pages are exempt from the orphan check, as are virtual source pages. A
page linking to itself does not stop it being an orphan, and `index` needs no
misuse rule of its own: an index page on disk is legitimate, and the tool simply
never writes one on an author's behalf.

A row in that table is a page the tool can still read, show and repair. Some
files are not, and they fail with exit code `2` instead:

- a file that is not valid UTF-8;
- a file that begins with a byte-order mark;
- a frontmatter block that is opened and never closed;
- a frontmatter block that is not valid YAML, that is not a mapping, or that
  defines the same key twice;
- a bibliography entry that is opened and never closed.

In each of those the tool cannot tell what it would be rewriting, so it refuses
rather than guessing. Everything else is a finding, not a refusal, and a page
with something wrong in it can still be read and repaired.

### Preservation

The single most important invariant in the format is that a writer never
destroys content it does not understand. Concretely:

- Every unknown frontmatter field survives every write, with its value intact.
- Every unknown top-level manifest key survives every write.
- The body is never rewritten except where a command's purpose requires it.
  `rename` rewrites inbound links; nothing else touches a body.
- Where a field is written, the tool edits the frontmatter block in place rather
  than re-emitting it. Key order, comments, and quoting style outside the edited
  field are therefore not disturbed, and a page the tool has nothing to change
  comes back byte-identical.
- The tool never deletes a page.

Round-trip fidelity is a tested property, not an aspiration.

Exit codes are `0` for a clean run, `1` when lint reports findings, and `2` for
an operational failure. Every command supports `--json`.

## What the format is not

- Not versioned, and not gated by a version field.
- Not path-aware: no path-qualified links, no directory scope, no per-directory
  indexes or exports.
- Not anchor-aware: `[[page#heading]]` is not link syntax in this version.
- Not transclusion: there are no embeds.
- Not CSL: `citation_style` names a built-in formatter, not a style ID.
- Not a converter: importing an existing wiki is not a feature and is not
  planned.
- Not dependent on git. Every command works in a plain directory, and history
  features switch on when a repository happens to be present.
- Not a substitute for git history. Because history is optional, a page is
  expected to carry the reasoning for its own changes in its own text. A commit
  log records what changed and when; it does not record why.
- Not required to be readable raw. Preview and export are the human interface,
  so the format is free to favour fidelity over how the files look on their own.

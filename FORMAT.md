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

Two of the things inside it are worth naming, because commands put them there and
a person may want to read them:

- `.stemma/shim/extract.py` — the extraction script. `init` writes it and the tool
  rewrites it whenever it does not match the binary, so an edit there is lost;
  the script is changed in the repository.
- `.stemma/fetched/<name>-<digest>.txt` — text that `fetch` read, named after the
  source it came from. Both halves are derived from the pointer, so fetching one
  source twice lands on the same file and two sources cannot land on one
  (`--clear` empties the directory and leaves the rest of `.stemma/` alone).

The fetched text is a cache and never a record. What a source *is* lives in the
bibliography, with its pointer, its retrieval date and its content hash; what it
*says* lives here until someone clears it.

### The bibliography

The bibliography is BibTeX. A single `bibliography.bib` in the KB root is the
default form; a `bibliography/` directory of `.bib` files is the equivalent form
for a KB large enough to want one. Both are first-class, and citation keys are
unique across whichever form is in use. Two entries sharing a key is a finding;
the tool does not silently pick one. Both forms may be present at once and are
then read together, so a key defined in each of them is a duplicate like any
other.

A KB starts with the single file. The directory form is worth switching to when
one file stops being reviewable — when several people edit the bibliography, or
when every change touches a file everyone else also has open and merges conflict
on. There is no threshold, because the cost is conflict and not size.

A new entry is written to `bibliography.bib` whenever that file exists, so both
forms together are a migration in progress and new work joins the default file.
A KB with only the directory form writes one file per entry, named from the key
(`smith-2020-widgets.bib`), with a `-2` suffix when another key already owns that
name. An update always goes to the file the entry is already in, so a form in
transition never moves an entry on its own.

The bibliography is the source of truth for sources. Pages are the origin of
their own content: a source is cited from a page, never the reverse, and a page
that cites nothing is still a complete page.

Every source is recorded by pointer — a DOI, an arXiv identifier, a URL, an
ISBN, or a path to a local file — together with the date it was retrieved and a
hash of what was retrieved. Both are carried as fields on the BibTeX entry, and
they have to be, because the bibliography is committed and is the only place a
source's record can live. A source page is generated and never committed, so it
cannot be the record of anything.

A pointer is compared by what it names rather than by how it was typed. A DOI is
one DOI whether it is written bare, as a resolver URL, or with a query string; an
ISBN is one ISBN with or without its dashes; an arXiv identifier names one paper
with or without its version, even though the version is what gets fetched. Two
entries that point at one work are one work, and the check says so rather than
leaving a person to notice.

| Field                  | Meaning                                                       |
| ---------------------- | ------------------------------------------------------------- |
| `stemma-retrieved`     | the date the source was fetched, as `YYYY-MM-DD`              |
| `stemma-content-hash`  | the hash of what was fetched, as `<algorithm>:<digest>`       |
| `stemma-vendored-hash` | the hash of the captured text in `sources/`, in the same form |

The first two are on every entry the tool has fetched a record for. The third is
optional: it is there exactly when the source's full text has been vendored, and
its value is the hash of what is in `sources/`.

These exist so that drift is detectable. Re-fetching a source and finding a
different hash is a fact worth reporting, not a silent overwrite.

Three further questions must be answerable from the bibliography and the pages
together, because they are what make it useful rather than merely well-formed:
which pages cite a given key, which cited keys are missing, and which present
keys are never cited.

The bibliography exports to BibTeX and to CSL-JSON. BibTeX is the KB's own form,
so an export can be handed to a reference manager; CSL-JSON is what a citation
processor reads, so the same records can be formatted by a tool that knows CSL
without the KB becoming one. `--cited` narrows either export to the records some
page actually relies on.

The BibTeX export is self-contained: every entry is re-rendered with its values
resolved, so a macro a source file defined is written out in full and the result
needs nothing from the KB it came from. The CSL-JSON export maps the fields CSL
names and carries the rest through under their own name, so the retrieval date
and the content hash survive the trip; the retrieval date becomes CSL's
`accessed`, which is what it is.

Export is a rendering, not a rewrite. It never edits the KB and it loses no
record: an entry type CSL has no name for becomes a plain document rather than
being dropped. What the tool emits is checked by a reader other than the one
that wrote it, because an export that nothing else can open is not an export.

### `sources/`

The full text of a source is not stored by default. Vendoring it is opt-in, and
when it is asked for the text goes into `sources/` in the KB root.

A vendored file is a capture rather than a generated artifact, so unlike
`.stemma/` it is committed. The directory is absent in a KB that has vendored
nothing, which is the ordinary case, so the default KB root is unchanged by its
existence. Files in `sources/` are not pages, and no rule above applies to them
beyond their location.

The file is named from the key it belongs to, `sources/<name>-<digest>.txt`, so
one source always lands on one file. Both halves are derived from the key, which
is what the digest is for: two keys that reduce to one filename, such as
`smith:2020` and `smith-2020`, would otherwise have to share a capture. The name
is derived rather than stated because the entry records the capture's hash and
not its path, so the key is what tells you where to look.

Capturing is the one thing a bibliography cannot do for itself: an address goes
stale and a capture does not. It is therefore explicit, and it checks itself. When
a capture and its recorded hash disagree — a hand-edit, a bad merge, a download
that stopped early — `cite check` reports it and neither side is corrected, because
the tool cannot know which of the two someone meant and overwriting either would
destroy the evidence of what happened. Replacing a capture with different text
takes `--force`, and the text is only written once all of it has been read: a
capture that stops at a size limit would be a capture of something else.

## Tiers and scale

Retrieval runs in two tiers. Tier 0 is the KB itself: pages are read and
resolved in memory, with no setup and nothing generated. Tier 1 is the SQLite
index under `.stemma/`, built by `stemma index`. Every command uses the index
when one exists and is fresh and falls back to Tier 0 otherwise, and no command
requires it.

The index is never a source of truth. It is stamped with each page's content
hash, so it is checked by content rather than by timestamp: a file touched
without changing does not invalidate it, and a file changed on a machine with a
wrong clock does. That check reads every page's bytes to hash them — cheaper
than parsing, but not free — so the index saves the parse and the tokenization,
not the read.

How far Tier 0 scales was measured rather than guessed. `make bench` runs the
benchmark behind these numbers, one iteration per size; re-run it on the machine
that matters. `search` and `graph` are the commands end to end — the fixed cost a
run pays (loading the KB, or checking the index) plus the work itself; graph is a
one-hop walk in both directions from a page:

| pages  | lint   | resolve | search, Tier 0 | search, Tier 1 | graph, Tier 0 | graph, Tier 1 | build the index |
| ------ | ------ | ------- | -------------- | -------------- | ------------- | ------------- | --------------- |
| 100    | 0.2 ms | 7 µs    | 3.4 ms         | 2.4 ms         | 21 µs         | 0.22 ms       | 19 ms           |
| 1,000  | 2.4 ms | 9 µs    | 35 ms          | 26 ms          | 47 µs         | 0.23 ms       | 0.19 s          |
| 10,000 | 31 ms  | 14 µs   | 0.35 s         | 0.23 s         | 0.36 ms       | 0.55 ms       | 2.7 s           |

Measured on a 2020 six-core laptop; read the numbers as orders of magnitude.
Together they answer `U1`: resolution is O(1) and never a reason to build an
index; lint is linear and still comfortable at ten thousand pages; and a Tier-0
search is comfortable to roughly a thousand pages, after which it costs a few
hundred milliseconds. The index cuts a search by about a third — the freshness
check still reads and hashes every page, and ranking still re-tokenizes each
candidate — so a build pays for itself after roughly twenty searches, at a
thousand pages and at ten thousand alike. Whether that is worth it depends on
how often the KB is queried between edits, not on how large it is. An orphan
listing is 0.4 ms / 3.5 ms / 36 ms on the index and 69 µs / 0.67 ms / 9.8 ms on
the KB; the index answers it from the whole backlink relation at once.

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
| `tags`           | no       | list of strings                  | author                                  |
| `archive_reason` | no       | string                           | `archive`                               |
| `key`            | no       | string                           | tool, and only on a virtual source page |

Any other field is an **unknown field**. Unknown fields are preserved verbatim
and never interpreted. They are not an error, not a warning, and not a reason to
refuse to write a file.

There is no date field and no identifier field. A page is identified by its
title.

### Tags

`tags` is optional and holds free-form labels:

```yaml
tags: [transformers, attention, architecture]
```

A tag is not a name. It takes no part in link resolution and no link resolves to
one: a page is identified by its title alone. A tag says what a page is *about*;
it does not say what the page is *called*.

There is no vocabulary of tags and no manifest key that declares one. This is the
opposite of `type`, and it is deliberate: a type is a closed set the tool reasons
about, while a tag is a judgement the author makes, and the author owns anything
requiring judgement. A tag is never unknown, because there is nothing for it to
be unknown to.

Tags are compared by their normalised form, so `Machine-Learning`,
`machine learning` and `machine_learning` are one tag; the spelling the author
wrote is what is kept. Listing the same tag twice, by any spellings that
normalise the same, says the same thing twice and is a warning by default and an
error under `--strict`. A tag filter, a per-tag index and a per-tag count are all
built on this and nothing else in the format depends on tags.

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

A title has to leave a slug behind. A title that normalises to nothing — only
punctuation, or only whitespace — names nothing, and a name nothing can point
at is not a name: such a title is treated as a missing one.

The slug is not a second identity. It is the same name as the title, spelled in
a form that is forgiving to type. Comparing a name means comparing its
normalisation, so resolving a link "by title" and resolving it "by slug" are the
same operation, differing in appearance and not in identity.

A title can be changed, and doing so retires the old one: the page stops
answering to it. The old name is not quietly kept as an alias, because two names
for one page is the ambiguity this section exists to avoid. Every link that
named the page by its title is rewritten to the new one, since it would
otherwise stop resolving. A link that named the page by an alias is left alone:
the alias still names it, and rewriting working prose would be editing something
that was not broken.

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

Two styles are built in. `citation_style` names one of them, and nothing else.

| Style         | In text                      | Reference list                          |
| ------------- | ---------------------------- | --------------------------------------- |
| `author-date` | `(Bush 1945)`, `Bush (1945)` | `Bush, Vannevar. 1945. As We May Think. The Atlantic Monthly, 176(1): 101--108.` |
| `numeric`     | `[1]`, `[1; 2]`              | `[1] Bush, Vannevar. As We May Think. The Atlantic Monthly, 176(1): 101--108. 1945.` |

Both styles render all five inline forms. A locator becomes `, p. 33` inside the
label, a narrative citation puts the author in the prose (`Bush (1945)`), a
suppressed author drops the name (`(1945)`), and a group renders as one
bracketed list. `numeric` numbers a key by its first citation on the page.

There is no CSL. A CSL style identifier is not a `citation_style`, no CSL
processor is involved, and a style this tool does not have is an error rather
than a silent fallback. Those two styles are the whole vocabulary.

## The manifest

`stemma.toml` is the manifest. It is optional-with-defaults, and it is the
discovery marker. Unknown keys are preserved verbatim.

| Key              | Type            | Default                      | Meaning                                   |
| ---------------- | --------------- | ---------------------------- | ----------------------------------------- |
| `title`          | string          | the KB root's directory name | the KB's title                            |
| `description`    | string          | empty                        | a sentence or two about the KB            |
| `default_type`   | string          | `topic`                      | the type `new` applies when none is given |
| `types`          | list of strings | empty                        | types added to the built-in three         |
| `citation_style` | string          | `author-date`                | a built-in formatter: `author-date` or `numeric` |
| `ignore`         | list of strings | empty                        | path globs, relative to the KB root       |

One table holds the export family's defaults. `default_depth` is how many hops a
scoped export takes when the command line does not say. It is `1` unless the
manifest changes it; `0` means no limit, so the export is the whole reachable
set. `--depth all` on the command line means the same thing.

| Key                    | Type    | Default | Meaning                                     |
| ---------------------- | ------- | ------- | ------------------------------------------- |
| `export.default_depth` | integer | `1`     | hops a scoped export takes; `0` is no limit |

```toml
title = "Stemma"
description = "Notes on attention and architecture."
default_type = "concept"
types = ["person", "project", "question"]
citation_style = "author-date"
ignore = ["pages/attic/**"]

[export]
default_depth = 1
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
| a `title` that names nothing                  | warning        | error          |
| a field holding the wrong shape for its value | warning        | error          |
| unknown type                                  | warning        | error          |
| invalid `status`                              | warning        | error          |
| `tags` listing the same tag twice             | warning        | error          |
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

`index` pages are exempt from the orphan check, as are virtual source pages and
archived pages. A page linking to itself does not stop it being an orphan, and
`index` needs no misuse rule of its own: an index page on disk is legitimate,
and the tool simply never writes one on an author's behalf.

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
- A write either happens or does not. The new bytes go to a temporary file
  beside the target and are renamed into place, so an interrupted or failed
  write leaves the page exactly as it was rather than half of a new one.
- The tool never deletes a page, and never replaces one without being asked to.

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
- Not a reader of every kind of document. `fetch` reads PDFs and HTML pages and
  refuses the rest by name: an EPUB is a zip container the tool does not open, a
  scanned page has no text layer to read, and a page fetched from an address whose
  text only exists once JavaScript has run has nothing on the server to read. Each
  is reported as itself — `unsupported` for the EPUB, `empty` for the other two —
  rather than handed back as an empty document, because "there is nothing there"
  and "there is nothing I can read" are different facts and a reader has to know
  which one happened. A file that *is* text is read as text whatever it is called,
  so fetching a local `.html` file returns its markup as it stands rather than a
  rendering of it.
- Not dependent on git. Every command works in a plain directory, and history
  features switch on when a repository happens to be present.
- Not a substitute for git history. Because history is optional, a page is
  expected to carry the reasoning for its own changes in its own text. A commit
  log records what changed and when; it does not record why.
- Not required to be readable raw. Preview and export are the human interface,
  so the format is free to favour fidelity over how the files look on their own.

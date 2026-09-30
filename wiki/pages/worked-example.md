---
title: A first knowledge base, end to end
type: concept
aliases:
  - worked example
  - first kb
tags: [workflow]
---
One small KB, built from nothing, with the commands and the output they really
produce. It is the shortest path from an empty directory to a built site, and
every command here is one the rest of this wiki names in the abstract.

**Start a root.** `stemma init` writes the whole root at once — the manifest, an
entry document at `pages/index.md`, a `bibliography.bib`, an `inbox/`, and the
extraction shim:

```
$ stemma init demo --title "Demo KB"
demo: created a KB root titled "Demo KB"
```

```
demo/
├── bibliography.bib
├── inbox/
├── pages/index.md
├── .stemma/shim/extract.py
└── stemma.toml
```

Everything under `.stemma/` is generated. See [[Structure]] for what each part is
for.

**Add pages.** `stemma new` writes the file, taking the filename from the title
and the type from `--type` or the manifest. `--draft` writes into the inbox
instead:

```
$ stemma new "Vector search" --type concept
pages/vector-search.md
$ stemma new "Retrieval" --type concept
pages/retrieval.md
$ stemma new --draft "Half-written note"
inbox/half-written-note.md
```

A new page carries a title and a type and nothing else. The tool never writes
prose, so the next step is an editor.

**Write.** Two things connect pages, and both are written inline: `[[name]]`
names another page, and `[@key]` names a source. The entry document is where
links start, because it is the page the rest is reachable from:

```markdown
---
title: Demo KB
type: index
---
Start at [[Vector search]] and [[Retrieval]].
```

```markdown
---
title: Vector search
type: concept
---
Vector search ranks pages by meaning rather than by matching words. The idea
comes from the transformer architecture [@vaswani2017]. See [[Retrieval]].
```

**Add the source the citation names.** `stemma cite add` resolves a DOI, an arXiv
identifier, a URL or an ISBN, or takes the fields by hand. `--key` names the
entry, and it is the key that `[@...]` uses:

```
$ stemma cite add --key vaswani2017 \
    --title "Attention is all you need" --author "Vaswani" --year 2017
added vaswani2017 in bibliography.bib
```

See [[Citations]] for the syntax and the bibliography.

**Check.** `stemma lint` reports everything wrong. A page nothing links to is an
orphan, which is a warning rather than an error:

```
$ stemma lint
pages/vector-search.md: warning: no page links here
stemma: 1 finding: 1 warning, 0 errors
```

`--strict` turns warnings into errors, which is what a gate uses. With the links
in place the same check is clean:

```
$ stemma lint --strict
clean
```

**Look at one page.** `stemma show` resolves a page's links and citations, so it
is the quickest way to see whether what you wrote points where you meant:

```
$ stemma show "Vector search"
pages/vector-search.md

title   Vector search
type    concept
status  active

Vector search ranks pages by meaning rather than by matching words. The idea
comes from the transformer architecture [@vaswani2017]. See [[Retrieval]].

links
  [[Retrieval]] -> pages/retrieval.md

citations
  [@vaswani2017] -> bibliography.bib
```

**Summarise and build.** `stemma status` gives the whole KB at a glance, and
`stemma build` writes the site into `.stemma/site`:

```
$ stemma status
kb        Demo KB
root      demo
pages     3
          2 concept
          1 index
orphans   0
ambiguous 0
sources   1 (0 uncited)
drafts    1
index     absent

$ stemma build
wrote 13 files to demo/.stemma/site
```

The draft is still in the inbox: it is not a page until `stemma promote` moves
it, which is also where it stops being invisible to `lint`. See [[Drafts]].
Nothing generated was written into the KB outside `.stemma/`, and [[The command
surface]] names every verb used here.

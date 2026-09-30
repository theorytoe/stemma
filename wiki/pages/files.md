---
title: Files outlive their tools
type: principle
aliases:
  - durability
  - file over app
tags: [design]
---
A program is replaced. The files it wrote are not, if the format lets them be
read without it. That is the argument Ango makes for putting files before apps
[@ango2023], and it is the bet a KB has to make if it is meant to outlast the
tool that keeps it.

So a KB is a directory of markdown, a BibTeX bibliography, and nothing else that
matters. No database owns the text, no application holds the index, and nothing
in the format is defined by the program that reads it. The tool can be deleted
and the KB is still there, because it was never the tool's.

Everything generated is disposable by design. `.stemma/` can be removed and is
never committed, and the rendered site is a view of the files rather than their
home. See [[The rendered site]].

The test is whether a person who has never installed the tool can read the KB.
They can: it is text, and the only structure in it that needs explaining is
which fields mean something and how a name is written. See [[The format]] and
[[Why Stemma]].

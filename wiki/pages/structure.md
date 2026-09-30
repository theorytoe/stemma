---
title: Structure
type: concept
tags: [format]
---
A knowledge base (KB) root holds everything, and almost all of it is yours:

```
my-kb/
├── stemma.toml        # the manifest. Optional.
├── bibliography.bib   # the sources pages cite. BibTeX.
├── pages/             # every authored page
├── inbox/             # drafts, outside pages/
├── sources/           # vendored source text, when asked for. Committed.
└── .stemma/           # generated. Never committed.
```

`pages/` is the only place pages live, and directories inside it are yours to
arrange. They carry no meaning: no directory confers scope, no directory is
indexed separately, and moving a page between directories changes no link
anywhere. See [[The format]] for what goes inside one.

`bibliography.bib` is where the sources live, and `sources/` holds a source's
full text only when vendoring was asked for; both are described in
[[Citations]]. `sources/` is absent in a KB that has vendored nothing, which is
the ordinary case.

`.stemma/` holds generated things, is gitignored, and can be deleted at any
time. No command needs it.

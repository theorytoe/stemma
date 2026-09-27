---
title: Structure
type: concept
---
A KB root holds everything, and almost all of it is yours:

```
my-kb/
├── stemma.toml        # the manifest. Optional.
├── pages/             # every authored page
├── inbox/             # drafts, outside pages/
└── .stemma/           # generated. Never committed.
```

`pages/` is the only place pages live, and directories inside it are yours to
arrange. They carry no meaning: no directory confers scope, no directory is
indexed separately, and moving a page between directories changes no link
anywhere. See [[The format]] for what goes inside one.

`.stemma/` holds generated things, is gitignored, and can be deleted at any
time. No command needs it.

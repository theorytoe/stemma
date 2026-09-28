---
title: The format
type: concept
aliases:
  - format specification
---
A page is a markdown file with a small YAML frontmatter block: a title, a type,
and whatever else you want. Two fields are required and everything else is
optional.

```markdown
---
title: Why Stemma
type: principle
---
prose, and links like [[Structure]]
```

The few things the tool gives meaning to are fixed:

- `title` and `type` are required.
- `status` is `active` or `archived`. It defaults to `active`.
- `aliases` lists other names the page answers to.
- `tags` lists free-form labels. They group pages for filtering and indexes,
  and they never act as names.
- `archive_reason` records why a page was retired.

Anything else is yours. An unknown field is preserved verbatim and never
interpreted, which means the format can carry your own structure without the
tool knowing what it means. See [[Structure]] for where pages live, and
[[Wikilinks]] for how they name each other.

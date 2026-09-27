---
title: Wikilinks
type: concept
---
One page names another by writing its title in double brackets:

```markdown
See [[Structure]] for where pages live.
```

What is inside the brackets is a name, not a path. `[[Structure]]`,
`[[structure]]` and `[[  structure  ]]` are the same link, because a name is
compared by folding case and collapsing punctuation. A page can also be named
by an alias, so [[format specification]] reaches the same page as
[[The format]].

Names have to be unique across the whole KB, and a link matching two pages is a
hard error rather than a guess. That is why the examples above are written in
code: a link inside a code span or a fenced block is text and not a link, which
is how this page can show the syntax without linking to a page called
`Structure`.

Retitling a page rewrites every link that named it by its title. Links that
named it by an alias are left alone, because they still work.

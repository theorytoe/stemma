---
title: Search and ranking
type: concept
aliases:
  - retrieval
  - search
tags: [cli]
---
Retrieval runs in two tiers. Tier 0 reads the KB itself: load, resolve, rank,
with no setup and nothing generated. Tier 1 is an index under `.stemma/`, built
by `stemma index`, and a command reads it when it is fresh and the KB when it is
not. Nothing requires the index, and nothing breaks without it.

Freshness is by content, not by timestamp. The index is stamped with a hash of
the pages, so editing a file invalidates it and touching a file without changing
it does not. That check reads every page, so the index saves parsing and ranking
rather than reading.

Both tiers share one tokenizer and one ranking function, so the same query scores
the same in either. `stemma search` ranks the pages and `stemma graph` walks the
links; the server answers its own search with the same code, which is why the
form needs no JavaScript. See [[Serving the site]].

Matching is lexical. A query finds the terms it contains, not the meaning behind
them: there is no embedding model and no learned ranker, so two passages that say
one thing in different words do not find each other. Semantic retrieval, built on
the transformer line of work [@vaswani2017], brings a model, a runtime and a
download that this project's self-contained, offline build will not carry. The
tool trades that recall for being one binary that needs nothing else. See
[[Creating and maintaining a KB]].

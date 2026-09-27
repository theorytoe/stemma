---
title: The old sharing model
type: note
status: archived
archive_reason: superseded once distribution turned out to need no tooling
review:
  state: closed
  notes:
    - kept because the reasoning is worth more than the decision
    - bundling was going to be a subsystem; it became one command
---
This page argued for topic bundles: named subsets of a KB, cut along subject
lines, each with its own manifest and its own export.

It was wrong, and the reason is worth keeping. A knowledge base is contiguous —
there are no topic boundaries in it — so a bundle would have been a second
almost-KB with its own rules about links crossing its edge. Meanwhile
distribution turned out to need nothing: a KB is a directory, so `git clone` or
`tar` already works, and the one genuinely useful case, handing someone a
subset, is [[Drafts]]-style extraction rather than a new artifact.

The page is kept rather than deleted. The tool does not delete pages, and a
record of a design that was abandoned, with the reason, is more useful than a
clean history that has forgotten it.

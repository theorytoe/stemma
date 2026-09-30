# Project context

Working context for the Stemma project. Tracked alongside the code: the plan,
the delegates, and the design record are all part of the repository.

## Files

- `planfile.md` — top-level planfile. This project is multi-tiered; its tasks are
  the delegate plans in `delegates/`.
- `delegates/` — one delegate plan per component, each in the planfile template.
- `design/decisions.md` — the decision register. Principles `P1..P10`, decisions
  `D1..D72`, the naming collision log, and the residual unknowns `U1..U6` (`U1` and
  `U5` since resolved).
- `design/qa-session.md` — the full design interview, nine rounds, including the
  questions that were corrected and the reversals they caused.
- `design/render-export-changes.md` — the edit trail for Task 6: every change it
  makes to code authored by Tasks 1–5, recorded as it happens.
- `design/skills-changes.md` — the edit trail for Task 7: every change it makes to
  code authored by Tasks 1–6, recorded as it happens.
- `design/docs-changes.md` — the edit trail for Task 9: every change it makes to
  code authored by Tasks 1–8, recorded as it happens.
- `design/shim-contract.md` — the Go-to-Python interface: how the extraction script
  is invoked, the JSON object it prints, its failure classes, its limits, and where
  the script lives.
- `plans/mcp/planfile.md` — the MCP server's own plan, moved out of the top-level
  Task 8. It is a single-tier plan, so it has no delegates of its own.

## Reading order

1. `design/decisions.md` — what was decided and why.
2. `planfile.md` — the shape of the work.
3. The relevant delegate in `delegates/` — the detail for one component.
4. `design/qa-session.md` — only when the rationale for a decision is unclear.
5. `design/render-export-changes.md`, `design/skills-changes.md` and
   `design/docs-changes.md` — before touching a file a later task did not author,
   to see whether an earlier change already moved it.

## Delegates

- `delegates/foundation.md` — format, parser, resolver, lint, core CLI verbs.
- `delegates/cli-surface.md` — dispatch, discovery, manifest, remaining commands.
- `delegates/sources-bibliography.md` — BibTeX, identifier resolution, citations.
- `delegates/source-extraction.md` — Python shim, PDF and URL text extraction.
- `delegates/index-retrieval.md` — SQLite FTS5 Tier-1 index, search, graph.
- `delegates/render-export.md` — renderer, serve, build, JSON dump, scoped extract.
- `delegates/skills.md` — umbrella skill, five workflow skills, disjointness review.
- `delegates/mcp.md` — superseded by `plans/mcp/planfile.md`; kept for its original
  curation reasoning.
- `delegates/docs.md` — example wiki as documentation, FORMAT.md, README, CI gates.

# Stemma — Render and export: changes to prior code

**Scope.** Task 6 (`.ctx/delegates/render-export.md`) may need to change code
authored by Tasks 1–5. Every such change is recorded here, as it is made. This is
an edit trail, not a design document: intent lives in `decisions.md`, the work
lives in the delegate. An entry here does not authorise a change on its own. If a
change alters an interface another delegate depends on, it is raised as a decision
(`D<n>`) and recorded in the register as well.

## What goes here

- Edits to files first authored by Tasks 1–5: `internal/kb`, `internal/cli`,
  `internal/index`, `internal/source`, `internal/extract`, `internal/citestyle`,
  `internal/deps`, and `FORMAT.md` where it describes settled behaviour.
- Additions that move a settled shape: a new hook on the shared resolver, a new
  field on the page model, a new registered command, an import that retires a
  pinned no-op in `internal/deps/deps.go`.
- Deletions or renames of anything the earlier tasks put in place.

Not recorded here: new files owned only by this task (`internal/render`, the site
templates and assets) and tests for this task's own code. Those belong to the
delegate.

## Entry format

    ### <file or area> — <short title>

    - Date — YYYY-MM-DD
    - Prior task — the task that authored the code being changed
    - What changed —
    - Why —
    - Interface impact — none, or the shape that moved and who consumes it
    - Tests — what now covers it
    - Decision ref — D<n>, or none

The top-level planfile constraint applies throughout: "an interface change that
another delegate depends on is a decision to raise rather than a step to take
quietly."

## Entries

### internal/kb — one scan for links and citations

- Date — 2026-09-29
- Prior task — 1 (foundation), 3 (sources and bibliography)
- What changed — `Page.Inlines()` added in a new `inline.go`, returning every
  wikilink and citation group with the byte range it occupies in `Body()`.
  `Links()` and `Citations()` are now thin readers of it. Internally
  `scanCitationGroups` returns group spans, replacing `parseCitations`;
  `findWikilinks` and `proseRuns` were removed as dead.
- Why — blackfriday has no inline extension hook, and it splits `[[a *b* c]]`
  across three nodes, so a renderer that scanned the parsed tree would miss a
  link lint counts. Sharing the scan is what makes "agree by construction"
  literal rather than a promise.
- Interface impact — new exported `Inline` and `Page.Inlines()`. `Links()` and
  `Citations()` keep their output contract; the existing kb tests pass unchanged,
  and no other package used the removed helpers.
- Tests — `internal/kb/inline_test.go` covers spans, code exclusion, the
  emphasis case, and group numbering across runs.
- Decision ref — none

### internal/kb — per-page findings

- Date — 2026-09-29
- Prior task — 1 (foundation), 3 (sources and bibliography)
- What changed — `KB.FindingsFor(path, mode)` added. The link loop in
  `Graph.Findings` moved to `Graph.linkFindings(path, soft)` and the
  missing-citation loop in `Graph.CitationFindings` to
  `Graph.missingCitationFindings(path, b, soft)`. `softSeverity(mode)` replaced
  three copies of the warning-or-error switch.
- Why — a renderer needs one page's findings without linting the whole KB;
  calling `Lint` per page would be quadratic (`P7`). Reading them from the same
  helpers is what makes render and lint report the same text and severities.
- Interface impact — new exported `KB.FindingsFor`. `Graph.Findings` and
  `Graph.CitationFindings` keep their output. `CitationFindings` now returns no
  uncited or duplicate findings when the bibliography is nil, where it previously
  dereferenced nil; `Load` never produces a nil bibliography.
- Tests — `internal/render` `TestFindingsMatchLint` compares render findings with
  the lint subset for one page, under both modes.
- Decision ref — none

### internal/citestyle — a citation group in pieces

- Date — 2026-09-29
- Prior task — 3 (sources and bibliography)
- What changed — `Piece` and `Layout` added, and `Formatter` gains
  `Layout(group, refs) Layout`. Both styles implement it, and `Cite` is now
  `Layout(...).Text()`.
- Why — `D69` links each citation key to its source page, which needs the
  rendered label split per key. The formatter should not know about HTML, so it
  hands back pieces and the renderer wraps them.
- Interface impact — `Formatter` gains a method. No other implementer exists in
  the tree, so nothing else breaks; `Cite` output is unchanged.
- Tests — `TestLayoutMatchesCite` asserts `Layout().Text() == Cite()` and that
  each piece names its key, for both styles.
- Decision ref — D69

### internal/deps — the blackfriday pin removed

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — `internal/deps/deps.go` deleted, the whole package with it.
- Why — it existed only to hold `blackfriday/v2` in `go.mod` until a task
  imported it. `internal/render` imports it now, so the no-op import and its
  build tag are dead weight and the file's comment is no longer true. `go.mod`
  and `go.sum` are unchanged by the removal.
- Interface impact — none; nothing referenced the package or the `stemma_deps`
  build tag.
- Tests — `make check` builds and tests every package.
- Decision ref — none

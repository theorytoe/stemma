# Stemma — Documentation and CI: changes to prior code

**Scope.** Task 9 (`.ctx/delegates/docs.md`) writes documentation, but exercising
the authoring workflow and closing the residual unknowns led to changes in code
earlier tasks authored. Every such change is recorded here, as it is made.

## What goes here

- Edits to files first authored by Tasks 1–8: `internal/kb`, `internal/index`,
  `internal/cli`, `internal/render`, `internal/export`, `internal/citestyle`,
  `FORMAT.md`, and the `Makefile`.
- A change that moves a settled shape.

Not recorded here: new files owned only by this task (`wiki/pages/*`) and edits to
this task's own deliverables (the README), which the delegate covers directly.

## Entry format

    ### <file or area> — <short title>

    - Date — YYYY-MM-DD
    - Prior task — the task that authored the code being changed
    - What changed —
    - Why —
    - Interface impact — none, or the shape that moved and who consumes it
    - Tests — what now covers it
    - Decision ref — D<n>, or none

## Entries

### internal/cli — a family member's help names its family

- Date — 2026-09-29
- Prior task — 2 (CLI surface)
- What changed — `commandHelp` takes the full invocation name instead of deriving
  the usage line from `c.name`, so `stemma cite add --help` prints
  `usage: stemma cite add` rather than `usage: stemma add`. The two call sites —
  the `help` command and dispatch's `--help` — pass the name they already hold.
- Why — found by writing the worked example and running the commands it shows. The
  generated markdown reference already had it right, because it builds the name
  from the family and the member; only the runtime help dropped the family, so the
  two surfaces disagreed about the same verb.
- Interface impact — the text of `--help` for a family member changes; the JSON
  help surface is unchanged.
- Tests — `TestCommandHelpForAFamilyMemberNamesItsFamily`; the existing family-help
  test passes the family's own name.
- Decision ref — none

### internal/kb and internal/citestyle — numeric is the default citation style

- Date — 2026-09-29
- Prior task — 1 (foundation) and 4 (sources and bibliography)
- What changed — `kb.DefaultCitationStyle` is `numeric` rather than `author-date`,
  and `citestyle.Parse("")` returns the numeric formatter. Both styles are kept.
- Why — closing `U3`, the question of which styles the built-in formatter should
  cover. The author's answer is that both are covered and numeric is the default.
- Interface impact — a knowledge base (KB) whose manifest does not name a
  `citation_style` now renders numeric citations. `stemma init` writes
  `citation_style = "numeric"`. `FORMAT.md`'s field table records the new
  default. The example wiki no longer names a style, so it exercises the
  default; the renderer's own golden is pinned to author-date instead, so a
  manifest default cannot silently rewrite it.
- Tests — `internal/citestyle`'s parse test asserts the empty name is numeric;
  `internal/export`'s golden now records `numeric`; the site gate builds the
  example wiki, so the default is exercised end to end.
- Decision ref — `U3` in `design/decisions.md`

### internal/render — the golden is pinned to one style

- Date — 2026-09-29
- Prior task — 6 (render and export)
- What changed — the render tests' fixture sets `CitationStyle` to `author-date`
  rather than taking the manifest default.
- Why — the fixture used `kb.DefaultManifest`, so changing the default silently
  changed what the renderer's golden was asserted to produce. A renderer test
  should say which style it is about.
- Interface impact — none; test-only.
- Tests — the existing golden and `TestCitationsLinkEachKey` now stand on an
  explicit choice.
- Decision ref — `U3`

### FORMAT.md — the documented citation-style default

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — the manifest field table gives `citation_style`'s default as
  `numeric`.
- Why — the default moved (above), and the specification is where a default is
  stated.
- Interface impact — none beyond the default itself.
- Tests — none; the specification is prose.
- Decision ref — `U3`

### Makefile — the comment that promised a move that will not happen

- Date — 2026-09-29
- Prior task — 1 (foundation) and 7 (skill suite)
- What changed — the `skills` target's comment no longer says the format reference
  is a copy "until docs.md moves it into the wiki".
- Why — Task 2 was resolved by keeping `FORMAT.md` the canonical, self-contained
  specification, so the reference stays a copy of it and no move is coming. The
  comment described a plan that had been abandoned.
- Interface impact — none; comment only.
- Tests — none.
- Decision ref — none

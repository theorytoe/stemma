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

_None yet._

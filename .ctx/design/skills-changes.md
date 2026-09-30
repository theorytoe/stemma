# Stemma — Skill suite: changes to prior code

**Scope.** Task 7 (`.ctx/delegates/skills.md`) may need to change code authored
by earlier tasks. Every such change is recorded here, as it is made. This is an
edit trail, not a design document: intent lives in `decisions.md`, the work lives
in the delegate. An entry here does not authorise a change on its own.

## What goes here

- Edits to files first authored by Tasks 1–6: `internal/kb`, `internal/cli`,
  `internal/index`, `internal/source`, `internal/extract`, `internal/citestyle`,
  `internal/render`, the `Makefile`, `.gitignore`, and `FORMAT.md`.
- Additions that move a settled shape: a new target on `check`, a new generated
  artifact.

Not recorded here: new files owned only by this task (`skills/`, `internal/skills`)
and tests for this task's own code.

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

### internal/cli — the CLI reference documents family members

- Date — 2026-09-29
- Prior task — 2 (CLI surface)
- What changed — `referenceMarkdown` in `internal/cli/help.go` now emits a
  `####` subsection for each member of a noun family, with the member's usage
  line, its summary, and a table of its flags. The flag table was extracted to
  `flagTable`, which both a leaf command and a family member render through.
  Previously only the member names were listed, and the family's own `--json`
  table stood in for all of them.
- Why — the skill suite consumes this document as its command reference, and the
  skills name `cite add --doi`, `cite export --format`, `export page --depth` and
  the like. A generated reference that names `cite add` but documents none of its
  flags cannot be the single copy the umbrella points at. This is the gap the
  Task 7 sub-agent exercise surfaced: with only file tools, agents could not
  reconstruct the flags and reached for the source instead.
- Interface impact — the markdown reference gains sections; the JSON `help`
  surface is unchanged and still lists member names only. Nothing reads the
  markdown except the skill suite and a person.
- Tests — `TestReferenceMarkdownDocumentsFamilyMembers` in
  `internal/cli/surface_test.go` asserts the member headings and a sample of
  member flags are present, so the reference cannot thin out again.
- Decision ref — none

### Makefile — the skill suite added to check

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — a `skills` target that writes the umbrella's two generated
  references, a `check-skills` target that regenerates them and runs the
  validator, and `check-skills` added to `check`.
- Why — the Makefile's own note says further gates belong in `check` so a local
  run and a CI run cannot drift apart. The skills are part of the shipped
  surface, so their conformance to the open standard is a gate rather than a
  thing someone remembers. The references are generated here rather than
  committed, because nothing generated is committed (`P8`).
- Interface impact — none; additive, and `check` passes. `test` runs the
  validator's own Go tests separately, so `go test ./...` does not depend on the
  generated files existing.
- Tests — `internal/skills` validates the suite; the target's `test -s` checks
  that both references were written.
- Decision ref — none

### .gitignore — generated skill references ignored

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — `skills/stemma/references/` added to `.gitignore`.
- Why — those two files are written by `make skills` from the tool's command
  table and from `FORMAT.md`. Committing them would put a second copy of each in
  the tree, which is exactly what the umbrella's reference pointers exist to
  avoid.
- Interface impact — none; the committed tree ships the six `SKILL.md` files and
  a person runs `make skills` before copying `skills/` into a harness.
- Tests — `git status` after `make skills` is clean, which `make check` relies on
  rather than asserts.
- Decision ref — none

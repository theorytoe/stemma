# Stemma — Typography: changes to prior code

**Scope.** The typography change sets the site's reading face. It is a one-file
change to the stylesheet Task 6 authored, recorded here as it is made.

## What goes here

- Edits to files first authored by an earlier task — here,
  `internal/render/assets/style.css` (Task 6).

Not recorded here: the wiki page and decision register that describe the change,
which are this change's own record.

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

### internal/render/assets/style.css — the site's reading face

- Date — 2026-09-29
- Prior task — 6 (render and export)
- What changed — `body` read in a serif stack and thirteen chrome selectors each
  named the same `ui-sans-serif` stack. Both are gone: one `--font-sans` custom
  property holds `"Work Sans", Roboto, ui-sans-serif, system-ui, sans-serif`, the
  body and every chrome selector use `var(--font-sans)`, and only `pre` keeps its
  monospace stack. No font files were added.
- Why — the author asked for Work Sans as the primary face with Roboto as the
  fallback. Naming the font rather than shipping it keeps the site self-contained
  (`D3`) and costs no bytes.
- Interface impact — the built site's text renders differently for a reader who
  has Work Sans or Roboto installed; nothing else moves. The repeated stacks
  collapse into one property, so a future face is a one-line change.
- Tests — none assert CSS text. The site gate rebuilds the example wiki, and the
  no-JavaScript conformance gate reads the same stylesheet.
- Decision ref — `D73`

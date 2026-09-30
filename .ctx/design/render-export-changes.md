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

### internal/cli — the serve command registered

- Date — 2026-09-29
- Prior task — 2 (CLI surface)
- What changed — `serveCommand` added in `internal/cli/serve.go` and to the
  `commands` table in `internal/cli/cli.go`.
- Why — the renderer needs a local entry point, and the command table is where a
  surface is added: dispatch, help and the generated CLI reference are all
  derived from it.
- Interface impact — the table gains `serve`; nothing else moves. Help and the
  reference pick it up without further change.
- Tests — the `internal/serve` suite exercises the server; the CLI surface tests
  pass unchanged.
- Decision ref — none

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

### internal/cli — the build command registered

- Date — 2026-09-29
- Prior task — 2 (CLI surface)
- What changed — `buildCommand` added in `internal/cli/build.go` and to the
  `commands` table in `internal/cli/cli.go`, next to `serve`.
- Why — the renderer has two entry points (`D40`), and the table is where a
  surface is added: dispatch, help and the generated CLI reference are all read
  from it, so none of them needed a second list. The command itself stays thin,
  resolving the output directory and handing the writing to `internal/build`.
- Interface impact — the table gains `build`; nothing else moves. `internal/build`
  is new and belongs to this delegate.
- Tests — `internal/build` covers the write, the URL decoding, the relative
  addresses, and what a rebuild removes and what it leaves alone;
  `internal/cli/build_test.go` covers the default directory, a relative `--out`,
  and the JSON payload.
- Decision ref — none

### Makefile — the static site added to check

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — a `site` target added, and `check` now depends on it. It runs
  `stemma build --kb wiki` with the binary the same run just built.
- Why — the Makefile already says further gates belong in `check` so that a local
  run and a CI run cannot drift apart. Building the example wiki is the end-to-end
  path — parse, render, write — and the assertions about what the output must
  contain are the conformance test, so the gate is those two halves together.
- Interface impact — none; the target is additive and `check` passes.
- Tests — `internal/build`'s conformance test is the assertion half and runs
  inside `test`.
- Decision ref — none

### internal/cli — the export family registered

- Date — 2026-09-29
- Prior task — 2 (CLI surface)
- What changed — `exportCommand` and its `json` member added in
  `internal/cli/export.go` and to the `commands` table in `internal/cli/cli.go`.
  The rule for where a command writes was extracted to `outputPath`, which
  `build` now reads as well.
- Why — a surface is added in the table: dispatch, help and the generated CLI
  reference all read it. `outputPath` exists because two commands now take a path
  that is relative to the KB root rather than to the working directory, and that
  rule should be stated once rather than twice.
- Interface impact — the table gains `export`, a noun family with one member, so
  `export` alone is a usage error the way `cite` alone is. `internal/export` is
  new and belongs to this delegate.
- Tests — `internal/export` covers the shape against a golden, each link
  resolution, a citation that does not resolve, the empty lists, determinism and
  the body; `internal/cli/export_test.go` covers the default location, a
  relative `--out`, `--out -`, and the JSON payload.
- Decision ref — D70

### FORMAT.md — the JSON dump documented

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — a `## Exports` section added with `### The JSON dump`, and
  `.stemma/export.json` added to the named artifacts under `.stemma/`.
- Why — `D70` settles a shape other systems read, so it belongs where the format
  is written. `.stemma/` is where the file lands, and the new section is the
  place `export page` will be documented too.
- Interface impact — none; the KB format itself does not change. This documents a
  generated artifact rather than the KB root.
- Tests — the dump's golden test is the shape's regression; nothing in the format
  reads this section.
- Decision ref — D70

### internal/kb — a body rewrite, an orphan exemption, and a manifest writer

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — three additions to `internal/kb`. `Page.RewriteBody` replaces
  the inline constructs a body holds, offering each one its own bytes;
  `Graph.exemptFromOrphan` lifts the orphan exemption out of `Orphans` and adds
  the entry document to it; `Manifest.TOML` renders a new manifest as file
  text, and `init`'s private `manifestFor` and `tomlString` now live with it.
- Why — an extract has to rewrite the constructs it cannot keep, and
  `RewriteLinks` only changes the target *inside* a wikilink's brackets, so it
  cannot prune one to plain text. The orphan exemption is `D71`: the page every
  other page is reachable from has no inbound links by construction, and an
  extract's root is that page with a type that is not `index`. A manifest
  renderer exists because an extract is a KB root and needs one; the invariant
  that a manifest which has been *read* is never re-emitted is untouched, and
  `init` now writes its manifest through the same code, which adds
  `default_type` to a new KB's file.
- Interface impact — additive: one new `Page` method, one new `Manifest` method,
  and a behaviour change in `Orphans` that only removes a finding. No signature
  changed. `init`'s manifest text gains a `default_type` line.
- Tests — `internal/export`'s scoped tests cover the rewrite on every case an
  extract decides on, and assert the source KB comes back byte-identical. The
  exemption is covered by hand on a KB whose entry document is not an index.
- Decision ref — D71

### internal/cli — the export page command registered

- Date — 2026-09-29
- Prior task — 2 (CLI surface)
- What changed — `exportPageCommand` added to the `export` family in
  `internal/cli/export.go`.
- Why — the family was registered for `export json` in the same task that added
  this member, so what changed for the earlier tasks is only that the table's
  `export` entry now has two members rather than one.
- Interface impact — `export page` joins `export json`; nothing else moves.
- Tests — `internal/cli/export_test.go` covers the default destination, a
  relative `--out`, `--depth` in all three spellings, and the JSON payload.
- Decision ref — D71

### FORMAT.md — the scoped extract documented

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — `### The scoped extract` added under `## Exports`; the orphan
  exemption in "Leniency and preservation" now names the entry document; and
  `.stemma/extract.json` joined the named artifacts under `.stemma/`.
- Why — `D71` settles what an extract is, and the format is where that is
  written. The leniency sentence changed because the rule it stated changed.
- Interface impact — none to the KB format; the exemption change is a lint rule,
  and it removes findings rather than adding them.
- Tests — `internal/export`'s scoped tests write an extract and lint it under
  `kb.Strict`, which is the sentence being true.
- Decision ref — D71

### Makefile — the extract added to check

- Date — 2026-09-29
- Prior task — 1 (foundation)
- What changed — an `extract` target added, and `check` now depends on it. It
  extracts the example wiki's entry document at full depth with the binary the
  same run just built, lints the extract under `--strict`, and builds it as a
  site.
- Why — the same reason `site` is there: the Makefile's own note says further
  gates belong in `check` so that a local run and a CI run cannot drift apart,
  and the definition of done names the extract as its last step. The exhaustive
  assertions are `internal/export`'s, which `test` runs; this target is what
  proves the shipped binary produces an extract that holds up.
- Interface impact — none; additive, and `check` passes.
- Tests — `internal/export`'s gate test is the exhaustive half.
- Decision ref — D30, D71

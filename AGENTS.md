# AGENTS.md

## Project Overview

Stemma is a harness-agnostic tool for authoring and maintaining a
citation-bearing knowledge base (KB) on disk. A KB is a directory of markdown
pages linked with `[[wikilinks]]` that cite sources as `[@key]` from a BibTeX
bibliography. The `stemma` CLI is the universal surface; an MCP server is
planned but not built.

- Language: Go 1.27 (`go.mod`). Python is permitted **only** as the document
  text-extraction shim.
- Module path: `github.com/theorytoe/stemma`. The checkout directory may be
  named differently (it is `agentic-wiki` here); always use the module path from
  `go.mod`, not the directory name.
- Canonical format specification: [FORMAT.md](FORMAT.md). `README.md` is the
  short user-facing entry; keep it true, but leave the format rules to
  `FORMAT.md`.
- The `wiki/` directory is the project's own documentation, written in the
  format the tool defines and built by the tool itself. It doubles as a test
  fixture.
- Design intent lives in `.ctx/`; `.ctx/index.md` lists every file and the
  reading order. The register is `.ctx/design/decisions.md` — principles
  `P1..P10` and the decisions. `.ctx/design/qa-session.md` is the source
  interview, `.ctx/design/shim-contract.md` fixes the Go-to-Python interface,
  the initial plan and its delegates are archived under
  `.ctx/archive/plans/initial/`, and `.ctx/plans/mcp/planfile.md` is the MCP
  plan. **Read the relevant decision before changing a settled behaviour.** If a
  task appears to contradict a decision, raise it rather than working around it.

There is no `package.json`, no npm, and no JavaScript build step anywhere. JS
in the rendered site is polish only; every web surface must work with
JavaScript disabled (`P9`).

## Setup Commands

Go 1.27 or newer. PDF/HTML extraction additionally needs a `python3` on `PATH`;
everything else works without it.

```sh
make build          # build bin/stemma (version-stamped from git describe)
make install        # go install to GOBIN (or GOPATH/bin)
make install-skills # copy skills/ into ~/.agents/skills (override SKILLS_DIR)
make man            # write the manual pages to man/ (override MAN)
make install-man    # copy them to share/man/man1 (override MANPREFIX)
make tidy           # go mod tidy
make fmt            # go fmt ./...
make clean          # rm -r bin man
```

`make build` writes `bin/stemma`. The version is injected with
`-X github.com/theorytoe/stemma/internal/version.Version=...`; a working tree
reports its commit.

## Development Workflow

- Build and run the shipped binary against the example wiki:
  ```sh
  make build
  bin/stemma lint --kb wiki --strict   # must print "clean"
  bin/stemma build --kb wiki           # writes wiki/.stemma/site
  bin/stemma serve --kb wiki           # local site (Ctrl-C to stop)
  bin/stemma-mcp --kb wiki             # the MCP surface, over stdio
  ```
- The example wiki sits **below** the repository root, while discovery only
  walks upward. From the repo root always pass `--kb wiki` (or `--kb
  wiki/<subdir>`); do not rely on discovery.
- The binary is the product. When behaviour changes, exercise it through
  `bin/stemma`, not only through Go unit tests.
- `make check` is the definition of done; see below.

### Architecture

The core library owns the format and invariants; every other package is a thin
surface over it (`P3`, `D10`).

| Path                 | Role                                                                                                                            |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/stemma`         | `main` only: `os.Exit(cli.Run(...))`                                                                                            |
| `cmd/stemma-mcp`     | `main` only: the MCP server's process shell over the curated surface in `internal/mcp`                                          |
| `internal/kb`        | Core: manifest, page model, frontmatter, link and citation resolution, lint invariants, preservation                            |
| `internal/cli`       | Command surface. Dispatch, help, and the generated CLI reference all derive from **one** command table in `internal/cli/cli.go` |
| `internal/source`    | A DOI, arXiv ID, ISBN, URL, or local file path to a bibliography entry; never invents prose or metadata                         |
| `internal/citestyle` | Two named citation styles and nothing else (deliberately not CSL)                                                               |
| `internal/extract`   | Go side of the Python text-extraction shim; `extract.py` is embedded and written into each KB                                   |
| `internal/mcp`       | The curated MCP surface: the tool registry, the payload shapes, and the stateless stdio server over protocol 2026-07-28         |
| `internal/index`     | Tier-1 SQLite FTS5 cache, plus the Tier-0/Tier-1 `Source` interface used by search and graph                                    |
| `internal/render`    | markdown to HTML, the site's templates and assets, resolving wikilinks and citations through the same scan `lint` reads         |
| `internal/serve`     | Local HTTP server over the rendered KB, with server-side search and live reload                                                 |
| `internal/build`     | Static-site writer; the offline entry point of the renderer                                                                     |
| `internal/export`    | Machine-readable JSON dump and scoped KB extract                                                                                |
| `internal/skills`    | Validates `SKILL.md` files against the open Agent Skills standard                                                               |
| `internal/version`   | The version string, stamped at build time                                                                                       |

Retrieval runs in two tiers (`D23`). Tier 0 reads the KB directly and needs no
setup; Tier 1 is the `.stemma/index.sqlite` cache built by `stemma index`. No
command may require the index; a KB with no `.stemma/` is fully usable.

**Adding a CLI verb means editing the command table** in `internal/cli/cli.go`.
A verb that is not in the table does not exist. Help text, the generated
reference, and the manual pages all follow automatically; do not hand-write any
of them.

### Hard constraints

- **Preserve what you do not understand** (`P4`). Writing a page must splice the
  original frontmatter and body, not re-encode them. A parse-then-write round
  trip is byte-identical; a field edit moves only that field (see
  `internal/kb/page_test.go`, `gnarly`).
- **Four direct dependencies** are pinned in `go.mod` for offline builds:
  `yaml.v3`, `BurntSushi/toml`, `blackfriday/v2`, `modernc.org/sqlite`. Prefer
  the standard library; do not add a dependency without a decision.
- **Blackfriday is deliberate** (custom renderer for wikilinks/citations). Do
  not migrate to goldmark.
- **Git is optional** (`P5`). Every command must work in a plain directory.
- **A source is pointed at by an address or a path** (`D74`). `url` is an
  address; a local file is `path`, kept as written and read relative to the
  working directory. A capture vendored under `sources/` is a local source's
  provenance and stands in for a fetched hash in the provenance check (`D79`).
- Generated artifacts are never committed (`P8`); a KB's own live under
  `.stemma/`.
- Exit codes are fixed: `0` clean, `1` validation findings, `2` operational
  error. Every command supports `--json` with one envelope (`command`, `ok`,
  `data`, and `findings` or `error`). Tools are lenient unless `--strict`.

## Testing Instructions

`make check` is exactly what CI runs (`.github/workflows/ci.yml`) and must pass
before you are done. **Add new gates to the `check` target, not to the
workflow**, so local and CI runs cannot drift.

```sh
make test        # go test ./...
make test-shim   # internal/extract/extract_test.py (skipped when no python3)
make vet         # go vet ./...
make lint-wiki   # bin/stemma lint --kb wiki --strict
make site        # bin/stemma build --kb wiki
make extract     # scoped extract, then lint and build it as its own KB
make skills      # regenerate skills/stemma/references/{cli.md,format.md}
make check-skills
make mcp-docs    # regenerate docs/mcp-tools.{json,md}, the MCP tool reference
make man         # regenerate the manual pages
make check-man
make check       # all of the above
make bench       # Tier-0 vs Tier-1 timing; measures, never passes/fails
```

- Tests use the standard `testing` package only; no third-party test framework.
- Golden files live in `internal/**/testdata/` and are compared byte-for-byte.
  If a change legitimately alters output, regenerate the golden and inspect the
  diff rather than loosening the assertion.
- The exhaustive extract/round-trip gate is `internal/export/gate_test.go`; the
  `make extract` target exercises the same path through the shipped binary.
- `make test` and `make check` unset `STEMMA_KB` themselves, so an existing KB
  in the environment cannot reach into a run; a bare `go test ./...` still
  needs `env -u STEMMA_KB`.
- `make test-shim` skips (not fails) without `python3`. The Go suite must pass on
  a machine with no interpreter at all. Do not make Python a build prerequisite.
- `make lint-wiki` failing means the project's own documentation no longer
  satisfies its own checks; fix the wiki or the rule, never lower the gate.

## Code Style

- **gofmt is mandatory.** `gofmt -l .` must be empty; run `make fmt`.
- `go vet ./...` must be clean.
- **Comments are prose written for a reader, and explain *why*, not *what*.**
  Full sentences, wrapped near 80 columns, package-level doc comments on every
  package. This is the strongest convention in the repository — match the
  surrounding density and voice; do not add noise or restate the code.
- Errors: return them with context (`fmt.Errorf("...: %w", err)`). A command
  decides an exit code through the `output` helper; do not call `os.Exit` from
  library code.
- Keep surfaces thin: format knowledge and invariants belong in `internal/kb`,
  not in `internal/cli` or a renderer.
- Name things after the domain (KB, page, source, entry, extract), not after
  mechanisms.

## Documentation and the Example Wiki

- `FORMAT.md` is the **canonical, self-contained** format spec. Change it when
  the format changes; do not fork its rules into prose elsewhere.
- `README.md` is the short user-facing entry: install, a first KB, and the build
  targets. Keep it current when a command or setup step changes.
- `wiki/pages/*.md` are real KB pages: they need `title` and `type` frontmatter,
  use `[[wikilinks]]` and `[@key]` citations, and must pass `lint --strict`.
  Cite an entry from `wiki/bibliography.bib`; running `make check` catches a
  dangling link or key.
- The umbrella skill's references — `skills/stemma/references/cli.md` and
  `format.md` — are **generated** by `make skills` (from the command table and
  from `FORMAT.md`) and are gitignored. Never edit or commit them. Edit the
  source (the CLI table, or `FORMAT.md`) and regenerate.
- The manual pages in `man/` are **generated** by `make man` from the same
  command table and are gitignored. Never write or commit one; add the verb to
  the table and regenerate. The MCP server's page is generated the same way
  from its own registry, by the same target. `make install-man` copies them into
  `$(MANPREFIX)/share/man/man1`.
- Skills follow the open Agent Skills standard; `make check-skills` validates
  name, description, and size. The suite is one umbrella (`skills/stemma`) plus
  five workflow skills (`stemma-research`, `stemma-author`, `stemma-maintain`,
  `stemma-query`, `stemma-publish`).

## Generated Files — Do Not Edit or Commit

`.gitignore` covers all of these; if you see them change in `git status`, stop:

- `.stemma/` at any depth (index, shim, fetch scratch, extract, rendered site).
- `bin/`.
- `__pycache__/` (from running the shim's tests).
- `skills/stemma/references/`.
- `man/` (the manual pages, written by `make man`).
- `docs/` (the MCP schema set and tool reference, written by `make mcp-docs`).

`internal/extract/extract.py` is the source of truth for the shim: the binary
embeds it and writes it to `<kb>/.stemma/shim/extract.py`, rewriting it whenever
it differs. An edit in a KB is therefore lost; change the repository copy.

One directory in a KB is the opposite. `sources/` holds vendored captures; it is
committed, not generated (`D76`).

## Commit and Pull Request Guidelines

Commit subjects are lowercase, imperative, type-prefixed, and **no colon** —
e.g. `add scoped extract`, `fix serve on escaped page names`,
`tweak site typeface to work sans`. Allowed types: `add`, `rm`, `fix`, `update`,
`tweak`, `bump`, `doc`, `fmt`, `release`, `revert`. Keep the subject at 55
characters or fewer (favour 40) and one concern per commit; add a body only to
justify a change that would otherwise look wrong or read as a regression. See
the `commit-style` skill for the full rule set.

Before submitting or asking for review:

1. `make check` passes.
2. `gofmt -l .` prints nothing.
3. New behavior has a test; a settled behaviour change is recorded in
   `.ctx/design/decisions.md`; `FORMAT.md` is updated when the format changes.
4. No generated file is staged.

## Troubleshooting

- **`stemma` not found:** run `make build` and use `bin/stemma`, or `make
  install` to put it on `PATH`.
- **KB not discovered:** pass `--kb PATH`. The example wiki is below the repo
  root, and discovery only walks upward.
- **`lint` exits 1:** findings, not a crash; add `--json` for machine-readable
  detail. Exit 2 is an operational error.
- **`make check` fails in `skills/`:** the generated references are missing.
  Run `make skills`.
- **Index seems stale:** it is stamped by content hash, not mtime; delete
  `.stemma/` and rebuild. The index is a cache and never a source of truth.
- **`cite vendor` refuses a URL:** the reader reduces HTML and reads PDFs, so a
  raw text response is a reported failure. Download it and record it with
  `--path` instead.
- **A round-trip or golden test fails after a writer change:** the preservation
  invariant was broken. Fix the writer; do not update the golden to accept it.

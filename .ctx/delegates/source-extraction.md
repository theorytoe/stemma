---
title: Stemma — Source extraction
date: 2026-09-26
---

This is a delegate plan. It covers Task 4 of the top-level planfile
(`.ctx/planfile.md`). All decisions referenced as `D<n>` or `P<n>` are recorded in
`.ctx/design/decisions.md`.

# Plan details

## Purpose

Turn a document into text an agent can read. This is the one place where Python is
permitted, and the entire delegate exists to keep that permission narrow.

## Why Python, and how narrow

Go is the project language (`D6`). Document extraction is the single exception,
because the PDF and HTML ecosystem is Python's and rebuilding it in Go would be a
project of its own. The rule that keeps this contained: **Python is a leaf, never a
dependency of the core.** The core library must build, test, and run with no
Python present; only this delegate's commands degrade when it is missing.

Verified on the development machine: Python 3.14.7 is present with `pymupdf`,
`pypdf`, `httpx`, `requests`, and `bs4` already importable system-wide. Note that
system pip is `EXTERNALLY-MANAGED`, so the project must never attempt to install
into it. There is no `uv` and no `pipx`; if a managed environment is ever needed,
it is the project's responsibility to create one, and `env` must report it.

## The shim contract

Go invokes a small script bundled with the project, passing a file path or URL,
and receives structured JSON on standard output. Design the contract before
writing extractors:

- One script per input family, or one script with a subcommand; keep it small.
- Input and output are explicit; nothing is configured through the environment.
- Success, "unsupported input", and "extractor missing" are distinguishable, so
  the Go side can degrade rather than guess.
- Extraction output is text plus minimal metadata, never a reformatted document.
- A missing interpreter or a missing library is reported, never worked around.

Bundling the script inside the Go binary is not required, but wherever it lands it
must be findable without a checkout-specific absolute path.

## Scratch area and vendoring

Extracted text goes to a scratch area inside the generated-artifact directory,
which is gitignored and therefore never committed (`D23`, `P8`). Vendor a source's
full text into the KB only when explicitly asked, and verify the recorded content
hash when doing so (`D38`). Vendoring is opt-in because a KB that carries PDFs
becomes unshippable.

## Dependencies

Task 1, and the key and provenance conventions from
`delegates/sources-bibliography.md` — the retrieval date and content hash has to
mean the same thing on both sides.

# Tasks

## Task 1:

Status: Done

Define the shim contract, in writing, before implementing any extractor. Cover the
invocation, the JSON schema for success and for each failure class, the streaming
or buffering behaviour for large documents, and the size limits. This document is
what lets the extractors be replaced without touching Go.

## Task 2:

Status: Done

Prove the contract end to end with one trivial extractor, for plain text or
markdown. The point is to validate the Go-to-Python boundary, the error paths, and
the absence-of-Python path before real extraction complexity is added.

## Task 3:

Status: Done

PDF extraction. Read with `pymupdf` and report it. Handle the cases that matter in
practice: no extractable text layer, multi-column layouts, and very large files.
Refuse to guess rather than emitting garbage. The `pypdf` fallback this originally
asked for was dropped during implementation, for the reason recorded against `D66`
in the decision register.

## Task 4:

Status: Done

URL fetching and main-content extraction. Fetch politely with a timeout and a
user agent, then reduce the page to its main content rather than its navigation.
Report the failure classes separately: network failure, non-HTML content type, and
JS-only pages with no server-rendered text — the last of which is a case the tool
should acknowledge rather than silently return empty.

## Task 5:

Status: Done

The `fetch` command and the scratch area. Deterministic output location, a
predictable filename derived from the source, and a documented lifecycle so an
agent can rely on where extracted text will be. Include a way to clear scratch
without touching anything else.

## Task 6:

Status: Done

Opt-in vendoring. Copy a source's full text into the KB on request, record the
hash, and verify it on subsequent reads. Define what happens when a vendored copy
and its recorded hash disagree — report it, do not silently trust either side.

## Task 7:

Status: Done

Graceful degradation and `env` integration. Every command in this delegate must
fail with exit code `2` and an actionable message when Python or a required library
is absent, and `env` must be able to explain the situation without running an
extraction. The core library must be unaffected: no import, no build tag, no
startup cost.

## Task 8:

Status: Not started

Deferred inputs. Record EPUB, OCR for scanned PDFs, and any other format as
explicitly out of scope. `D43` settles DOI, arXiv, URL, local PDF, and ISBN; the
shim contract exists so these can be added later without touching the Go side.

---
title: Stemma — The extraction shim contract
date: 2026-09-28
---

This document is the interface between the Go core and the Python that reads
documents. It is a deliverable in its own right (`delegates/source-extraction.md`,
Task 1): the extractors are expected to be replaced, and this is the part that must
not move when they are. Nothing in it depends on which library does the reading.

# Why there is a contract at all

Go is the project language (`D6`) and Python is the single exception, admitted
because the PDF and HTML ecosystem is Python's. The permission stays narrow by
being spent in exactly one place with one shape: a script that takes an explicit
input, prints one JSON object, and exits. Everything else — which file gets read,
what the tool says when it cannot read it, where the text is written, what an exit
code means — belongs to Go.

The rule that follows from it: **Python is a leaf, never a dependency of the
core.** The core library must build, test, and run with no Python present. Only
the commands in this delegate degrade, and they degrade by reporting, never by
working around (`P1`).

# Invocation

```
extract.py <subcommand> <input> [<user-agent>]
```

One script, one subcommand per input family. The script is never sourced, never
imported, and never told anything through the environment.

| subcommand | input           | user agent | reads                                  |
| ---------- | --------------- | ---------- | -------------------------------------- |
| `probe`    | —               | —          | nothing; reports what is importable     |
| `text`     | a file path     | —          | the file as UTF-8 text                  |
| `pdf`      | a file path     | —          | the file as a PDF                       |
| `url`      | an absolute URL | required   | the URL, then the main content of the page |

`probe` exists because `env` has to explain a degraded machine without running an
extraction (`Task 7`). It reports the interpreter and, for every library this
contract names, whether it imports and which version it is. It is the only
subcommand that must work on a machine where nothing else does, and it must not
fail: a missing library is a fact it reports, not an error it raises.

`url` takes the user agent as an argument rather than deciding it, because the Go
side already has one (`stemma/<version>`) and a second user agent invented here
would be a second thing to keep true.

**No flags.** The size limit below is a constant of the contract, not an option,
so the two sides cannot disagree about it. There is no verbose mode, no output
file, and no configuration argument: an extractor that needs a setting to be
useful is an extractor whose behaviour the Go side cannot predict.

**The shim defines no environment variable and branches on none.** Letting a
caller change behaviour through the environment is how an interface acquires a
second, invisible input. The one thing this does not cover is what a library does
internally, such as honouring a proxy or a certificate store; that is the
library's business, not configuration the shim accepts.

# The one object

stdout carries exactly one JSON object and nothing else. Both success and failure
use it, so the Go side parses one shape and branches on one field.

`contract` is present in every object. It is the integer version of this
document. Go refuses to use output whose `contract` it does not know, which is
what makes an old copy of the script a diagnosable error rather than a strange
one.

Success:

```json
{
  "contract": 1,
  "ok": true,
  "kind": "pdf",
  "text": "…",
  "extractor": "pymupdf 1.26.4",
  "pages": 12,
  "truncated": false,
  "notes": ["page 3 has no text layer"]
}
```

| field       | required | meaning                                                              |
| ----------- | -------- | -------------------------------------------------------------------- |
| `contract`  | always   | the contract version                                                 |
| `ok`        | always   | `true`                                                               |
| `kind`      | always   | the subcommand that ran                                              |
| `text`      | always   | the extraction, as UTF-8. Never empty when `ok` is `true`            |
| `extractor` | always   | what actually read it, as `<name> <version>` or a chain like `httpx+bs4` |
| `pages`     | PDF only | how many pages the document has                                      |
| `truncated` | always   | whether `text` was cut at the size limit                             |
| `notes`     | optional | short statements about the input that did not stop extraction        |

`notes` is where a fact about the input goes when the text is still usable: a page
with no text layer in an otherwise readable PDF, a table that could not be
linearised. It is not a place for warnings about the environment.

Failure:

```json
{
  "contract": 1,
  "ok": false,
  "error": { "class": "unsupported", "message": "epub is not read by this shim" }
}
```

`class` is the failure taxonomy and is the only field Go branches on. `message` is
for a person and may change freely; Go never parses it.

| class               | means                                                              | a fact about |
| ------------------- | ------------------------------------------------------------------ | ------------ |
| `unsupported`       | this shim does not read this kind of input at all                   | the input    |
| `missing_extractor` | the kind is supported, but no library for it is importable          | the machine  |
| `unreadable`        | the path is absent, unreadable, or is not what it claims to be      | the input    |
| `empty`             | extraction ran and found no text: no text layer, or a JS-only page  | the input    |
| `network`           | the fetch failed: DNS, TLS, a refused connection, or a non-2xx      | the machine  |
| `too_large`         | the input or the download exceeded the size limit                   | the input    |
| `internal`          | the shim was called wrongly, or hit a bug                           | the shim     |

Two of these carry the delegate's weight. `missing_extractor` is what lets `env`
say *why* nothing works instead of "failed". `empty` is the case that must not
become a silent success: a scanned PDF and a JavaScript-only page both extract to
nothing, and the tool has to say which happened rather than hand back an empty
string (`Task 3`, `Task 4`).

**`missing_python` is not in this table, because the shim cannot report it.** If
the interpreter is absent, nothing runs. Go checks for the interpreter before it
spawns anything and produces that outcome itself. A class the shim could never
emit would be a lie in the contract.

# Exit status, stderr, and the failures Go adds

The rule is short: **exit 0 whenever a JSON object was printed, non-zero only when
the shim could not print one.** Success and every typed failure above are exit 0,
because the object already says what happened and putting the same information in
two places would invite them to disagree.

A non-zero exit therefore means the shim died: a traceback, a syntax error, a
missing interpreter, a killed process. There is no object to parse, so Go adds the
failure itself. These are Go's classes, not the shim's:

| class            | means                                                        |
| ---------------- | ------------------------------------------------------------ |
| `missing_python` | no interpreter was found on `PATH`                            |
| `timeout`        | the shim was killed after the wall-clock limit                |
| `crashed`        | it exited non-zero, or printed something that is not one object |

stderr is for diagnostics. Go captures a bounded amount of it and quotes it only
when the shim crashed, on the reasoning that a traceback is the most useful thing
to show a person and an unused diagnostic stream is noise. Nothing ever parses
stderr.

# Size, buffering, and time

**The text is capped at 8 MiB of UTF-8.** The shim truncates at that point, on a
line boundary where one is available, and sets `truncated: true`. Go reads at most
the cap plus 64 KiB of slack and treats anything longer as `internal`, so a shim
that ignores its own limit cannot exhaust the process that called it.

The shim buffers rather than streams. One JSON object is one write, and a JSON
string cannot be escaped correctly across chunk boundaries without building a
streaming encoder on both sides — machinery that would exist only to let a
document be half-read. The cap is what makes buffering safe: memory is bounded by
the limit, not by the document. For a large PDF this costs nothing either way,
because the reading library takes a path and does not need the file in memory.

The URL family caps the downloaded body at the same 8 MiB, before parsing, and
reports `too_large` if a page exceeds it. A page that big is not a document.

**Go enforces the timeout, not the shim.** A process cannot be trusted to kill
itself on schedule, and a shim that blocked inside a C library would not get the
chance. Go runs the shim under a context with a wall-clock limit — 60 seconds for
`url`, 300 for `pdf`, since linearising a book is legitimately slow — and maps a
kill to `timeout`.

# What the text is

`text` is the document's text as UTF-8, and it is not a document. The shim does
not emit Markdown, does not reflow headings, does not insert page markers, and
does not add a title. Anything that is *about* the extraction rather than *in* the
document goes in `extractor`, `pages`, or `notes`.

The property this buys is determinism: the same input read with the same libraries
produces byte-identical text. No timestamps, no source path, no ordering that
depends on when it ran. Text that changes between two runs is text that cannot be
diffed, hashed, or vendored (`Task 6`).

The shim writes nothing but stdout and stderr. It creates no temporary file,
leaves no cache, and touches the network only for `url`. This is the reason
`pdftotext` was not adopted as a fallback despite being installed on the
development machine: it writes to a path, which would put file lifecycle into the
contract for a fallback of a fallback.

# Where the script lives

The script is embedded in the Go binary with `go:embed` and materialised to
`<kb>/.stemma/shim/extract.py` before it runs.

Embedding is what keeps the binary and the contract from drifting: the bytes that
print the JSON are the bytes compiled into the version that parses it, and a
`stemma` copied to another machine still carries them. Materialising to a path
rather than piping the script to the interpreter keeps it a real file — something
an author can open when the extraction is wrong, and something `env` can name.
`.stemma/` is the generated-artifact directory (`D23`), so it is gitignored,
outside the preservation guarantee, and rewritten whenever the embedded bytes
differ. Hand-edits there are lost by design; the place to change the shim is the
repository.

If the KB directory cannot be written, the shim is materialised in a temporary
directory instead. `env` reports the path it used and whether it is current, so
"which copy ran" is answerable without guessing.

# How this is tested without Python

The Go side never invokes `python3` directly; it invokes a runner that can be
pointed elsewhere. Every test of the Go half — parsing each class, a truncated
result, an oversized result, a crash with no object, a timeout, a missing
interpreter — runs against a fake script that prints canned JSON. Those tests must
pass on a machine with no Python at all, because that is the machine the core
promises to work on.

Tests that exercise real extraction call `probe` first and skip unless the library
is reported. They are evidence about the extractor, never a precondition for
building or testing Stemma.

Nothing in `internal/kb` imports the extraction package. The core's no-Python
property is a dependency direction, not a convention.

# Deliberately out of scope

OCR for scanned PDFs, EPUB, and rendering a JavaScript-only page are not inputs
this shim accepts (`Task 8`). They are `unsupported` and `empty` respectively,
which is the point of having those classes: a refusal that names itself is
degradation, and a blank string is not.

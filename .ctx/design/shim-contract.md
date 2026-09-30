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
what the tool says when it cannot read it, where the text goes, what an exit code
means — belongs to Go.

The rule that follows from it: **Python is a leaf, never a dependency of the
core.** The core library must build, test, and run with no Python present. Only
the commands in this delegate degrade, and they degrade by reporting, never by
working around (`P1`).

Writing the script is not one of the places Python is needed. The script is bytes
in the binary; installing it is a file write.

# Invocation

```
extract.py <subcommand> <input> <limit> [<user-agent>]
```

One script, one subcommand per input family. The script is never sourced, never
imported, and never told anything through the environment.

| subcommand | input           | limit   | user agent | reads                              |
| ---------- | --------------- | ------- | ---------- | ---------------------------------- |
| `probe`    | —               | —       | —          | nothing; reports what is importable |
| `text`     | a file path     | bytes   | —          | the file as UTF-8 text              |
| `pdf`      | a file path     | bytes   | —          | the file as a PDF                   |
| `url`      | an absolute URL | bytes   | required   | the URL, then the page's main text  |

`probe` exists because `env` has to explain a degraded machine without running an
extraction (Task 7). It reports the interpreter and, for every library this
contract names, whether it imports and which version it is. It is the only
subcommand that must work on a machine where nothing else does, and it must not
fail: a missing library is a fact it reports, not an error it raises.

`limit` is a required argument rather than a constant of this document. It is
data Go must supply — how much text it is willing to receive — and the two sides
cannot disagree about a number that only one of them chooses. It also means the
default can be tuned in Go, and a `--max-bytes` flag can appear later, without
editing this file.

`limit` bounds the text, and not the object that carries it. JSON escaping makes
the object larger than the text inside it — a quote, a backslash or a newline
becomes two bytes, and any other control character becomes six — so Go allows for
that rather than refusing an answer that is correct. A shim that prints past even
that allowance is the case the ceiling exists for.

`url` takes the user agent as an argument for the same reason: the Go side already
has one (`stemma/<version>`), and a second user agent invented here would be a
second thing to keep true.

There are no other arguments. No verbose mode, no output file, no configuration
option: an extractor that needs a setting to be useful is an extractor whose
behaviour Go cannot predict.

**The shim defines no environment variable and branches on none.** Letting a
caller change behaviour through the environment is how an interface acquires a
second, invisible input. The one thing this does not cover is what a library does
internally, such as honouring a proxy or a certificate store; that is the
library's business, not configuration the shim accepts.

# The one object

stdout carries exactly one JSON object and nothing else. Both success and failure
use it, so the Go side parses one shape and branches on one field.

`contract` is present in every object. It is the integer version of this document.
Go refuses to use output whose `contract` it does not know, which turns a stale
copy or a development override into a diagnosable error rather than a strange one.

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

`probe` is the one subcommand whose success object carries no `text`, because it
is describing the machine rather than a document. It answers with the
interpreter and the libraries instead:

```json
{
  "contract": 1,
  "ok": true,
  "kind": "probe",
  "python": "3.14.7",
  "libs": { "pymupdf": "1.26.4", "httpx": "0.28.1", "bs4": null }
}
```

| field       | present    | meaning                                                              |
| ----------- | ---------- | -------------------------------------------------------------------- |
| `contract`  | always     | the contract version                                                 |
| `ok`        | always     | `true`                                                               |
| `kind`      | always     | the subcommand that ran                                              |
| `text`      | except `probe` | the extraction, as UTF-8. Never empty when `ok` is `true`         |
| `extractor` | except `probe` | what read it, as `<name> <version>` or a chain like `httpx+bs4`   |
| `pages`     | PDF only   | how many pages the document has                                      |
| `truncated` | except `probe` | whether `text` was cut at the limit                               |
| `notes`     | optional   | short statements about the input that did not stop extraction        |
| `python`    | `probe`    | the interpreter's version                                            |
| `libs`      | `probe`    | module to version; a module that is not importable has none           |

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
| `too_large`         | the input or the download exceeded the limit                        | the input    |
| `internal`          | the shim was called wrongly, or hit a bug                           | the shim     |

Two of these carry the delegate's weight. `missing_extractor` is what lets `env`
say *why* nothing works instead of "failed". `empty` is the case that must not
become a silent success: a scanned PDF and a JavaScript-only page both extract to
nothing, and the tool has to say which happened rather than hand back an empty
string (Tasks 3 and 4).

**`missing_python` is not in this table, because the shim cannot report it.** If
the interpreter is absent, nothing runs. Go checks for the interpreter before it
spawns anything and produces that outcome itself. A class the shim could never
emit would be a lie in the contract.

# Exit status, stderr, and the failures Go adds

The rule is short: **exit 0 whenever a JSON object was printed, non-zero only when
the shim could not print one.** Success and every typed failure above are exit 0,
because the object already says what happened, and putting the same information in
two places invites the two to disagree.

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

The text is capped at the `limit` argument. The shim stops reading once it has
that much, cuts on a line boundary where one is available, and sets
`truncated: true`. Stopping early is the point: a nine-hundred-page PDF costs
roughly what its first pages cost, rather than being linearised in full before Go
can decide it does not want it.

The shim buffers rather than streams. One JSON object is one write, and a JSON
string cannot be escaped correctly across chunk boundaries without building a
streaming encoder on both sides — machinery that would exist only to let a
document be half-read. The limit is what makes buffering safe: memory is bounded
by the caller's number, not by the document. For a large PDF this costs nothing
either way, because the reading library takes a path and does not need the file in
memory.

Go reads at most `limit` plus 64 KiB of slack and treats anything longer as
`internal`, so a shim that ignores its own limit cannot exhaust the process that
called it.

The URL family bounds the downloaded body by the same limit, before parsing, and
reports `too_large` when a page exceeds it. A page larger than the text anyone
wants is not a document worth fetching.

**Go enforces the timeout, not the shim.** A process cannot be trusted to kill
itself on schedule, and a shim blocked inside a C library would not get the
chance. Go runs the shim under a context with a wall-clock limit — 60 seconds for
`url`, 300 for `pdf`, since linearising a book is legitimately slow — and maps a
kill to `timeout`.

# The PDF family

`pdf` reads with **PyMuPDF, and PyMuPDF alone**. `extractor` names it with its
version, so what read a document is never a guess. A machine without it is
`missing_extractor`, which is a fact about the machine rather than about the file.

One library and not a list is a deliberate narrowing. The delegate asked for
pymupdf with a pypdf fallback, and implementing it showed what the fallback
actually costs: the two libraries do not report the same things — the two-column
note below needs block geometry that pypdf does not offer — so the tool's answers
would have depended on which one happened to be installed. Requiring pymupdf makes
the machine stop being a variable.

**The file is judged by its own bytes, not by what the library will accept.** This
is the second, independent reason, and it is not about installations at all.
PyMuPDF opens EPUB, XPS, CBZ, MOBI and plain text files besides PDFs, and shown a
.txt it will happily return a page of text. Handing it a path unchecked would
therefore mean a document being read by a pipeline the contract does not describe
and reported as `kind: "pdf"` with a PDF library named as the extractor. Every
field of that answer would be wrong about what had happened, and EPUB would be
supported by accident rather than by design: no chapter structure, no metadata,
and nothing for the vendoring and hashing in Task 6 to hold on to.

So a document is accepted here as a PDF or not at all: a `%PDF-` header in the
first 1024 bytes, or a refusal. A zip container — which is what an EPUB is — is
`unsupported` by name, and anything else is `unreadable`. This is what keeps the
EPUB exclusion in the last section true rather than aspirational, and it turns
"that is not a PDF" into a sentence that says so instead of a library's format
error.

Three outcomes are worth telling apart, and the classes exist to keep them apart:

- **An encrypted PDF** is `unreadable`. It is a real document that cannot be read,
  which is a different thing from one with nothing in it.
- **A page with no text layer** is a `note` when other pages have text, and
  `empty` when none of them do. That is the case a scan produces, and saying so is
  the point: `empty` names OCR as the reason, where an empty string would look like
  a document that happened to say nothing.
- **A page that looks like two columns of body text** is a `note`. It is never
  reordered. An extractor's own block order is usually right, because a PDF stores
  text in the order it was drawn, and a heuristic that reordered a page wrongly
  would be worse than one that says the order may be the library's.

Reading stops as soon as the limit is passed, so a nine-hundred-page PDF costs
roughly what its first pages cost, and `pages` still reports the document's true
length rather than the number of pages read.

# The URL family

`url` fetches a page with **httpx** and reduces it with **bs4**, and `extractor`
reports `httpx+bs4`. Both are required and neither has a fallback: the same
reasoning as the PDF family applies, that a second path is a second behaviour, and
the standard library is not a quieter substitute for either.

**Politeness is in the request, not in a delay.** The request carries the user
agent Go supplied and an `Accept` for HTML, httpx is given a 30-second timeout for
connect and read, and Go enforces its own 60-second wall clock above that. The
library timeout is deliberately the shorter one, so a slow server produces a
sentence about the server rather than a killed process. Redirects are followed —
httpx does not do so unless asked, and a page behind a redirect is not a failure.

**What comes back is checked before it is read.**

- A response that does not declare `text/html` or `application/xhtml+xml` is
  `unsupported`, with the declared type named in the message.
- A status outside 2xx is `network`, with the status named. A 404 is a fact about
the request rather than about the machine, and it is still a failed fetch, so it
  is not given a class of its own.
- A body past `limit` is `too_large`, and it is stopped at the limit rather than
  after it, so a server that would stream a gigabyte is cut off instead of
  buffered.

**Main content means the document, not the page.** Elements that are never the
document — `script`, `style`, `nav`, `header`, `footer`, `aside`, `form` and a few
more — are removed, and then the text is taken from the first of `article`,
`main`, `[role=main]`, `#content`, `.content` that the page has. Falling back to
`body` is the honest last resort, and it is reported as a `note` rather than
silently accepted, because "the body was read" and "the article was read" are
different answers to the question this command exists to ask. Whitespace is
tidied, since line breaks in markup say nothing about the document; nothing is
reordered, nothing is added, and no Markdown or title is produced.

**A page with no server-rendered text is `empty`, and the message says
JavaScript.** This is the case the delegate singled out: a single-page application
serves a shell and fills it in later, so an extractor returns nothing at all.
Returning that nothing as an empty string would look like a document that happened
to say nothing; `empty` says which of the two happened, and that rendering a page
is out of scope.

# What the text is

`text` is the document's text as UTF-8, and it is not a document. The shim does
not emit Markdown, does not reflow headings, does not insert page markers, and
does not add a title. Anything that is *about* the extraction rather than *in* the
document goes in `extractor`, `pages`, or `notes`.

The property this buys is determinism: the same input read with the same libraries
produces byte-identical text. No timestamps, no source path, no ordering that
depends on when it ran. Text that changes between two runs is text that cannot be
diffed, hashed, or vendored (Task 6).

The shim writes nothing but stdout and stderr. It creates no temporary file,
leaves no cache, and touches the network only for `url`. This is the reason
`pdftotext` was not adopted as a fallback despite being installed on the
development machine: it writes to a path, so the tool would acquire a temporary
file's lifecycle in the middle of extracting someone's document. The one file the
tool writes of its own accord is the script itself, once, and that is a different
kind of thing.

# Where the script lives

The script is a tracked file in the repository, `internal/extract/extract.py`. It
is embedded in the Go binary at compile time, so the bytes that print the JSON are
the bytes compiled into the version that parses it.

`init` writes it out eagerly, to `<kb>/.stemma/shim/extract.py`, in the same
breath as the manifest and the entry document. A new knowledge base (KB) has its
whole shape from the moment it exists: nothing appears later the first time
someone runs `fetch`, and the file is there to be read, run by hand, and pointed
at. `kb.Init` does not know this file exists — `extract.Materialise` writes it,
and the init command calls both — so the core keeps no dependency on the
extraction package.

**Every `.stemma/` artifact is per-checkout and re-derivable, and is re-established
whenever it is absent or stale.** So the script is also checked before each
extraction against the embedded bytes: missing means write, identical means
nothing, different means overwrite. A cleared `.stemma/` (a documented operation),
an upgrade, and a KB received by clone all change or remove it, and none of them
should leave extraction broken. The check is a byte comparison against a file of a
few kilobytes, so there is nothing to record alongside it and no digest to keep in
step. The write goes to a temporary file in the same directory and is renamed into
place, so an interrupted write never leaves a half-finished script.

`init` fails if the write fails, like every other file it writes. A KB that
cannot take the script is a KB that could not be created, and the alternative —
reporting success for a half-made KB — would put the difference somewhere a
person would have to go looking for it.

`env` reports where the script is and whether it is `absent`, `current`, or
`stale`, and it writes nothing *into the KB*: a diagnostic that changes the thing
it is describing is answering a different question. It does need a script to ask
the interpreter anything, so with no KB, or one that cannot be written, it uses a
copy made in a temporary directory and says as much, rather than printing a path
that is about to disappear. An extraction still needs a KB it can write to,
because its text goes there — the temporary copy is for the question, not for the
work.

`STEMMA_SHIM` overrides the path, for developing an extractor without rebuilding
the binary each time. `env` reports when it is in force, and `contract` catches the
mismatch if what it points at is not the version this binary expects.

# How this is tested without Python

Because the script is a real file, it has its own tests beside it, runnable as
plain `python3` with no framework. They exercise each subcommand and each failure
class directly, which is not possible when the script exists only inside a binary.
What they cannot test is a library that is not installed: those cases skip.

The Go side never invokes `python3` directly; it invokes a runner that can be
pointed elsewhere. Every test of the Go half — parsing each class, a truncated
result, an oversized result, a crash with no object, a timeout, a missing
interpreter — runs against a fake script that prints canned JSON. Those tests pass
on a machine with no Python at all, because that is the machine the core promises
to work on. Alongside them, a test asserts that what `Materialise` writes is
byte-identical to the embedded script, and that a stale copy is refreshed.

Nothing in `internal/kb` imports the extraction package. The core's no-Python
property is a dependency direction, not a convention.

# Deliberately out of scope

OCR for scanned PDFs, EPUB, and rendering a JavaScript-only page are not inputs
this shim accepts (Task 8). EPUB is refused by the bytes guard above; a scan is
`empty` and a JavaScript-only page is `empty` too. That is the point of having
those classes: a refusal that names itself is degradation, and a blank string is
not.

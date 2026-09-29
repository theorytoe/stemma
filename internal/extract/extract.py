#!/usr/bin/env python3
"""The extraction shim.

One document in, one JSON object out. The contract this implements is written
down in .ctx/design/shim-contract.md; the short of it is that Go owns every
decision — what to read, what it means, what to say when it fails — and this file
owns only the reading.

Nothing here reads the environment, writes a file, or invents an argument. A
failure it understands becomes an object with a class; a failure it does not
becomes a traceback, which is how Go is told that the shim itself broke.
"""

import json
import sys

# The contract version this script implements. Go refuses an object whose
# contract it does not recognise, so a stale copy is a sentence rather than a
# mystery.
CONTRACT = 1

# The libraries this contract names, each with the module names that can satisfy
# it. probe reports on all of them, whether or not a subcommand uses one yet: its
# job is to describe the machine, because that is what `stemma env` has to
# explain. PyMuPDF answered to `fitz` before it answered to `pymupdf`, and a
# machine with either one has a PDF reader.
LIBRARIES = {
    "pymupdf": ("pymupdf", "fitz"),
    "httpx": ("httpx",),
    "bs4": ("bs4",),
}

# The most bytes one UTF-8 character can occupy. Reading this far past the limit
# is what lets a cut land on the last whole character rather than the middle of
# one.
MAX_CHAR = 4


def ok(**fields):
    result = {"contract": CONTRACT, "ok": True}
    result.update(fields)
    return result


def failed(cls, message):
    return {
        "contract": CONTRACT,
        "ok": False,
        "error": {"class": cls, "message": message},
    }


def import_module(name):
    """The module, or None when it cannot be imported.

    Catching everything is the point rather than a shortcut: "not importable" is
    the answer the caller wants, and an import can fail in many ways that are all
    equally uninteresting here.
    """
    try:
        return __import__(name)
    except Exception:
        return None


def import_any(candidates):
    """The first of several module names that imports, or None."""
    for name in candidates:
        module = import_module(name)
        if module is not None:
            return module
    return None


def version_of(module):
    """The version of an imported module, or None when there is no module."""
    if module is None:
        return None
    return str(getattr(module, "__version__", "") or "unknown")


def probe(_argv):
    return ok(
        kind="probe",
        python="%d.%d.%d" % sys.version_info[:3],
        libs={
            name: version_of(import_any(candidates))
            for name, candidates in LIBRARIES.items()
        },
    )


def parse_limit(value):
    try:
        limit = int(value)
    except ValueError:
        return None
    return limit if limit > 0 else None


def clip(raw, limit):
    """Cut raw to at most limit bytes without splitting a character.

    A cut prefers a line boundary, because text that stops mid-sentence is worse
    to read than text that stops a line early. Returns (bytes, was_cut).
    """
    if len(raw) <= limit:
        return raw, False

    end = limit
    while end > 0 and (raw[end] & 0xC0) == 0x80:
        end -= 1

    cut = raw[:end]
    newline = cut.rfind(b"\n")
    if newline >= 0:
        cut = cut[: newline + 1]
    return cut, True


def text(argv):
    if len(argv) != 2:
        return failed(
            "internal", "text takes a path and a limit, got %d arguments" % len(argv)
        )

    path = argv[0]
    limit = parse_limit(argv[1])
    if limit is None:
        return failed("internal", "the limit must be a positive integer, got %r" % argv[1])

    try:
        with open(path, "rb") as handle:
            raw = handle.read(limit + MAX_CHAR)
    except OSError as exc:
        return failed("unreadable", str(exc))

    if not raw:
        return failed("empty", "the file is empty")

    body, cut = clip(raw, limit)
    try:
        body = body.decode("utf-8")
    except UnicodeDecodeError as exc:
        return failed("unreadable", "not UTF-8 text: %s" % exc)

    if not body.strip():
        return failed("empty", "the file holds no text")

    return ok(kind="text", text=body, extractor="stdlib", truncated=cut)


def sniff_pdf(path):
    """Judge the file by its own bytes before any library does.

    PyMuPDF reads EPUB, XPS and a few other container formats besides PDF. That
    capability is deliberately not in the contract, so a document is accepted here
    as a PDF or not at all, rather than according to whichever library happens to
    be installed on the machine. Returns a failure, or None when the file looks
    like a PDF.
    """
    try:
        with open(path, "rb") as handle:
            head = handle.read(1024)
    except OSError as exc:
        return failed("unreadable", str(exc))

    if b"%PDF-" in head:
        return None
    if head[:4] == b"PK\x03\x04":
        return failed(
            "unsupported",
            "this is a zip container, most likely an EPUB, and EPUB is out of scope",
        )
    return failed("unreadable", "no %PDF- header in the first 1024 bytes, so this is not a PDF")


# How many pages are examined for the multi-column signature. It is a property of
# a document in practice, so sampling bounds what a heuristic costs.
COLUMN_SAMPLE = 5

# A block has to be at least this long to count as body text rather than a caption
# or a label.
COLUMN_MIN_CHARS = 200


def looks_two_column(page):
    """Whether this page looks like two columns of body text side by side.

    This is used to warn, never to reorder. An extractor's own block order is
    usually right for a two-column paper, because text is stored in the order it
    was drawn, and a heuristic that reordered a page wrongly would be worse than
    one that says the order may be the library's.
    """
    try:
        blocks = [
            block
            for block in page.get_text("blocks")
            if len(block[4].strip()) >= COLUMN_MIN_CHARS
        ]
    except Exception:
        return False

    for left in blocks:
        for right in blocks:
            if left[2] <= right[0]:  # entirely to the left of the other
                overlap = min(left[3], right[3]) - max(left[1], right[1])
                shorter = min(left[3] - left[1], right[3] - right[1])
                if shorter > 0 and overlap > shorter / 2:
                    return True
    return False


class Reading:
    """Pages accumulated until the caller has been given more than it asked for.

    The character count is the stopping test and the byte count is the cut. Every
    character is at least one byte, so passing the limit in characters means the
    limit has certainly been passed in bytes, and the exact boundary is then
    decided once, at the end, rather than measured on every page.
    """

    def __init__(self, limit):
        self.limit = limit
        self.chunks = []
        self.chars = 0
        self.blank = 0
        self.broken = 0
        self.columns = 0

    def add(self, text, two_column=False):
        if not text.strip():
            self.blank += 1
            return
        self.chunks.append(text)
        self.chars += len(text)
        if two_column:
            self.columns += 1

    def broken_page(self):
        self.broken += 1

    def full(self):
        return self.chars > self.limit

    def answer(self, extractor, pages):
        if not self.chunks:
            if self.broken and not self.blank:
                return failed("unreadable", "the PDF opened but no page could be read")
            return failed(
                "empty",
                "the PDF has no text layer, which is what a scan looks like; OCR is out of scope",
            )

        # A newline is added only where a page did not end with one, so that no word
        # of one page runs into the first word of the next. A page marker would be
        # adding something to the document; this is only separating two of them.
        joined = "".join(
            chunk if chunk.endswith("\n") else chunk + "\n" for chunk in self.chunks
        )
        body, cut = clip(joined.encode("utf-8"), self.limit)

        notes = []
        if self.blank:
            notes.append("%d of %d pages have no text layer" % (self.blank, pages))
        if self.broken:
            notes.append("%d of %d pages could not be read" % (self.broken, pages))
        if self.columns:
            notes.append("this PDF looks multi-column, so the reading order is the extractor's")

        result = ok(
            kind="pdf",
            text=body.decode("utf-8"),
            extractor=extractor,
            pages=pages,
            truncated=cut,
        )
        if notes:
            result["notes"] = notes
        return result


def pdf_with_pymupdf(path, limit):
    module = import_any(LIBRARIES["pymupdf"])
    try:
        document = module.open(path)
    except Exception as exc:
        return failed("unreadable", "the PDF could not be opened: %s" % exc)

    try:
        if document.needs_pass:
            return failed("unreadable", "the PDF is encrypted")

        pages = document.page_count
        reading = Reading(limit)
        for number, page in enumerate(document):
            try:
                reading.add(
                    page.get_text("text") or "",
                    two_column=number < COLUMN_SAMPLE and looks_two_column(page),
                )
            except Exception:
                reading.broken_page()
            if reading.full():
                break
        return reading.answer("pymupdf %s" % version_of(module), pages)
    finally:
        document.close()


def pdf(argv):
    if len(argv) != 2:
        return failed("internal", "pdf takes a path and a limit, got %d arguments" % len(argv))
    path = argv[0]
    limit = parse_limit(argv[1])
    if limit is None:
        return failed("internal", "the limit must be a positive integer, got %r" % argv[1])

    refused = sniff_pdf(path)
    if refused is not None:
        return refused

    if import_any(LIBRARIES["pymupdf"]) is None:
        return failed(
            "missing_extractor",
            "pymupdf is not importable, so no PDF can be read; "
            "`stemma env` says what this machine has",
        )
    return pdf_with_pymupdf(path, limit)


# What a response has to declare itself as to be read as a document.
HTML_TYPES = ("text/html", "application/xhtml+xml")

# How long one fetch may take. It is shorter than the wall clock Go enforces, so a
# slow server produces a sentence about the server rather than a killed process.
FETCH_TIMEOUT = 30.0

# Elements that are never the document. Dropping them is most of the difference
# between reading a page and reading its navigation.
CHROME = (
    "script",
    "style",
    "noscript",
    "nav",
    "header",
    "footer",
    "aside",
    "form",
    "button",
    "svg",
    "iframe",
    "template",
)

# Where the document is looked for, best first. The first of these a page has is
# the one whose text is taken; body is the honest last resort, and saying so is
# left to the note the caller gets.
MAIN = ("article", "main", "[role=main]", "#content", ".content", "body")


def looks_html(content_type):
    return content_type.split(";")[0].strip().lower() in HTML_TYPES


def read_at_most(response, limit):
    """A response body, or None when it is past the limit.

    Reading stops at the limit rather than after it, so a server that would stream
    a gigabyte is stopped instead of buffered.
    """
    body = bytearray()
    for chunk in response.iter_bytes():
        body.extend(chunk)
        if len(body) > limit:
            return None
    return bytes(body)


def tidy(text):
    """Collapse the whitespace an HTML parser leaves behind.

    Line breaks in markup say nothing about the document, so a run of blank lines
    becomes one and trailing spaces go. Nothing is reordered and nothing is added.
    """
    kept = []
    for line in text.splitlines():
        line = line.strip()
        if line or (kept and kept[-1]):
            kept.append(line)
    return "\n".join(kept).strip() + "\n"


def reduce_page(soup_module, body, limit):
    soup = soup_module.BeautifulSoup(body, "html.parser")
    for element in soup(list(CHROME)):
        element.decompose()

    main = None
    for selector in MAIN:
        main = soup.select_one(selector)
        if main is not None:
            break
    if main is None:
        return failed("empty", "the page has no body to read")

    text = tidy(main.get_text(separator="\n"))
    if not text.strip():
        return failed(
            "empty",
            "the page has no text outside its scripts and its navigation, "
            "which is what a page rendered by JavaScript looks like",
        )

    clipped, cut = clip(text.encode("utf-8"), limit)
    result = ok(kind="url", text=clipped.decode("utf-8"), extractor="httpx+bs4", truncated=cut)
    if main.name == "body":
        result["notes"] = ["the page has no article or main element, so the body was read"]
    return result


def url(argv):
    if len(argv) != 3:
        return failed(
            "internal", "url takes a URL, a limit and a user agent, got %d arguments" % len(argv)
        )
    address = argv[0]
    limit = parse_limit(argv[1])
    if limit is None:
        return failed("internal", "the limit must be a positive integer, got %r" % argv[1])
    agent = argv[2]

    http = import_any(LIBRARIES["httpx"])
    soup_module = import_any(LIBRARIES["bs4"])
    for name, module in (("httpx", http), ("bs4", soup_module)):
        if module is None:
            return failed(
                "missing_extractor",
                "%s is not importable, so a page cannot be read; "
                "`stemma env` says what this machine has" % name,
            )

    headers = {"User-Agent": agent, "Accept": "text/html,application/xhtml+xml"}
    try:
        with http.Client(follow_redirects=True, timeout=FETCH_TIMEOUT, headers=headers) as client:
            with client.stream("GET", address) as response:
                if not 200 <= response.status_code < 300:
                    return failed(
                        "network", "the server answered %d for %s" % (response.status_code, address)
                    )
                content_type = response.headers.get("content-type", "")
                if not looks_html(content_type):
                    return failed(
                        "unsupported",
                        "the response declares %s, which is not HTML"
                        % (content_type.split(";")[0].strip() or "no content type"),
                    )
                body = read_at_most(response, limit)
                if body is None:
                    return failed(
                        "too_large", "the page is larger than the %d bytes it was allowed" % limit
                    )
    except Exception as exc:
        return failed("network", "%s: %s" % (type(exc).__name__, exc))

    return reduce_page(soup_module, body, limit)


def main(argv):
    if not argv:
        return failed("internal", "no subcommand given")
    name, rest = argv[0], argv[1:]
    if name == "probe":
        return probe(rest)
    if name == "text":
        return text(rest)
    if name == "pdf":
        return pdf(rest)
    if name == "url":
        return url(rest)
    return failed("internal", "unknown subcommand %r" % name)


if __name__ == "__main__":
    # Written to the buffer rather than through sys.stdout, so the bytes on the
    # wire are UTF-8 whatever the locale claims the terminal is.
    encoded = json.dumps(main(sys.argv[1:]), ensure_ascii=False).encode("utf-8")
    sys.stdout.buffer.write(encoded + b"\n")

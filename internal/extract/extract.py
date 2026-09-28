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

# The libraries this contract names. probe reports on all of them, whether or not
# a subcommand uses one yet: its job is to describe the machine, because that is
# what `stemma env` has to explain.
LIBRARIES = ("pymupdf", "pypdf", "httpx", "bs4")

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


def version_of(module):
    """The version of an importable module, or None when it cannot be imported.

    Catching everything is the point rather than a shortcut: "not importable" is
    the answer the caller wants, and an import can fail in many ways that are all
    equally uninteresting here.
    """
    try:
        loaded = __import__(module)
    except Exception:
        return None
    return str(getattr(loaded, "__version__", "") or "unknown")


def probe(_argv):
    return ok(
        kind="probe",
        python="%d.%d.%d" % sys.version_info[:3],
        libs={name: version_of(name) for name in LIBRARIES},
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


def main(argv):
    if not argv:
        return failed("internal", "no subcommand given")
    name, rest = argv[0], argv[1:]
    if name == "probe":
        return probe(rest)
    if name == "text":
        return text(rest)
    return failed("internal", "unknown subcommand %r" % name)


if __name__ == "__main__":
    # Written to the buffer rather than through sys.stdout, so the bytes on the
    # wire are UTF-8 whatever the locale claims the terminal is.
    encoded = json.dumps(main(sys.argv[1:]), ensure_ascii=False).encode("utf-8")
    sys.stdout.buffer.write(encoded + b"\n")

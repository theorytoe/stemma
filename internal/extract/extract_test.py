#!/usr/bin/env python3
"""Tests for extract.py.

Run with plain python3: no framework, so the shim can be checked without a Go
build, and its behaviour is pinned independently of the binary that embeds it.
The Go side tests the boundary; this tests the reading.

    python3 internal/extract/extract_test.py
"""

import importlib.util
import json
import pathlib
import subprocess
import sys
import tempfile

HERE = pathlib.Path(__file__).resolve().parent

failures = []


def load_shim():
    spec = importlib.util.spec_from_file_location("stemma_extract", HERE / "extract.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


shim = load_shim()


def check(name, got, want):
    if got == want:
        print("ok    %s" % name)
        return
    failures.append(name)
    print("FAIL  %s\n        got  %r\n        want %r" % (name, got, want))


def check_true(name, condition, detail=""):
    if condition:
        print("ok    %s" % name)
        return
    failures.append(name)
    print("FAIL  %s %s" % (name, detail))


def write(directory, name, data):
    path = pathlib.Path(directory) / name
    if isinstance(data, bytes):
        path.write_bytes(data)
    else:
        path.write_text(data, encoding="utf-8")
    return str(path)


def case_probe():
    out = shim.main(["probe"])
    check("probe is ok", out["ok"], True)
    check("probe names itself", out["kind"], "probe")
    check("probe reports a contract", out["contract"], shim.CONTRACT)
    check_true("probe reports a version", out["python"].count(".") == 2, out["python"])
    check("probe reports every named library", sorted(out["libs"]), sorted(shim.LIBRARIES))
    for name, version in out["libs"].items():
        check_true(
            "probe's %s entry is a version or absent" % name,
            version is None or isinstance(version, str),
            repr(version),
        )


def case_probe_ignores_arguments():
    # Go passes nothing to probe, but extra arguments must not turn a diagnostic
    # into a failure.
    out = shim.main(["probe", "extra"])
    check("probe with extra arguments is still ok", out["ok"], True)


def case_text_success():
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "note.txt", "hello\nworld\n")
        out = shim.main(["text", path, "1024"])
        check("text is ok", out["ok"], True)
        check("text returns the file", out["text"], "hello\nworld\n")
        check("text names what read it", out["extractor"], "stdlib")
        check("text is not truncated", out["truncated"], False)


def case_text_cuts_on_a_line_boundary():
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "lines.txt", "line\n" * 100)
        out = shim.main(["text", path, "25"])
        check("truncated text says so", out["truncated"], True)
        check_true("truncated text is within the limit",
                   len(out["text"].encode("utf-8")) <= 25)
        check("truncated text ends at a line", out["text"], "line\n" * 5)


def case_text_does_not_split_a_character():
    with tempfile.TemporaryDirectory() as directory:
        # Each "é" is two bytes, so a limit of 11 lands inside the sixth one.
        path = write(directory, "accents.txt", "é" * 100)
        out = shim.main(["text", path, "11"])
        check("a cut never splits a character", out["text"], "é" * 5)
        check_true("and stays within the limit",
                   len(out["text"].encode("utf-8")) <= 11)


def case_text_missing_file():
    with tempfile.TemporaryDirectory() as directory:
        out = shim.main(["text", str(pathlib.Path(directory) / "absent.txt"), "1024"])
        check("a missing file is unreadable", out["error"]["class"], "unreadable")
        check("a missing file is not ok", out["ok"], False)


def case_text_empty_file():
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "empty.txt", "")
        out = shim.main(["text", path, "1024"])
        check("an empty file is empty", out["error"]["class"], "empty")


def case_text_blank_file():
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "blank.txt", "   \n\n\t\n")
        out = shim.main(["text", path, "1024"])
        check("a blank file is empty", out["error"]["class"], "empty")


def case_text_not_utf8():
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "latin.txt", b"caf\xe9\n")
        out = shim.main(["text", path, "1024"])
        check("a non-UTF-8 file is unreadable", out["error"]["class"], "unreadable")


def case_text_bad_arguments():
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "note.txt", "hello\n")
        too_few = shim.main(["text", path])
        check("a missing limit is internal", too_few["error"]["class"], "internal")
        zero = shim.main(["text", path, "0"])
        check("a zero limit is internal", zero["error"]["class"], "internal")
        negative = shim.main(["text", path, "-1"])
        check("a negative limit is internal", negative["error"]["class"], "internal")
        not_a_number = shim.main(["text", path, "lots"])
        check("a non-numeric limit is internal", not_a_number["error"]["class"], "internal")


def case_unknown_subcommand():
    check("no subcommand is internal", shim.main([])["error"]["class"], "internal")
    check("an unknown subcommand is internal",
          shim.main(["pdf", "x", "1024"])["error"]["class"], "internal")


def case_runs_as_a_process():
    # The one thing the in-process calls above cannot show: that the script exits
    # 0 and prints one parseable object, which is what Go actually relies on.
    proc = subprocess.run(
        [sys.executable, str(HERE / "extract.py"), "probe"],
        capture_output=True,
    )
    check("a run exits 0", proc.returncode, 0)
    check("a run says nothing on stderr", proc.stderr, b"")
    parsed = json.loads(proc.stdout.decode("utf-8"))
    check("a run prints the contract", parsed["contract"], shim.CONTRACT)
    # A failure is also exit 0, because the object already says what happened.
    failed_proc = subprocess.run(
        [sys.executable, str(HERE / "extract.py"), "text", "/nonexistent/x", "1024"],
        capture_output=True,
    )
    check("a reported failure still exits 0", failed_proc.returncode, 0)
    check("a reported failure prints its class",
          json.loads(failed_proc.stdout.decode("utf-8"))["error"]["class"], "unreadable")


def main():
    for name, case in sorted(globals().items()):
        if name.startswith("case_"):
            case()
    if failures:
        print("\n%d failing: %s" % (len(failures), ", ".join(failures)))
        return 1
    print("\nall good")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Tests for extract.py.

Run with plain python3: no framework, so the shim can be checked without a Go
build, and its behaviour is pinned independently of the binary that embeds it.
The Go side tests the boundary; this tests the reading.

    python3 internal/extract/extract_test.py
"""

import contextlib
import http.server
import importlib.util
import json
import pathlib
import socket
import subprocess
import sys
import tempfile
import threading

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
          shim.main(["epub", "x", "1024"])["error"]["class"], "internal")


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


# ------------------------------------------------------------------- PDF
#
# The fixtures are written by the tests rather than committed, which is the only
# way to get a page with no text layer, a page laid out in two columns, and an
# encrypted file on purpose. They need a library to build, so these cases skip
# where none is installed; the cases that need no library at all do not.

BODY = "Column text long enough to count as body text rather than a caption. "


def need_pymupdf(case):
    module = shim.import_any(shim.LIBRARIES["pymupdf"])
    if module is None:
        print("skip  %s: pymupdf is not installed" % case)
        return None
    return module


def make_pdf(path, pages, module, encryption=None):
    document = module.open()
    for sheet in pages:
        page = document.new_page()
        for rect, body in sheet:
            page.insert_textbox(module.Rect(*rect), body, fontsize=9)
    if encryption is None:
        document.save(str(path))
    else:
        document.save(
            str(path),
            encryption=getattr(module, "PDF_ENCRYPT_AES_256", 1),
            owner_pw="owner",
            user_pw="user",
        )
    document.close()
    return str(path)


def one_column(directory, name="fixture.pdf", repeat=6):
    module = need_pymupdf(name)
    if module is None:
        return None, None
    path = pathlib.Path(directory) / name
    return module, make_pdf(path, [[((40, 50, 570, 750), BODY * repeat)]], module)


def case_pdf_reads_a_pdf():
    with tempfile.TemporaryDirectory() as directory:
        module, path = one_column(directory)
        if module is None:
            return
        out = shim.main(["pdf", path, "1000000"])
        check("pdf is ok", out["ok"], True)
        check_true("pdf names what read it",
                   out["extractor"].startswith("pymupdf"), out["extractor"])
        check("pdf counts the pages", out["pages"], 1)
        check_true("pdf carries the text",
                   out["text"].count("Column") >= 3, out["text"][:80])
        check("pdf is not truncated", out["truncated"], False)


def case_pdf_truncates():
    module = need_pymupdf("pdf truncates")
    if module is None:
        return
    with tempfile.TemporaryDirectory() as directory:
        path = make_pdf(
            pathlib.Path(directory) / "long.pdf",
            [[((40, 50, 570, 750), BODY * 8)] for _ in range(3)],
            module,
        )
        out = shim.main(["pdf", path, "200"])
        check("pdf past the limit says so", out["truncated"], True)
        check_true("pdf past the limit stays within it",
                   len(out["text"].encode("utf-8")) <= 200, out["text"])
        check("pdf reports the whole document's length", out["pages"], 3)


def case_pdf_with_no_text_layer():
    module = need_pymupdf("pdf with no text layer")
    if module is None:
        return
    with tempfile.TemporaryDirectory() as directory:
        # A page that was never written to is what a scan looks like to an
        # extractor: a real page with no text layer.
        path = make_pdf(pathlib.Path(directory) / "blank.pdf", [[]], module)
        out = shim.main(["pdf", path, "1000"])
        check("a PDF with no text is empty", out["error"]["class"], "empty")
        check_true("and says what that means",
                   "OCR" in out["error"]["message"], out["error"]["message"])


def case_pdf_notes_a_page_with_no_text_layer():
    module = need_pymupdf("pdf notes a blank page")
    if module is None:
        return
    with tempfile.TemporaryDirectory() as directory:
        path = make_pdf(
            pathlib.Path(directory) / "mixed.pdf",
            [[((40, 50, 570, 750), BODY * 6)], []],
            module,
        )
        out = shim.main(["pdf", path, "1000000"])
        check("a partly blank PDF is ok", out["ok"], True)
        check_true(
            "and notes the page with no text layer",
            any("no text layer" in note for note in out.get("notes", [])),
            out.get("notes"),
        )


def case_pdf_counts_the_pages_it_read_and_not_the_document():
    module = need_pymupdf("pdf counts the pages it read")
    if module is None:
        return
    with tempfile.TemporaryDirectory() as directory:
        # A blank first page and four pages after it, read with a limit small enough
        # that the read stops almost at once. The blank page adds no characters, so
        # the loop passes one more page before it is past the limit: two pages are
        # looked at and one of them is blank. The note used to say "1 of 5 pages",
        # which is a survey of a document that was never read.
        path = make_pdf(
            pathlib.Path(directory) / "long.pdf",
            [[]] + [[((40, 50, 570, 750), BODY * 6)]] * 4,
            module,
        )
        out = shim.main(["pdf", path, "10"])
        check("a PDF stopped at the limit is ok", out["ok"], True)
        check("and says it stopped", out["truncated"], True)
        check("and still reports the document's length", out["pages"], 5)
        check_true(
            "and counts the pages it read in the note",
            any("1 of 2 pages" in note for note in out.get("notes", [])),
            out.get("notes"),
        )


def case_pdf_flags_a_two_column_page():
    module = need_pymupdf("pdf flags two columns")
    if module is None:
        return
    with tempfile.TemporaryDirectory() as directory:
        path = make_pdf(
            pathlib.Path(directory) / "columns.pdf",
            [[((40, 50, 300, 750), BODY * 5), ((310, 50, 570, 750), BODY * 5)]],
            module,
        )
        out = shim.main(["pdf", path, "1000000"])
        check("a two-column PDF is ok", out["ok"], True)
        check_true(
            "and says the order may be the extractor's",
            any("multi-column" in note for note in out.get("notes", [])),
            out.get("notes"),
        )


def case_pdf_refuses_an_encrypted_file():
    module = need_pymupdf("pdf refuses encryption")
    if module is None:
        return
    with tempfile.TemporaryDirectory() as directory:
        path = make_pdf(
            pathlib.Path(directory) / "locked.pdf",
            [[((40, 50, 570, 750), BODY * 6)]],
            module,
            encryption=True,
        )
        out = shim.main(["pdf", path, "1000000"])
        check("an encrypted PDF is unreadable", out["error"]["class"], "unreadable")
        check_true("and says it is encrypted",
                   "encrypted" in out["error"]["message"], out["error"]["message"])


def case_pdf_refuses_what_is_not_a_pdf():
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "notes.pdf", "this is not a PDF at all\n")
        out = shim.main(["pdf", path, "1024"])
        check("a file that is not a PDF is unreadable", out["error"]["class"], "unreadable")
        check_true("and says what is missing",
                   "%PDF-" in out["error"]["message"], out["error"]["message"])


def case_pdf_refuses_a_zip_container():
    # EPUB is explicitly out of scope, and pymupdf would happily read one. The file
    # is judged by its own bytes instead of by the library that happens to be here.
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "book.pdf", b"PK\x03\x04" + b"\x00" * 60)
        out = shim.main(["pdf", path, "1024"])
        check("a zip container is unsupported", out["error"]["class"], "unsupported")
        check_true("and names EPUB",
                   "EPUB" in out["error"]["message"], out["error"]["message"])


def case_text_refuses_a_zip_container():
    # The same file handed to the text reader, which is what a command that only
    # knows "this is not a PDF" will do. Without this the answer would be "not
    # UTF-8 text", which names the encoding rather than the format.
    with tempfile.TemporaryDirectory() as directory:
        path = write(directory, "book.epub", b"PK\x03\x04" + b"\x00" * 60)
        out = shim.main(["text", path, "1024"])
        check("a zip container is unsupported here too",
              out["error"]["class"], "unsupported")
        check_true("and still names EPUB",
                   "EPUB" in out["error"]["message"], out["error"]["message"])


def case_pdf_without_pymupdf():
    # No fixture needed: the sniff passes, and then there is nothing that can read
    # the file. PyMuPDF is the only PDF reader the shim uses, so on a machine that
    # has it, taking it away is the only way to see this outcome at all.
    saved = dict(shim.LIBRARIES)
    shim.LIBRARIES["pymupdf"] = ()
    try:
        with tempfile.TemporaryDirectory() as directory:
            path = write(directory, "bare.pdf", b"%PDF-1.4\n% nothing here will read this\n")
            out = shim.main(["pdf", path, "1024"])
    finally:
        shim.LIBRARIES.clear()
        shim.LIBRARIES.update(saved)
    check("pdf without pymupdf is missing_extractor",
          out["error"]["class"], "missing_extractor")
    check_true("and names pymupdf", "pymupdf" in out["error"]["message"],
               out["error"]["message"])


# ------------------------------------------------------------------- URL
#
# These are served by a real HTTP server on a loopback port, because the failure
# classes here are all about what a server does: a status code, a content type, a
# body that will not stop. A fake response would only test the fake.
#
# Fetching needs both libraries, so the cases that fetch skip where either is
# absent, the same way the PDF cases skip without pymupdf. The cases that only
# check argument handling or the missing-library path fetch nothing, and so run
# on a machine with neither library.


def need_http_libraries(case):
    missing = [
        name for name in ("httpx", "bs4") if shim.import_any(shim.LIBRARIES[name]) is None
    ]
    if missing:
        print(
            "skip  %s: %s %s not installed"
            % (case, " and ".join(missing), "are" if len(missing) > 1 else "is")
        )
        return False
    return True


PAGE = """<!doctype html>
<html><head><title>A Test Page</title></head>
<body>
<header><nav><a href="/">Home</a> <a href="/about">About the Site</a></nav></header>
<article><h1>The Article</h1><p>%s</p></article>
<footer>Copyright nobody</footer>
</body></html>""" % (BODY * 3)

PLAIN = """<!doctype html><html><body><div><p>%s</p></div></body></html>""" % BODY

SCRIPTED = (
    "<!doctype html><html><body><div id='root'></div>"
    "<script>document.write('rendered later')</script></body></html>"
)

AGENT = "stemma/test"

LAST_REQUEST = {}


class PageHandler(http.server.BaseHTTPRequestHandler):
    ROUTES = {
        "/page": ("text/html; charset=utf-8", PAGE),
        "/plain": ("text/html", PLAIN),
        "/scripted": ("text/html", SCRIPTED),
        "/data": ("application/json", '{"not": "html"}'),
        "/huge": ("text/html", "x" * 40000),
    }

    def do_GET(self):
        LAST_REQUEST["user-agent"] = self.headers.get("User-Agent", "")
        if self.path in self.ROUTES:
            status, kind = 200, None
            kind, text = self.ROUTES[self.path]
            body = text.encode("utf-8")
        else:
            status, kind, body = 404, "text/html", b"missing"
        self.send_response(status)
        self.send_header("Content-Type", kind)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass  # a test server's chatter is not test output


@contextlib.contextmanager
def serving():
    server = http.server.HTTPServer(("127.0.0.1", 0), PageHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:%d" % server.server_address[1]
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def free_port():
    # A port nothing is listening on, for the refused-connection case. Closing the
    # socket is what frees it; a test racing on this would be a test asserting the
    # wrong thing anyway.
    with socket.socket() as probe:
        probe.bind(("127.0.0.1", 0))
        return probe.getsockname()[1]


def case_url_reads_the_main_content():
    if not need_http_libraries("url reads the main content"):
        return
    with serving() as base:
        out = shim.main(["url", base + "/page", "1000000", AGENT])
        check("url is ok", out["ok"], True)
        check("url names its pipeline", out["extractor"], "httpx+bs4")
        check_true("url keeps the article",
                   "The Article" in out["text"], out["text"][:120])
        check_true("url drops the navigation",
                   "About the Site" not in out["text"], out["text"][:200])
        check_true("url drops the footer",
                   "Copyright nobody" not in out["text"], out["text"][:200])
        check("url is not truncated", out["truncated"], False)


def case_url_sends_the_user_agent_it_was_given():
    if not need_http_libraries("url sends the user agent it was given"):
        return
    with serving() as base:
        shim.main(["url", base + "/page", "1000000", AGENT])
    check("url sends the user agent it was given", LAST_REQUEST.get("user-agent"), AGENT)


def case_url_notes_a_page_with_no_article():
    if not need_http_libraries("url notes a page with no article"):
        return
    with serving() as base:
        out = shim.main(["url", base + "/plain", "1000000", AGENT])
        check("a page with no main element is still ok", out["ok"], True)
        check_true(
            "and says the body was read",
            any("no article" in note for note in out.get("notes", [])),
            out.get("notes"),
        )


def case_url_reports_a_page_that_needs_javascript():
    if not need_http_libraries("url reports a page that needs javascript"):
        return
    with serving() as base:
        out = shim.main(["url", base + "/scripted", "1000000", AGENT])
        check("a page with no server-rendered text is empty", out["error"]["class"], "empty")
        check_true("and says JavaScript is why",
                   "JavaScript" in out["error"]["message"], out["error"]["message"])


def case_url_refuses_a_response_that_is_not_html():
    if not need_http_libraries("url refuses a response that is not html"):
        return
    with serving() as base:
        out = shim.main(["url", base + "/data", "1000000", AGENT])
        check("a JSON response is unsupported", out["error"]["class"], "unsupported")
        check_true("and names the content type",
                   "application/json" in out["error"]["message"], out["error"]["message"])


def case_url_reports_a_bad_status():
    if not need_http_libraries("url reports a bad status"):
        return
    with serving() as base:
        out = shim.main(["url", base + "/absent", "1000000", AGENT])
        check("a 404 is a network failure", out["error"]["class"], "network")
        check_true("and names the status",
                   "404" in out["error"]["message"], out["error"]["message"])


def case_url_refuses_a_page_past_the_limit():
    if not need_http_libraries("url refuses a page past the limit"):
        return
    with serving() as base:
        out = shim.main(["url", base + "/huge", "1024", AGENT])
        check("a page past the limit is too_large", out["error"]["class"], "too_large")


def case_url_reports_a_connection_that_fails():
    if not need_http_libraries("url reports a connection that fails"):
        return
    out = shim.main(["url", "http://127.0.0.1:%d/" % free_port(), "1000000", AGENT])
    check("a refused connection is a network failure", out["error"]["class"], "network")


def case_url_without_its_libraries():
    # No server needed: the libraries are checked before anything is fetched, which
    # is also what keeps this case runnable on a machine that has neither. The
    # library that is not under test is stubbed with a module that always imports,
    # so the message names the one under test rather than whichever happens to be
    # missing on this machine.
    for missing in ("httpx", "bs4"):
        other = "bs4" if missing == "httpx" else "httpx"
        saved = dict(shim.LIBRARIES)
        shim.LIBRARIES[missing] = ()
        shim.LIBRARIES[other] = ("sys",)
        try:
            out = shim.main(["url", "http://127.0.0.1:1/", "1024", AGENT])
        finally:
            shim.LIBRARIES.clear()
            shim.LIBRARIES.update(saved)
        check("url without %s is missing_extractor" % missing,
              out["error"]["class"], "missing_extractor")
        check_true("and names %s" % missing,
                   missing in out["error"]["message"], out["error"]["message"])


def case_url_without_a_user_agent():
    out = shim.main(["url", "http://127.0.0.1:1/", "1024"])
    check("url without a user agent is internal", out["error"]["class"], "internal")


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

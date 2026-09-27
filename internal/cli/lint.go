package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/theorytoe/stemma/internal/kb"
)

// runLint implements `stemma lint`.
//
// It reports findings and returns exit code 1; a clean run returns 0; a KB that
// cannot be read returns 2. Findings are on stdout so they can be piped, and
// the summary is on stderr so that it does not end up in the pipe.
func runLint(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("stemma lint", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts options
	opts.register(fs)
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "stemma: lint takes no arguments, got %q\n", fs.Arg(0))
		return ExitError
	}

	k, code := opts.load(stderr)
	if k == nil {
		return code
	}

	findings := k.Lint(opts.mode())
	if opts.json {
		return reportJSON(stdout, stderr, opts.strict, findings)
	}
	return reportText(stdout, stderr, findings)
}

// jsonFinding is one finding as it appears in --json output. Field and Line are
// omitted rather than zeroed when they do not apply, so a consumer can tell
// "no line" from "line 0".
type jsonFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

// jsonReport is the whole document a --json run writes. Findings is always a
// list, never null, because a consumer should not have to special-case an empty
// KB.
type jsonReport struct {
	Strict   bool          `json:"strict"`
	Clean    bool          `json:"clean"`
	Warnings int           `json:"warnings"`
	Errors   int           `json:"errors"`
	Findings []jsonFinding `json:"findings"`
}

func reportJSON(stdout, stderr io.Writer, strict bool, findings []kb.Finding) int {
	report := jsonReport{
		Strict:   strict,
		Clean:    len(findings) == 0,
		Findings: make([]jsonFinding, 0, len(findings)),
	}
	for _, f := range findings {
		report.Findings = append(report.Findings, jsonFinding{
			Severity: string(f.Severity),
			Code:     f.Code,
			Path:     f.Path,
			Line:     f.Line,
			Field:    f.Field,
			Message:  f.Message,
		})
		if f.Severity == kb.Error {
			report.Errors++
		} else {
			report.Warnings++
		}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(stderr, "stemma: %v\n", err)
		return ExitError
	}
	if len(findings) == 0 {
		return ExitOK
	}
	return ExitFindings
}

func reportText(stdout, stderr io.Writer, findings []kb.Finding) int {
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "clean")
		return ExitOK
	}
	for _, f := range findings {
		fmt.Fprintf(stdout, "%s: %s: %s\n", location(f), f.Severity, f.Message)
	}
	fmt.Fprintf(stderr, "stemma: %s\n", tally(findings))
	return ExitFindings
}

// location renders a finding's position the way a compiler does, so that an
// editor can jump straight to it. A finding about a whole file, or about a
// field that is missing and so has no line, is named by its file alone.
func location(f kb.Finding) string {
	where := f.Path
	if where == "" {
		where = "stemma"
	}
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d", where, f.Line)
	}
	return where
}

func tally(findings []kb.Finding) string {
	warnings, errors := 0, 0
	for _, f := range findings {
		if f.Severity == kb.Error {
			errors++
			continue
		}
		warnings++
	}
	return fmt.Sprintf("%s: %s, %s", count(len(findings), "finding"),
		count(warnings, "warning"), count(errors, "error"))
}

func count(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

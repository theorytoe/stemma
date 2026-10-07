package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/theorytoe/stemma/internal/kb"
)

// lintCommand implements `stemma lint`.
//
// It reports findings and returns exit code 1; a clean run returns 0; a KB that
// cannot be read returns 2. Findings are on stdout so they can be piped, and
// the summary is on stderr so that it does not end up in the pipe.
var lintCommand = &command{
	name:    "lint",
	summary: "report everything wrong with the KB",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("lint takes no arguments, got %q", args[0]))
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}

			findings := k.Lint(o.mode())
			if w.json {
				return w.report(SummariseLint(o.strict, findings), findings)
			}
			return reportText(w.stdout, w.stderr, findings)
		}
	},
}

// LintSummary is the payload of a --json lint, without the findings themselves.
// The findings travel in the envelope, where every consumer looks for them.
type LintSummary struct {
	Strict   bool `json:"strict"`
	Clean    bool `json:"clean"`
	Warnings int  `json:"warnings"`
	Errors   int  `json:"errors"`
}

func SummariseLint(strict bool, findings []kb.Finding) LintSummary {
	s := LintSummary{Strict: strict, Clean: len(findings) == 0}
	for _, f := range findings {
		if f.Severity == kb.Error {
			s.Errors++
			continue
		}
		s.Warnings++
	}
	return s
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

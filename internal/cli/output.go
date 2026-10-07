package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/theorytoe/stemma/internal/kb"
)

// Response is the envelope every --json run writes.
//
// There is one shape for the whole surface, so a consumer learns it once:
//
//	{ "command": "lint", "ok": false,
//	  "data": {...}, "findings": [...], "error": "..." }
//
// command names the verb, which matters for a noun family where the envelope
// alone would not say which member ran.
//
// ok is true only for a clean run. Validation findings leave it false and set
// exit code 1; an operational failure leaves it false and sets Error and exit
// code 2. Data is the command's own payload, and is absent for a failure.
type Response struct {
	Command  string        `json:"command"`
	OK       bool          `json:"ok"`
	Data     any           `json:"data,omitempty"`
	Findings []JSONFinding `json:"findings,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// JSONFinding is one finding as it appears in --json output. Field and Line are
// omitted rather than zeroed when they do not apply, so a consumer can tell
// "no line" from "line 0".
type JSONFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

// output is where one command writes. It is created after the arguments are
// parsed, so it knows whether the run asked for JSON and can keep the two
// streams honest: a --json run writes nothing but JSON, and writes it to
// stdout.
type output struct {
	cmd    string
	json   bool
	stdout io.Writer
	stderr io.Writer
}

// emit writes a successful payload. In text mode the caller has already printed
// what a person reads, so emit does nothing.
func (w *output) emit(data any) int {
	if !w.json {
		return ExitOK
	}
	return w.write(Response{Command: w.cmd, OK: true, Data: data})
}

// report writes findings and the payload they are about, returning exit code 1
// when there are any. In text mode the caller prints the findings itself.
func (w *output) report(data any, findings []kb.Finding) int {
	if !w.json {
		if len(findings) == 0 {
			return ExitOK
		}
		return ExitFindings
	}
	resp := Response{Command: w.cmd, OK: len(findings) == 0, Data: data}
	if len(findings) > 0 {
		resp.Findings = make([]JSONFinding, 0, len(findings))
		for _, f := range findings {
			resp.Findings = append(resp.Findings, JSONFindingOf(f))
		}
	}
	if code := w.write(resp); code != ExitOK {
		return code
	}
	if len(findings) == 0 {
		return ExitOK
	}
	return ExitFindings
}

// fail reports an operational failure. Under --json the error travels in the
// envelope on stdout and stderr stays empty, so the JSON stream is the whole
// story; without it the error is one line on stderr, the way every other tool
// writes one.
func (w *output) fail(err error) int {
	if w.json {
		if code := w.write(Response{Command: w.cmd, OK: false, Error: err.Error()}); code != ExitOK {
			return code
		}
		return ExitError
	}
	fmt.Fprintf(w.stderr, "stemma: %v\n", err)
	return ExitError
}

// write encodes one envelope. It never re-enters itself on a write failure,
// because a broken pipe would then fail forever.
func (w *output) write(resp Response) int {
	enc := json.NewEncoder(w.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		fmt.Fprintf(w.stderr, "stemma: %v\n", err)
		return ExitError
	}
	return ExitOK
}

func JSONFindingOf(f kb.Finding) JSONFinding {
	return JSONFinding{
		Severity: string(f.Severity),
		Code:     f.Code,
		Path:     f.Path,
		Line:     f.Line,
		Field:    f.Field,
		Message:  f.Message,
	}
}

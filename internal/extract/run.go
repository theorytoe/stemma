package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/theorytoe/stemma/internal/version"
)

// contract is the version of the shim interface this binary speaks. It has to
// match the script's, which is what turns a stale copy into a sentence rather
// than a mystery.
const contract = 1

// probeKind is the one successful answer that describes the machine instead of a
// document, and so the one that carries no text.
const probeKind = "probe"

const (
	// DefaultLimit is how much text a caller gets when it does not choose. It is
	// a bound on three things at once: the memory the text costs in two
	// processes, the size of what a reader is handed, and the work the shim does
	// before it can stop reading.
	DefaultLimit = 8 << 20

	// DefaultTimeout bounds one extraction when a caller does not choose. A fetch
	// and a parse are quick, and a server that has not answered in a minute is not
	// going to.
	DefaultTimeout = 60 * time.Second

	// pdfTimeout bounds reading a PDF, which is legitimately slower: linearising a
	// book is work, not a failure.
	pdfTimeout = 300 * time.Second

	// slack is how far past the limit this side will read before deciding the
	// answer is oversized. It is not zero because the object is larger than the
	// text inside it: JSON escaping costs something, and the envelope costs a
	// fixed amount.
	slack = 64 << 10

	// escapeWorst is the most the text can grow on its way through JSON. The shim
	// writes UTF-8 rather than ASCII escapes, so an ordinary character costs what
	// it costs, but a quote, a backslash or a newline becomes two bytes, and any
	// other control character becomes six. The drain has to allow for that because
	// the limit bounds the text and not the envelope carrying it: sized from the
	// limit alone, a document that is largely quotation is refused as a shim that
	// printed too much, which is a reader being told the tool is broken.
	escapeWorst = 6

	// stderrKept bounds what is quoted from a shim that died. A traceback is
	// useful; a runaway log is not.
	stderrKept = 8 << 10
)

// Kind is what went wrong, and the only thing a caller needs to branch on.
//
// The first seven are the shim's own classes, reported in the object it prints.
// The last three are what this side adds when the shim could not report anything
// at all — an absence the shim cannot describe, because describing it would mean
// running.
type Kind string

const (
	KindUnsupported      Kind = "unsupported"
	KindMissingExtractor Kind = "missing_extractor"
	KindUnreadable       Kind = "unreadable"
	KindEmpty            Kind = "empty"
	KindNetwork          Kind = "network"
	KindTooLarge         Kind = "too_large"
	KindInternal         Kind = "internal"

	KindMissingPython Kind = "missing_python"
	KindTimeout       Kind = "timeout"
	KindCrashed       Kind = "crashed"
)

// Error is a failed extraction.
type Error struct {
	// Kind is what went wrong, and what a caller branches on.
	Kind Kind
	// Message is what the shim said, when it said something. It is for a person
	// and is never parsed.
	Message string
	// Detail is what a dying shim wrote to stderr.
	Detail string
	// Err is the underlying failure, when there is one.
	Err error
}

func (e *Error) Error() string {
	message := e.Message
	if message == "" && e.Err != nil {
		message = e.Err.Error()
	}
	if message == "" {
		message = "extraction failed"
	}
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s\n%s", e.Kind, message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Kind, message)
}

func (e *Error) Unwrap() error { return e.Err }

// Result is one successful extraction.
type Result struct {
	// Kind is which subcommand ran: text, pdf or url.
	Kind string
	// Text is the document's text. It is never empty: the boundary refuses an empty
	// answer rather than passing one on.
	Text string
	// Extractor is what read it.
	Extractor string
	// Pages is set for a PDF.
	Pages int
	// Truncated says whether Text was cut at the limit.
	Truncated bool
	// Notes are facts about the input that did not stop extraction.
	Notes []string
}

// Probe is what this machine can do.
type Probe struct {
	// Python is the interpreter's version.
	Python string
	// Libs maps a module to its version. A module that is absent, or present
	// with an empty version, is not importable.
	Libs map[string]string
}

// Version is the version of a module, and "" when it is not importable.
func (p *Probe) Version(module string) string { return p.Libs[module] }

// Has reports whether a module is importable.
func (p *Probe) Has(module string) bool { return p.Libs[module] != "" }

// Runner runs the shim.
//
// Everything needed to invoke it lives here, which is what lets a test point it
// at a shell script: the whole boundary can then be exercised on a machine with
// no Python, and that machine is the one the core promises to work on.
type Runner struct {
	// Python is the interpreter. Empty means find one.
	Python string
	// Script is the script to run.
	Script string
	// Limit is how many bytes of text to ask for. Zero means DefaultLimit.
	Limit int
	// Timeout bounds one extraction. Zero means DefaultTimeout.
	Timeout time.Duration
}

// Probe asks what this machine can do. It reports rather than fails, so a
// missing library comes back as a library with no version.
func (r *Runner) Probe(ctx context.Context) (*Probe, error) {
	env, err := r.call(ctx, DefaultTimeout, "probe")
	if err != nil {
		return nil, err
	}
	return &Probe{Python: env.Python, Libs: env.Libs}, nil
}

// Text reads a file as UTF-8 text.
func (r *Runner) Text(ctx context.Context, path string) (*Result, error) {
	env, err := r.call(ctx, DefaultTimeout, "text", path, strconv.Itoa(r.limit()))
	if err != nil {
		return nil, err
	}
	return env.result(), nil
}

// PDF reads a PDF.
func (r *Runner) PDF(ctx context.Context, path string) (*Result, error) {
	env, err := r.call(ctx, pdfTimeout, "pdf", path, strconv.Itoa(r.limit()))
	if err != nil {
		return nil, err
	}
	return env.result(), nil
}

// URL fetches a page and reads its main content.
//
// The user agent is passed in rather than chosen here, because the tool already
// has one and a second spelling would be a second thing to keep true.
func (r *Runner) URL(ctx context.Context, address string) (*Result, error) {
	env, err := r.call(ctx, DefaultTimeout, "url", address, strconv.Itoa(r.limit()), version.UserAgent())
	if err != nil {
		return nil, err
	}
	return env.result(), nil
}

// FindPython locates the interpreter the shim runs under.
//
// Python is never a dependency of the core, so this is only reached on a path
// that has already decided to read a document. When there is no interpreter the
// answer is a typed failure rather than a workaround, and it says how to change
// the answer: the interpreter is looked up on PATH like any other tool, so
// activating the environment that has one is what fixes it.
func FindPython() (string, error) {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", &Error{
		Kind: KindMissingPython,
		Err: errors.New("no python3 on PATH, and reading a document needs an interpreter; " +
			"activate the environment that has one, and `stemma env` says what this machine can see"),
	}
}

// envelope is the one object the shim prints, success or failure.
//
// It is one struct rather than several because "one shape, branch on one field"
// is the contract, and a second decoder would be a second thing to keep true.
// The probe fields are simply absent from every other answer.
type envelope struct {
	Contract  int               `json:"contract"`
	OK        bool              `json:"ok"`
	Kind      string            `json:"kind"`
	Text      string            `json:"text"`
	Extractor string            `json:"extractor"`
	Pages     int               `json:"pages"`
	Truncated bool              `json:"truncated"`
	Notes     []string          `json:"notes"`
	Python    string            `json:"python"`
	Libs      map[string]string `json:"libs"`
	Error     *struct {
		Class   string `json:"class"`
		Message string `json:"message"`
	} `json:"error"`
}

func (e *envelope) result() *Result {
	return &Result{
		Kind:      e.Kind,
		Text:      e.Text,
		Extractor: e.Extractor,
		Pages:     e.Pages,
		Truncated: e.Truncated,
		Notes:     e.Notes,
	}
}

// call runs one subcommand and returns the object it printed, or the failure it
// described. fallback is the wall-clock bound for this family, used when the
// caller did not set one.
func (r *Runner) call(ctx context.Context, fallback time.Duration, args ...string) (*envelope, error) {
	python := r.Python
	if python == "" {
		found, err := FindPython()
		if err != nil {
			return nil, err
		}
		python = found
	}

	budget := r.timeout(fallback)
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	stdout := &sink{room: int64(r.limit())*escapeWorst + slack}
	stderr := &sink{room: stderrKept}
	cmd := exec.CommandContext(ctx, python, append([]string{r.Script}, args...)...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return nil, &Error{
			Kind: KindTimeout,
			Err:  fmt.Errorf("the extractor did not finish within %s", budget),
		}
	case errors.Is(err, exec.ErrNotFound), errors.Is(err, os.ErrNotExist):
		return nil, &Error{
			Kind: KindMissingPython,
			Err:  fmt.Errorf("%s could not be run: %v", python, err),
		}
	case err != nil:
		// It died. Whatever is on stdout is not an answer, and what it said on
		// stderr is the most useful thing anyone can be shown.
		return nil, &Error{Kind: KindCrashed, Err: err, Detail: stderr.String()}
	}

	if stdout.overflowed() {
		return nil, &Error{
			Kind: KindInternal,
			Err: fmt.Errorf("the shim printed more than the %d bytes an answer may hold, which is %d times the %d byte limit",
				stdout.room, escapeWorst, r.limit()),
		}
	}
	return parse(stdout.bytes())
}

// parse turns the shim's one object into an answer, or into the failure it
// described.
func parse(raw []byte) (*envelope, error) {
	var env envelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&env); err != nil {
		return nil, &Error{
			Kind: KindCrashed,
			Err:  fmt.Errorf("the shim printed no JSON object: %v", err),
		}
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, &Error{
			Kind: KindCrashed,
			Err:  errors.New("the shim printed more than one object"),
		}
	}
	if env.Contract != contract {
		return nil, &Error{
			Kind: KindCrashed,
			Err: fmt.Errorf("the shim speaks contract %d and this binary expects %d, so the script is out of date",
				env.Contract, contract),
		}
	}
	if env.OK {
		// A reading subcommand that answers with nothing is not an answer. The shim has
		// a class for having found no text, so this is a shim disagreeing with its own
		// contract, and passing it on would write an empty capture into the KB and
		// report success. A probe is the one answer that is about the machine rather
		// than about a document, and so the one that has no text in it.
		if env.Kind != probeKind && env.Text == "" {
			return nil, &Error{
				Kind: KindInternal,
				Err:  errors.New("the shim reported success and sent no text"),
			}
		}
		return &env, nil
	}
	if env.Error == nil {
		return nil, &Error{
			Kind: KindCrashed,
			Err:  errors.New("the shim reported a failure without saying what failed"),
		}
	}

	kind, known := classKind(env.Error.Class)
	if !known {
		return nil, &Error{
			Kind:    KindInternal,
			Message: fmt.Sprintf("the shim reported an unknown class %q: %s", env.Error.Class, env.Error.Message),
		}
	}
	return nil, &Error{Kind: kind, Message: env.Error.Message}
}

// classKind maps a class the shim may report onto a Kind. A class this binary
// does not know is not guessed at: it becomes internal, and the message says
// which one arrived.
func classKind(class string) (Kind, bool) {
	switch Kind(class) {
	case KindUnsupported, KindMissingExtractor, KindUnreadable, KindEmpty,
		KindNetwork, KindTooLarge, KindInternal:
		return Kind(class), true
	}
	return KindInternal, false
}

// sink collects output up to a limit and remembers whether more arrived.
//
// It always reports a successful write, because it is a drain rather than a
// gate: the point is to stop a misbehaving shim exhausting this process, not to
// make it fail while doing so.
type sink struct {
	room int64
	buf  bytes.Buffer
	over bool
}

func (s *sink) Write(p []byte) (int, error) {
	if take := s.room - int64(s.buf.Len()); take > 0 {
		if int64(len(p)) <= take {
			s.buf.Write(p)
		} else {
			s.buf.Write(p[:take])
			s.over = true
		}
	} else if len(p) > 0 {
		s.over = true
	}
	return len(p), nil
}

func (s *sink) String() string   { return s.buf.String() }
func (s *sink) bytes() []byte    { return s.buf.Bytes() }
func (s *sink) overflowed() bool { return s.over }

func (r *Runner) limit() int {
	if r.Limit > 0 {
		return r.Limit
	}
	return DefaultLimit
}

func (r *Runner) timeout(fallback time.Duration) time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return fallback
}

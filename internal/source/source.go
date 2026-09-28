// Package source turns an identifier into a bibliography entry.
//
// Resolution is deterministic and network-bound: the tool asks an authoritative
// resolver for a record and copies down what it says. It never writes a record
// from memory, because a bibliography exists to keep a model's confident
// invention out of the citation record (D37). When the network is not available
// or not wanted, a person supplies the metadata by hand through Manual, and the
// tool keeps it exactly as given.
//
// Nothing here writes prose and nothing here reads a document's text. That
// boundary belongs to extraction; this package stops at the record.
package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/theorytoe/stemma/internal/kb"
)

// ErrorKind says what sort of failure resolution hit, so a caller can decide an
// exit code without reading the message.
type ErrorKind int

const (
	// ErrNotFound means the identifier is well formed but names nothing.
	ErrNotFound ErrorKind = iota
	// ErrInvalid means the identifier itself is malformed.
	ErrInvalid
	// ErrOffline means the network was not used, because it was not wanted.
	ErrOffline
	// ErrNetwork means the service could not be reached, or failed to answer.
	ErrNetwork
)

func (k ErrorKind) String() string {
	switch k {
	case ErrNotFound:
		return "not-found"
	case ErrInvalid:
		return "invalid"
	case ErrOffline:
		return "offline"
	case ErrNetwork:
		return "network"
	}
	return "unknown"
}

// Error is a resolution failure. It carries the identifier and, for an
// operational failure, the error underneath, so errors.Is and errors.As still
// see through it.
type Error struct {
	Kind ErrorKind
	ID   string
	Err  error
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrNotFound:
		return fmt.Sprintf("%s: %v", e.ID, e.Err)
	case ErrInvalid:
		return fmt.Sprintf("%s: %v", e.ID, e.Err)
	case ErrOffline:
		return fmt.Sprintf("%s: resolution is offline, so the record was not fetched; enter it by hand instead", e.ID)
	default:
		return fmt.Sprintf("%s: %v", e.ID, e.Err)
	}
}

func (e *Error) Unwrap() error { return e.Err }

// KindOf reports the kind of a resolution failure.
func KindOf(err error) (ErrorKind, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind, true
	}
	return 0, false
}

// Operational reports whether a failure is one the run could not complete
// rather than one about what was asked for. An operational failure is exit code
// 2; everything else is a finding about the input.
func Operational(err error) bool {
	k, ok := KindOf(err)
	return ok && (k == ErrOffline || k == ErrNetwork)
}

func notFound(id, format string, args ...any) *Error {
	return &Error{Kind: ErrNotFound, ID: id, Err: fmt.Errorf(format, args...)}
}

func invalid(id, format string, args ...any) *Error {
	return &Error{Kind: ErrInvalid, ID: id, Err: fmt.Errorf(format, args...)}
}

func network(id string, err error) *Error {
	return &Error{Kind: ErrNetwork, ID: id, Err: err}
}

func offline(id string) *Error {
	return &Error{Kind: ErrOffline, ID: id}
}

// Endpoints are the services resolution reads from. They are fields rather than
// constants so that a test can point them at a local server and so that a
// future manifest can move one.
type Endpoints struct {
	DOI         string // DOI content negotiation, agency-agnostic
	ArXiv       string // arXiv's BibTeX export
	OpenLibrary string // Open Library's edition records
}

// DefaultEndpoints are the public services resolution uses.
func DefaultEndpoints() Endpoints {
	return Endpoints{
		DOI:         "https://doi.org",
		ArXiv:       "https://arxiv.org",
		OpenLibrary: "https://openlibrary.org",
	}
}

// Resolver turns identifiers into entries.
//
// Everything it needs that touches the outside world is a field, so the whole
// type is testable without a network and so a caller can say "do not reach out"
// once, with Offline, instead of at every call site.
type Resolver struct {
	Client    *Client
	Endpoints Endpoints
	// Offline refuses to use the network at all. Resolution then fails with
	// ErrOffline immediately, which is what makes "no unrequested network" a
	// mode rather than an accident.
	Offline bool
	// Now is the clock the retrieval date is stamped with. A test sets it.
	Now func() time.Time
}

// New returns a resolver with the default client, endpoints and clock.
func New() *Resolver {
	return &Resolver{
		Client:    NewClient(),
		Endpoints: DefaultEndpoints(),
		Now:       time.Now,
	}
}

// Result is a resolved record together with the provenance that came with it.
//
// Record is the bytes the resolver actually returned — the BibTeX text, the
// edition JSON, the page — which is what a content hash covers. Keeping it here
// means the caller that records provenance neither fetches anything a second
// time nor has to reconstruct what the hash should be over.
type Result struct {
	Entry     *kb.BibEntry
	Kind      Kind
	URL       string
	Record    []byte
	Retrieved time.Time
}

// Resolve reads the record an identifier names. See ResolveResult when the
// provenance matters as well.
func (r *Resolver) Resolve(ctx context.Context, id string) (*kb.BibEntry, error) {
	res, err := r.ResolveResult(ctx, id)
	if err != nil {
		return nil, err
	}
	return res.Entry, nil
}

// ResolveResult reads the record an identifier names, along with the bytes it
// came from and the moment it was read.
//
// It classifies the identifier, refuses to guess at one it does not know, and
// dispatches to the service that owns that kind. The entry it returns carries
// the retrieval date in stemma-retrieved, so provenance is recorded at the same
// moment the record is.
func (r *Resolver) ResolveResult(ctx context.Context, id string) (*Result, error) {
	id = strings.TrimSpace(id)
	kind := Detect(id)
	if kind == KindUnknown {
		return nil, invalid(id, "not a DOI, arXiv identifier, ISBN or URL")
	}
	if r.Offline {
		return nil, offline(id)
	}
	switch kind {
	case KindDOI:
		return r.resolveDOI(ctx, id)
	case KindarXiv:
		return r.resolvearXiv(ctx, id)
	case KindISBN:
		return r.resolveISBN(ctx, id)
	case KindURL:
		return r.resolveURL(ctx, id)
	}
	return nil, invalid(id, "not a DOI, arXiv identifier, ISBN or URL")
}

// fetchBibTeX asks one resolver for a BibTeX record and turns what comes back
// into a Result.
//
// DOI and arXiv differ only in how they name the record and where they serve it
// from, so the request, the status handling, the parse and the stamping live
// here once, and each resolver is left with the part that is its own.
func (r *Resolver) fetchBibTeX(ctx context.Context, id string, kind Kind, who, url string) (*Result, error) {
	resp, err := r.Client.Get(ctx, url, "application/x-bibtex")
	if err != nil {
		return nil, network(id, err)
	}
	if err := statusError(id, resp, who); err != nil {
		resp.Body.Close()
		return nil, err
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, network(id, err)
	}

	entry, err := firstEntry(id, who, body)
	if err != nil {
		return nil, err
	}
	now := r.now()
	stampAt(entry, now)
	return &Result{Entry: entry, Kind: kind, URL: url, Record: body, Retrieved: now}, nil
}

// now is the clock, defaulted so that a Resolver built by hand still works.
func (r *Resolver) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// stampAt records when a record was fetched. It is the one field resolution
// adds to every entry, because a pointer without a date cannot be checked for
// drift.
func stampAt(e *kb.BibEntry, t time.Time) {
	e.Set(kb.FieldRetrieved, bibValue(t.Format("2006-01-02")))
}

// maxBody bounds a response. A record and a page's metadata are both small, and
// a resolver that streams a gigabyte into memory is a resolver that can be made
// to fall over.
const maxBody = 2 << 20

// readBody reads a bounded response and closes it.
func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}

// firstEntry parses the BibTeX a resolver returned and takes its one entry.
//
// A resolver that returns BibTeX is the whole reason this path is preferred: a
// record that arrives already in the source-of-truth format has been produced
// by the authority that owns it, and reading it is a parse rather than a
// transformation.
func firstEntry(id, who string, body []byte) (*kb.BibEntry, error) {
	f, err := kb.ParseBibFile(id, body)
	if err != nil {
		return nil, network(id, fmt.Errorf("%s returned text that is not BibTeX: %w", who, err))
	}
	entries := f.Entries()
	if len(entries) == 0 {
		return nil, network(id, fmt.Errorf("%s returned no BibTeX entry", who))
	}
	return entries[0], nil
}

// bibValue renders text as a BibTeX field value: braced when it can be, and
// quoted when the text itself carries braces that a braced value could not hold
// safely.
func bibValue(s string) string {
	if strings.ContainsAny(s, "{}") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "{" + s + "}"
}

// setIf writes a field when there is something to write. An empty value is
// absent, not an empty field.
func setIf(e *kb.BibEntry, name, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	e.Set(name, bibValue(value))
}

// statusError turns a non-2xx response into the right failure: a missing record
// is not-found, anything else is the service failing to answer.
func statusError(id string, resp *http.Response, who string) error {
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return notFound(id, "%s has no record for it", who)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return network(id, fmt.Errorf("%s returned %s", who, resp.Status))
	}
	return nil
}

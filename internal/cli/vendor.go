package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/theorytoe/stemma/internal/extract"
	"github.com/theorytoe/stemma/internal/kb"
)

// vendorReport is the payload of `cite vendor --json`.
//
// Action is what actually happened, which is not always what was asked for: a
// capture already holding this text is left alone, and a record that lags behind a
// correct capture is brought up to date without the text being written again.
type vendorReport struct {
	Action    string   `json:"action"`
	Key       string   `json:"key"`
	Path      string   `json:"path"`
	Pointer   string   `json:"pointer"`
	Extractor string   `json:"extractor"`
	Hash      string   `json:"hash"`
	Bytes     int      `json:"bytes"`
	Notes     []string `json:"notes,omitempty"`
}

// citeVendorCommand implements `stemma cite vendor`.
//
// It is a member of the cite family because a capture records a hash on the entry,
// and it is built beside `fetch` because what it does is read a document. What it
// buys is the one thing a bibliography cannot do for itself: ten years from now
// the address will be gone and the capture will not be.
var citeVendorCommand = &command{
	name:    "vendor",
	summary: "capture a source's full text into the KB",
	args:    "<key>",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		force := fs.Bool("force", false, "replace a capture whose text has changed")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("cite vendor takes one key, got %d", len(args)))
			}
			k, code := o.load(w)
			if k == nil {
				return code
			}
			key := args[0]

			entry, ok := k.Bibliography.Entry(key)
			if !ok {
				return w.fail(fmt.Errorf("no entry for %q, so there is no source to capture", key))
			}
			pointer := entry.URL()
			if pointer == "" {
				return w.fail(fmt.Errorf("%q has no url, so there is nothing to read; "+
					"give the entry an address or a path to a file, then vendor it", key))
			}

			runner, err := newRunner(k.Root)
			if err != nil {
				return w.fail(err)
			}
			result, err := readPointer(runner, pointer)
			if err != nil {
				return w.fail(err)
			}
			// A capture that stops at a limit is not a capture. This is also the
			// point at which vendoring and fetching part company: `fetch` is happy to
			// hand back the first part of something enormous.
			if result.Truncated {
				return w.fail(fmt.Errorf("%q reads to more than the %d byte limit, and a capture has to be all of it",
					pointer, extract.DefaultLimit))
			}

			name := kb.VendoredName(key)
			full := kb.VendoredPath(k.Root, key)
			digest := kb.HashOf([]byte(result.Text))
			report := vendorReport{
				Key:       key,
				Path:      name,
				Pointer:   pointer,
				Extractor: result.Extractor,
				Hash:      digest,
				Bytes:     len(result.Text),
				Notes:     result.Notes,
			}

			have, readErr := os.ReadFile(full)
			present := readErr == nil
			if readErr != nil && !os.IsNotExist(readErr) {
				return w.fail(readErr)
			}
			onDisk := ""
			if present {
				onDisk = kb.HashOf(have)
			}
			recorded, claimed := entry.Value(kb.FieldVendored)

			needsText := !present || onDisk != digest
			needsRecord := !claimed || recorded != digest

			// What is in the KB is not what the source gives now. Replacing a
			// capture is a decision about evidence, so it is asked for rather than
			// assumed, and nothing is touched until it is.
			if needsText && present && !*force {
				return w.fail(fmt.Errorf(
					"%s already holds a capture of %q and this is a different text (%s on disk, %s read now), "+
						"so neither was touched; --force replaces it", name, key, onDisk, digest))
			}
			if !needsText && !needsRecord {
				report.Action = "unchanged"
				return emitVendor(w, report, fmt.Sprintf("%s is already captured in %s, and nothing has changed", key, name))
			}

			if needsText {
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					return w.fail(err)
				}
				if err := kb.WriteFileAtomic(full, []byte(result.Text)); err != nil {
					return w.fail(err)
				}
			}
			if needsRecord {
				if err := recordCapture(k, key, digest); err != nil {
					return w.fail(err)
				}
			}

			if needsText {
				report.Action = "captured"
				return emitVendor(w, report, fmt.Sprintf("captured %s in %s", key, name))
			}
			report.Action = "recorded"
			return emitVendor(w, report, fmt.Sprintf("%s: recorded the capture already in %s", key, name))
		}
	},
}

// recordCapture writes the hash of a capture onto its entry.
//
// The entry is re-read from the file that defines it, because the bibliography a
// command loads is a view across every file in the KB: setting a field on the view
// would write nothing anywhere, which is a quiet way to lose work.
func recordCapture(k *kb.KB, key, digest string) error {
	name := k.Bibliography.PathOf(key)
	file, err := k.ReadBibliography(name)
	if err != nil {
		return err
	}
	entry, ok := file.Entry(key)
	if !ok {
		return fmt.Errorf("%s defines no entry for %q", name, key)
	}
	entry.Set(kb.FieldVendored, "{"+digest+"}")
	return k.WriteBibliography(name, file)
}

// emitVendor reports what vendoring did, in whichever shape was asked for.
func emitVendor(w *output, report vendorReport, message string) int {
	if w.json {
		return w.emit(report)
	}
	fmt.Fprintln(w.stdout, message)
	return ExitOK
}

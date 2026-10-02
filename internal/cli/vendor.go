package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
			pointer := entry.Pointer()
			if pointer == "" {
				return w.fail(fmt.Errorf("%q has no url or path, so there is nothing to read; "+
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

			// The extracted text is what the entry records a hash of, so it is always
			// the capture. When the pointer is a file on this machine, the original is
			// written beside it, so a reader can see the PDF or the markdown rather
			// than only the text drawn out of it.
			text := []byte(result.Text)
			textName := kb.VendoredName(key)
			textFull := kb.VendoredPath(k.Root, key)

			var orig []byte
			origName, origFull := "", ""
			if info, statErr := os.Stat(pointer); statErr == nil && !info.IsDir() {
				orig, err = os.ReadFile(pointer)
				if err != nil {
					return w.fail(err)
				}
				if ext := strings.ToLower(filepath.Ext(pointer)); ext != "" && ext != ".txt" {
					origName = kb.OriginalName(key, ext)
					origFull = kb.OriginalPath(k.Root, key, ext)
				}
			}

			// The hash covers the capture a reader would open: the original when
			// there is one, and the extracted text otherwise.
			capture := text
			if origName != "" {
				capture = orig
			}
			digest := kb.HashOf(capture)

			type target struct {
				name string
				full string
				data []byte
			}
			targets := []target{{name: textName, full: textFull, data: text}}
			if origName != "" {
				targets = append(targets, target{name: origName, full: origFull, data: orig})
			}

			existing, err := kb.VendoredFiles(k.Root, key)
			if err != nil {
				return w.fail(err)
			}
			onDisk := map[string]string{}
			for _, f := range existing {
				raw, err := os.ReadFile(f.Path)
				if err != nil {
					return w.fail(err)
				}
				onDisk[f.Name] = kb.HashOf(raw)
			}

			report := vendorReport{
				Key:       key,
				Path:      textName,
				Pointer:   pointer,
				Extractor: result.Extractor,
				Hash:      digest,
				Bytes:     len(capture),
				Notes:     result.Notes,
			}

			// A target already there with the same bytes needs no write. Any other
			// difference is a decision about evidence, so it is asked for rather than
			// assumed, and nothing is touched until it is.
			var changed []target
			for _, t := range targets {
				sum := kb.HashOf(t.data)
				have, present := onDisk[t.name]
				if present && have == sum {
					continue
				}
				if present && !*force {
					return w.fail(fmt.Errorf(
						"%s already holds a capture of %q and this is a different text (%s on disk, %s read now), "+
							"so neither was touched; --force replaces it", t.name, key, have, sum))
				}
				changed = append(changed, t)
			}
			recorded, claimed := entry.Value(kb.FieldVendored)
			needsRecord := !claimed || recorded != digest

			if len(changed) == 0 && !needsRecord {
				report.Action = "unchanged"
				return emitVendor(w, report, fmt.Sprintf("%s is already captured in %s, and nothing has changed", key, textName))
			}

			for _, t := range changed {
				if err := os.MkdirAll(filepath.Dir(t.full), 0o755); err != nil {
					return w.fail(err)
				}
				if err := kb.WriteFileAtomic(t.full, t.data); err != nil {
					return w.fail(err)
				}
			}
			// A capture whose original changed extension leaves the old one behind, so
			// remove whatever is not part of this capture. It is the only file a command
			// is allowed to remove, and only because it wrote it.
			keep := map[string]bool{}
			for _, t := range targets {
				keep[t.name] = true
			}
			for _, f := range existing {
				if keep[f.Name] {
					continue
				}
				if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
					return w.fail(err)
				}
			}
			if needsRecord {
				if err := recordCapture(k, key, digest); err != nil {
					return w.fail(err)
				}
			}

			if len(changed) > 0 {
				report.Action = "captured"
				return emitVendor(w, report, fmt.Sprintf("captured %s in %s", key, textName))
			}
			report.Action = "recorded"
			return emitVendor(w, report, fmt.Sprintf("%s: recorded the capture already in %s", key, textName))
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

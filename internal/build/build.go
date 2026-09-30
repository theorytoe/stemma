// Package build writes a rendered KB to a directory as a static site.
//
// It is the offline entry point of the renderer, and it writes the same
// documents a server answers from, less the two things only a running server
// can provide: the search page, which is answered on demand, and the reload
// helper, which exists to talk to that server. Nothing here needs a server or a
// network. Every address the renderer emits is relative, so the directory can
// be moved, copied, or opened straight from disk and every link still resolves.
package build

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/render"
)

// manifestName is the file a build leaves behind to remember what it wrote.
//
// It is the whole of the builder's state across runs. A rebuild reads it,
// removes the files an earlier build wrote that are no longer part of the site,
// and writes it again. Those files are the only ones ever removed, so a file
// the author put in the output directory by hand survives every build (`P4`).
const manifestName = ".stemma-manifest"

// Result is what one build wrote.
type Result struct {
	// Dir is the output directory, as it was resolved by the caller.
	Dir string `json:"dir"`

	// Written is every file written, as a URL relative to Dir. It is the site:
	// Dir after this build holds exactly these files, plus whatever the author
	// put there.
	Written []string `json:"written"`

	// Removed is every file an earlier build wrote that this one did not. A
	// rebuild of an unchanged KB removes nothing.
	Removed []string `json:"removed,omitempty"`
}

// Write renders the KB at root into dir, creating it when it is not there.
//
// The directory is brought to the site rather than replaced by it. Files this
// build writes are written, files an earlier build wrote that are not part of
// the site now are removed, and everything else is left where it is. A rebuild
// therefore cannot leave a renamed page reachable at its old address, and
// cannot remove anything it did not write.
func Write(root, dir string) (*Result, error) {
	k, err := kb.Load(root)
	if err != nil {
		return nil, err
	}
	r, err := render.New(k)
	if err != nil {
		return nil, err
	}
	docs, err := r.Documents()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	res := &Result{Dir: dir, Written: make([]string, 0, len(docs))}
	written := make(map[string]bool, len(docs))
	for _, d := range docs {
		p, err := pathFor(dir, d.URL)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, d.Body, 0o644); err != nil {
			return nil, err
		}
		written[d.URL] = true
		res.Written = append(res.Written, d.URL)
	}

	removed, err := prune(dir, written)
	if err != nil {
		return nil, err
	}
	res.Removed = removed
	if err := writeManifest(dir, res.Written); err != nil {
		return nil, err
	}
	return res, nil
}

// pathFor is where a document's URL lives under dir. The URL is decoded first,
// so the file carries the name a server looks for once it has decoded the
// request; the escaped form would be a file no request could ever match.
func pathFor(dir, pageURL string) (string, error) {
	p, err := render.FilePath(pageURL)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.FromSlash(p)), nil
}

// prune removes the files an earlier build wrote that are not part of the site
// now, and returns the URLs it removed. A directory left empty is removed with
// them, so a page that moved does not leave its directory behind.
func prune(dir string, keep map[string]bool) ([]string, error) {
	old, err := readManifest(dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, u := range old {
		if keep[u] {
			continue
		}
		p, err := pathFor(dir, u)
		if err != nil {
			continue
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed = append(removed, u)
		removeEmptyDirs(dir, filepath.Dir(p))
	}
	return removed, nil
}

// removeEmptyDirs removes p and its parents for as long as they are empty,
// stopping at dir, which is never removed.
func removeEmptyDirs(dir, p string) {
	for p != dir && strings.HasPrefix(p, dir+string(filepath.Separator)) {
		if err := os.Remove(p); err != nil { // fails while not empty
			return
		}
		p = filepath.Dir(p)
	}
}

// readManifest returns the URLs an earlier build wrote. No manifest is not an
// error: it means the build to come is the first one, or the output directory
// is not one this tool made.
func readManifest(dir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var urls []string
	for _, line := range strings.Split(string(b), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			urls = append(urls, line)
		}
	}
	return urls, nil
}

// writeManifest records what this build wrote, so the next one knows what is
// its own to remove. One URL per line, which a URL cannot contain unmangled.
func writeManifest(dir string, urls []string) error {
	var b strings.Builder
	for _, u := range urls {
		b.WriteString(u)
		b.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join(dir, manifestName), []byte(b.String()), 0o644)
}

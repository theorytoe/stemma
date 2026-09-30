package kb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// KB is a knowledge base on disk: its pages, its bibliography, and the manifest
// that configures both.
type KB struct {
	// Root is the directory it was read from.
	Root string

	// Manifest is the KB's stemma.toml, or the defaults when it has none.
	Manifest Manifest

	// Vocabulary is the set of types the manifest accepts.
	Vocabulary Vocabulary

	// Graph holds the pages and the relations between them.
	Graph *Graph

	// Bibliography is every key the KB's BibTeX files define.
	Bibliography *Bibliography
}

// Load reads a knowledge base from a directory.
//
// It reads pages/ and the bibliography, and it does not read inbox/: a draft is
// not part of the knowledge, so nothing here reports on one. It also skips
// whatever the manifest's ignore list names.
//
// Load fails when a page or a bibliography file cannot be read at all — bytes
// that are not UTF-8, a frontmatter block that is never closed or is not a
// mapping, a bibliography entry that is never closed. In each of those the tool
// cannot tell what it would be rewriting, so it refuses rather than guessing.
// Every file it could not read is reported, not only the first, because a
// reader fixing a KB should see the whole list once.
func Load(root string) (*KB, error) {
	fsys := os.DirFS(root)
	name := filepath.Base(root)

	var m Manifest
	raw, err := fs.ReadFile(fsys, ManifestName)
	switch {
	case err == nil:
		m, err = ParseManifest(name, raw)
		if err != nil {
			return nil, err
		}
	case errors.Is(err, fs.ErrNotExist):
		m = DefaultManifest(name)
	default:
		return nil, fmt.Errorf("%s: %w", ManifestName, err)
	}

	hasPages, err := isDir(fsys, PagesDir)
	if err != nil {
		return nil, err
	}
	if !hasPages && len(m.raw) == 0 {
		return nil, fmt.Errorf("%s is not a KB root: it has neither %s nor %s",
			root, ManifestName, PagesDir)
	}

	k := &KB{
		Root:         root,
		Manifest:     m,
		Vocabulary:   m.Vocabulary(),
		Graph:        NewGraph(),
		Bibliography: NewBibliography(),
	}

	var unreadable []error

	if hasPages {
		paths, err := pageFiles(fsys, m)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", PagesDir, err)
		}
		for _, p := range paths {
			raw, err := fs.ReadFile(fsys, p)
			if err != nil {
				unreadable = append(unreadable, fmt.Errorf("%s: %w", p, err))
				continue
			}
			page, err := ParsePage(raw)
			if err != nil {
				unreadable = append(unreadable, fmt.Errorf("%s: %w", p, err))
				continue
			}
			k.Graph.Add(p, page)
		}
	}

	if err := k.loadBibliography(fsys, &unreadable); err != nil {
		return nil, err
	}
	if len(unreadable) > 0 {
		return nil, errors.Join(unreadable...)
	}
	return k, nil
}

// loadBibliography reads bibliography.bib, every .bib under bibliography/, or
// both, and merges them into one set of keys. Both forms being present is not
// an error: a key defined in each of them is a duplicate, which is reported
// like any other.
func (k *KB) loadBibliography(fsys fs.FS, unreadable *[]error) error {
	read := func(p string) {
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			*unreadable = append(*unreadable, fmt.Errorf("%s: %w", p, err))
			return
		}
		b, err := ParseBibliography(p, raw)
		if err != nil {
			*unreadable = append(*unreadable, fmt.Errorf("%s: %w", p, err))
			return
		}
		k.Bibliography.Add(b)
	}

	if _, err := fs.Stat(fsys, BibliographyName); err == nil {
		read(BibliographyName)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: %w", BibliographyName, err)
	}

	entries, err := fs.ReadDir(fsys, BibliographyDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", BibliographyDir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".bib") {
			continue
		}
		read(path.Join(BibliographyDir, e.Name()))
	}
	return nil
}

// Lint reports everything wrong with the KB under one mode.
//
// Three kinds of finding meet here: what a page gets wrong about itself, what
// the pages get wrong about each other, and what they get wrong about the
// bibliography. The result is sorted by path and line so that it reads in the
// order a person would fix it.
func (k *KB) Lint(mode Mode) []Finding {
	var out []Finding

	for _, p := range k.Graph.Paths() {
		page, ok := k.Graph.Page(p)
		if !ok {
			continue
		}
		for _, f := range page.Validate(k.Vocabulary, mode) {
			f.Path = p
			out = append(out, f)
		}
	}

	out = append(out, k.Graph.Findings(mode)...)
	out = append(out, k.Graph.CitationFindings(k.Bibliography, mode)...)

	sortFindings(out)
	return out
}

// FindingsFor returns the findings that belong to one page: the links on it
// that do not resolve to exactly one page, and the citations on it that the
// bibliography does not define.
//
// The text and the severities are the ones Lint reports, read from the same
// helpers, but only the one page is examined. A renderer asks for this rather
// than for Lint so that rendering a whole KB stays proportional to the KB
// instead of quadratic in it.
func (k *KB) FindingsFor(path string, mode Mode) []Finding {
	soft := softSeverity(mode)
	out := k.Graph.linkFindings(path, soft)
	out = append(out, k.Graph.missingCitationFindings(path, k.Bibliography, soft)...)
	sortFindings(out)
	return out
}

// sortFindings puts findings in the order a reader would work through them.
func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Path != fs[j].Path {
			return fs[i].Path < fs[j].Path
		}
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Code < fs[j].Code
	})
}

// pageFiles returns the KB-relative path of every page, in lexical order,
// honouring the manifest's ignore list. It reads no file's contents.
//
// Load and Hashes both go through it, so the pages a KB contains and the pages
// an index stamps cannot drift apart: a walk that disagreed would make a fresh
// index look stale, or worse, hide a page from a rebuild.
func pageFiles(fsys fs.FS, m Manifest) ([]string, error) {
	hasPages, err := isDir(fsys, PagesDir)
	if err != nil {
		return nil, err
	}
	if !hasPages {
		return nil, nil
	}
	var out []string
	err = fs.WalkDir(fsys, PagesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if m.Ignores(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Hashes returns the content digest of every page in a KB, keyed by path.
//
// It reads the manifest to honour the ignore list and reads each page's bytes,
// but parses none of them. An index stamps pages by this digest, and checking
// the stamps means reading the same bytes; a caller deciding whether a cache is
// still true does not need the page model to decide it.
func Hashes(root string) (map[string]string, error) {
	fsys := os.DirFS(root)
	name := filepath.Base(root)

	var m Manifest
	raw, err := fs.ReadFile(fsys, ManifestName)
	switch {
	case err == nil:
		m, err = ParseManifest(name, raw)
		if err != nil {
			return nil, err
		}
	case errors.Is(err, fs.ErrNotExist):
		m = DefaultManifest(name)
	default:
		return nil, fmt.Errorf("%s: %w", ManifestName, err)
	}

	paths, err := pageFiles(fsys, m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", PagesDir, err)
	}
	out := make(map[string]string, len(paths))
	for _, p := range paths {
		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out[p] = HashOf(raw)
	}
	return out, nil
}

// isDir reports whether a path inside a KB is a directory, and whether the
// answer could be worked out at all.
func isDir(fsys fs.FS, name string) (bool, error) {
	info, err := fs.Stat(fsys, name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return info.IsDir(), nil
}

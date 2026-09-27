package kb

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// Path returns where a KB-relative name lives on disk.
func (k *KB) Path(name string) string {
	return filepath.Join(k.Root, filepath.FromSlash(name))
}

// PagePath is where a page titled title is written when the author does not
// choose a filename. The filename is the title's slug, which is what makes it
// predictable without anyone having to decide it.
func PagePath(dir, title string) string {
	return path.Join(dir, Normalize(title)+".md")
}

// WritePage writes a page to a KB-relative path, replacing whatever was there.
//
// The bytes go to a temporary file beside the target and are renamed into
// place, so a failure part way through leaves the old page exactly as it was
// rather than half of a new one. A tool whose central promise is that it never
// destroys what it does not understand cannot afford a write that can truncate.
func (k *KB) WritePage(name string, p *Page) error {
	return writeFileAtomic(k.Path(name), p.Bytes())
}

// CreatePage writes a new page, refusing to replace anything. A page is
// identified by its title, so replacing one silently would be losing a page
// whose name the author still expects to mean something.
func (k *KB) CreatePage(name string, p *Page) error {
	full := k.Path(name)
	if err := mustNotExist(full, name); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(full, p.Bytes())
}

// MovePage moves a page to another KB-relative path.
//
// Directories inside pages/ carry no meaning, so a move changes no link and
// nothing has to be rewritten. The filename does not change either: it is not
// what a page is.
func (k *KB) MovePage(from, to string) error {
	src, dst := k.Path(from), k.Path(to)
	if err := mustNotExist(dst, to); err != nil {
		return err
	}
	if _, err := os.Lstat(src); err != nil {
		return fmt.Errorf("%s: %w", from, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// mustNotExist fails when something is already at a path.
func mustNotExist(full, name string) error {
	_, err := os.Lstat(full)
	switch {
	case err == nil:
		return fmt.Errorf("%s already exists", name)
	case os.IsNotExist(err):
		return nil
	default:
		return err
	}
}

// writeFileAtomic writes a file by way of a temporary neighbour, so that no
// reader ever sees a partial page and no failed write costs a page.
func writeFileAtomic(full string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(full); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(full), "."+filepath.Base(full)+".tmp*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // a no-op once the rename below has happened

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, full)
}

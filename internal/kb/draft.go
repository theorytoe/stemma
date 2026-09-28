package kb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

// DraftPath is where a draft titled title lives: the inbox, named by the
// title's slug.
func DraftPath(title string) string {
	return path.Join(InboxDir, Normalize(title)+".md")
}

// DraftPaths returns every draft in the KB as KB-relative paths, sorted.
//
// A draft is a .md file under inbox/. Drafts sit outside the knowledge proper,
// so Load never reads them and nothing reports on one until a command asks
// about the inbox specifically. A path the manifest ignores is not a draft,
// for the same reason it is not anything else: it is outside the KB.
func (k *KB) DraftPaths() ([]string, error) {
	fsys := os.DirFS(k.Root)
	var out []string
	err := fs.WalkDir(fsys, InboxDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if k.Manifest.Ignores(p) {
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
		return nil, fmt.Errorf("%s: %w", InboxDir, err)
	}
	sort.Strings(out)
	return out, nil
}

// ReadDraft reads one draft by its KB-relative path.
//
// An inbox file is markdown but is not a page: the format's rules do not apply
// to it, so this is the same parse a page gets and nothing more. A draft whose
// frontmatter cannot be read at all still fails, because promotion could not
// know what it would be promoting.
func (k *KB) ReadDraft(name string) (*Page, error) {
	raw, err := os.ReadFile(k.Path(name))
	if err != nil {
		return nil, err
	}
	page, err := ParsePage(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return page, nil
}

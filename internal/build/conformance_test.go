package build

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/render"
)

// The gate reads the built output the way a reader would: it finds the addresses
// a document carries and the lists it shows, and asks whether any of it needs a
// script. A full HTML parser is not in the pinned allowlist, and the renderer
// writes its own attributes with double quotes, so scanning for the attribute
// names and the section markers answers the question without one.
var (
	addressAttr = regexp.MustCompile(`(?:href|src)="([^"]*)"`)
	scriptTag   = regexp.MustCompile(`(?s)<script\b([^>]*)>`)
	item        = regexp.MustCompile(`(?s)<li>(.*?)</li>`)
	tag         = regexp.MustCompile(`(?s)<[^>]*>`)

	backlinks  = regexp.MustCompile(`(?s)<nav class="backlinks">(.*?)</nav>`)
	references = regexp.MustCompile(`(?s)<section class="references">(.*?)</section>`)
)

// navTargets are the five addresses every document must reach. The entry
// document is among them: it is reachable from anywhere in the site, not only
// from the root.
var navTargets = []string{"index.html", "all.html", "types.html", "tags.html", "graph.html"}

// TestNoJSConformance is the P9 gate over a built site.
//
// It asserts on the output rather than on the templates, because the output is
// what a reader is handed: every page, index, navigation and list is there as
// HTML, and every address in it names a file that exists. A surface that only
// worked with JavaScript would fail one of these, which is what keeps the
// requirement from eroding on the first convenient exception.
func TestNoJSConformance(t *testing.T) {
	site := filepath.Join(t.TempDir(), "site")
	if _, err := Write(exampleWiki(t), site); err != nil {
		t.Fatal(err)
	}
	docs := readSite(t, site)

	withBacklinks, withReferences := 0, 0
	for _, doc := range sortedKeys(docs) {
		body := docs[doc]

		// Navigation and reachability: every address in the document names a
		// file the build wrote, and the five of the navigation are among them.
		reached := map[string]bool{}
		for _, href := range addresses(body) {
			target, internal, err := internalTarget(doc, href)
			if err != nil {
				t.Errorf("%s carries an address it cannot be read as: %s (%v)", doc, href, err)
				continue
			}
			if !internal {
				continue
			}
			if !writtenAt(site, target) {
				t.Errorf("%s links to %s, which the build did not write", doc, href)
			}
			reached[target] = true
		}
		for _, want := range navTargets {
			if !reached[want] {
				t.Errorf("%s cannot reach %s without a script", doc, want)
			}
		}

		// Readable means a heading, and no marker standing in for something a
		// script was meant to produce. A marked link is a link that did not
		// resolve, which from a reader's side is a page that is missing.
		if !strings.Contains(body, "<h1>") {
			t.Errorf("%s has no heading", doc)
		}
		for _, dead := range []string{"javascript:", " onclick=", `class="stemma-unresolved"`, `class="stemma-ambiguous"`} {
			if strings.Contains(body, dead) {
				t.Errorf("%s carries %q", doc, dead)
			}
		}

		// Every script is optional: it is a static asset, or it is data rather
		// than code. Nothing a reader needs is computed after the page arrives.
		for _, m := range scriptTag.FindAllStringSubmatch(body, -1) {
			attrs := m[1]
			if !strings.Contains(attrs, "src=") && !strings.Contains(attrs, `type="application/json"`) {
				t.Errorf("%s carries a script that is not optional: <script%s>", doc, attrs)
			}
		}

		// The one control that needs a script is hidden until a script shows it,
		// so a reader without JavaScript sees no dead control.
		if !strings.Contains(body, "data-theme-toggle hidden") {
			t.Errorf("%s shows the theme toggle without a script to drive it", doc)
		}

		// The two lists this task names, each holding real links or real text.
		if s := backlinks.FindStringSubmatch(body); s != nil {
			withBacklinks++
			if !strings.Contains(s[1], `href="`) {
				t.Errorf("%s has a backlinks section with no links in it", doc)
			}
		}
		if s := references.FindStringSubmatch(body); s != nil {
			withReferences++
			entries := item.FindAllStringSubmatch(s[1], -1)
			if len(entries) == 0 {
				t.Errorf("%s has a references section with no entries", doc)
			}
			for _, e := range entries {
				if strings.TrimSpace(tag.ReplaceAllString(e[1], "")) == "" {
					t.Errorf("%s has an empty reference entry", doc)
				}
			}
		}
	}

	// A gate over a wiki with no backlinks or no citations would pass while
	// checking nothing, so the fixture has to exercise both.
	if withBacklinks == 0 {
		t.Error("the example wiki produced no backlink list")
	}
	if withReferences == 0 {
		t.Error("the example wiki produced no reference list")
	}

	// The per-type and per-tag indexes are surfaces of their own: each exists,
	// is reachable from its listing page, and lists pages.
	indexFamily(t, site, docs, "types.html", "types/")
	indexFamily(t, site, docs, "tags.html", "tags/")
}

// indexFamily checks one family of index pages: the listing reaches every index
// under prefix, and every one of them lists pages.
func indexFamily(t *testing.T, site string, docs map[string]string, listing, prefix string) {
	t.Helper()
	reached := map[string]bool{}
	for _, href := range addresses(docs[listing]) {
		if target, internal, err := internalTarget(listing, href); err == nil && internal {
			reached[target] = true
		}
	}

	found := 0
	for _, doc := range sortedKeys(docs) {
		if !strings.HasPrefix(doc, prefix) {
			continue
		}
		found++
		if !reached[doc] {
			t.Errorf("%s does not reach %s", listing, doc)
		}
		if !strings.Contains(docs[doc], `class="page-list"`) {
			t.Errorf("%s lists no pages", doc)
		}
	}
	if found == 0 {
		t.Errorf("the example wiki produced no index under %s", prefix)
	}
}

// internalTarget reads an address in a document as a path inside the site. It
// reports false for an address that is not the site's business -- a fragment, or
// a link to another host -- and reports an error for one that climbs above the
// output directory, since a built site has nothing outside it to reach.
func internalTarget(doc, href string) (target string, internal bool, err error) {
	if href == "" || strings.HasPrefix(href, "#") {
		return "", false, nil
	}
	u, err := url.Parse(href)
	if err != nil {
		return "", false, err
	}
	if u.Scheme != "" || u.Host != "" || u.Path == "" {
		return "", false, nil
	}
	// The escaped path, because that is the form a URL is written in and the
	// form the file is decoded from.
	target = path.Join(path.Dir(doc), u.EscapedPath())
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", false, fmt.Errorf("it climbs out of the site")
	}
	return target, true, nil
}

// addresses returns every address a document carries, as written.
func addresses(body string) []string {
	ms := addressAttr.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}

// writtenAt reports whether the build wrote the file an address names. An
// address is a URL and a file name is not, so it is decoded on the way, which is
// the same rule the writer and the server read from render.FilePath.
func writtenAt(site, target string) bool {
	decoded, err := render.FilePath(target)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(site, filepath.FromSlash(decoded)))
	return err == nil
}

// readSite reads every HTML document a build wrote, keyed by its path relative
// to the site root.
func readSite(t *testing.T, site string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(site, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		rel, err := filepath.Rel(site, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("the build wrote no HTML")
	}
	return out
}

// exampleWiki copies the project's example wiki into a temporary directory, so
// the gate runs over the real documentation and the repository is left alone.
// The generated .stemma/ directory is skipped: it is a cache, not corpus.
func exampleWiki(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "wiki")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("the example wiki is not present: %v", err)
	}
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if rel == ".stemma" || strings.HasPrefix(rel, ".stemma"+string(filepath.Separator)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

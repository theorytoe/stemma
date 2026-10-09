package cli

import (
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// PromoteReport is what --json says about a promotion.
type PromoteReport struct {
	From          string `json:"from"`
	To            string `json:"to"`
	Title         string `json:"title"`
	Type          string `json:"type"`
	LinksResolved int    `json:"links_resolved"`
}

// promoteCommand implements `stemma promote`.
//
// Promotion is the single inbox transition. A draft is outside the knowledge
// proper and the format's rules do not apply to it, so everything the inbox
// permitted becomes an error at once: the draft must have a title, it must have
// a type (from itself or --type), and it must satisfy every page rule. A draft
// that cannot become a valid page is refused rather than moved and left broken.
//
// A page is identified by its title, so a promotion resolves the links that
// already pointed at the draft without rewriting anything: moving the file into
// pages/ is what makes them resolve. The count is reported so the author can
// see the effect.
var promoteCommand = &command{
	name:    "promote",
	summary: "move a draft from the inbox into the pages",
	args:    "DRAFT",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		typ := fs.String("type", "", "the type to give the page; the draft's own when it has one")
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 1 {
				return w.fail(fmt.Errorf("promote takes one draft"))
			}

			k, code := o.load(w)
			if k == nil {
				return code
			}
			report, findings, err := Promote(k, args[0], *typ)
			if err != nil {
				return w.fail(err)
			}
			if w.json {
				if len(findings) > 0 {
					return w.report(report, findings)
				}
				return w.emit(report)
			}
			if len(findings) > 0 {
				return reportText(w.stdout, w.stderr, findings)
			}
			fmt.Fprintf(w.stdout, "%s -> %s\n", report.From, report.To)
			fmt.Fprintf(w.stdout, "promoted %q as %s; %s now resolve\n",
				report.Title, report.Type, count(report.LinksResolved, "link"))
			return ExitOK
		}
	},
}

// Promote moves a draft from the inbox into the pages and returns its
// report. A page is identified by its title, so the links that already point
// at the draft resolve the moment the file moves; the report counts them. The
// draft is held to the page format first — everything the inbox permitted is
// an error now — and when validation fails the findings travel back with the
// report and nothing is written. pageType, when given, replaces the draft's
// own.
func Promote(k *kb.KB, name, pageType string) (PromoteReport, []kb.Finding, error) {
	from, page, err := ResolveDraft(k, name)
	if err != nil {
		return PromoteReport{}, nil, err
	}

	title := page.Title()
	if kb.Normalize(title) == "" {
		return PromoteReport{}, nil, fmt.Errorf("%s has no usable title, so nothing could link to the page it became", from)
	}

	if pageType != "" {
		// A reserved type is the tool's own and is never assigned, so
		// choosing one is a usage error. An unknown type is not: it is
		// caught by validation below, as a finding about the draft (D55).
		if kb.IsReservedType(pageType) {
			return PromoteReport{}, nil, fmt.Errorf("%q is a type the tool owns and does not assign", pageType)
		}
		if err := page.Set(kb.FieldType, pageType); err != nil {
			return PromoteReport{}, nil, err
		}
	}
	// The type the page will carry, which may be the draft's own. A missing
	// or unknown one is caught by validation below, because everything the
	// inbox permitted is an error now.
	pageType = page.Type()

	// Everything the inbox permitted is an error now.
	if findings := page.Validate(k.Vocabulary, kb.Strict); len(findings) > 0 {
		return PromoteReport{From: from, Title: title, Type: pageType}, findings, nil
	}

	// A page may not take a name another page answers to: every link
	// to that name would become ambiguous, which is a hard error.
	if claimants := k.Graph.Claimants(title); len(claimants) > 0 {
		return PromoteReport{}, nil, fmt.Errorf("%q is already the name of %s", title, strings.Join(claimants, " and "))
	}

	to := kb.PagePath(kb.PagesDir, title)
	resolved := LinksThatWillResolve(k, page)

	if err := k.CreatePage(to, page); err != nil {
		return PromoteReport{}, nil, err
	}
	if err := os.Remove(k.Path(from)); err != nil {
		return PromoteReport{}, nil, fmt.Errorf("%s is now %s, but the draft could not be removed: %w", from, to, err)
	}

	return PromoteReport{
		From:          from,
		To:            to,
		Title:         title,
		Type:          pageType,
		LinksResolved: resolved,
	}, nil, nil
}

// LinksThatWillResolve counts the links in the pages that point at a name the
// draft answers to and do not resolve yet. They resolve the moment the draft
// becomes a page, because resolution is by title and the title moves with it.
func LinksThatWillResolve(k *kb.KB, draft *kb.Page) int {
	n := 0
	for _, p := range k.Graph.Paths() {
		for _, l := range k.Graph.Links(p) {
			if k.Graph.Resolve(l.Name).Kind != kb.Unresolved {
				continue
			}
			if draft.AnswersTo(l.Target) {
				n++
			}
		}
	}
	return n
}

// ResolveDraft finds a draft by path, by filename, or by the title or alias it
// answers to.
//
// A draft with no readable frontmatter cannot be one of the matches, but it
// must not stop a different draft being found, so it is held back and named
// only if nothing matched.
func ResolveDraft(k *kb.KB, name string) (string, *kb.Page, error) {
	paths, err := k.DraftPaths()
	if err != nil {
		return "", nil, err
	}

	read := func(p string) (*kb.Page, error) { return k.ReadDraft(p) }

	for _, p := range paths {
		if p == name || path.Base(p) == name {
			page, err := read(p)
			if err != nil {
				return "", nil, err
			}
			return p, page, nil
		}
	}
	want := kb.Normalize(name) + ".md"
	for _, p := range paths {
		if path.Base(p) == want {
			page, err := read(p)
			if err != nil {
				return "", nil, err
			}
			return p, page, nil
		}
	}

	var matches, unreadable []string
	for _, p := range paths {
		page, err := read(p)
		if err != nil {
			unreadable = append(unreadable, p)
			continue
		}
		if page.AnswersTo(name) {
			matches = append(matches, p)
		}
	}
	sort.Strings(matches)
	switch len(matches) {
	case 0:
		if len(unreadable) > 0 {
			sort.Strings(unreadable)
			return "", nil, fmt.Errorf("no draft is called %q; %s could not be read",
				name, strings.Join(unreadable, ", "))
		}
		return "", nil, fmt.Errorf("no draft is called %q", name)
	case 1:
		page, err := read(matches[0])
		if err != nil {
			return "", nil, err
		}
		return matches[0], page, nil
	default:
		return "", nil, fmt.Errorf("%q could be %s", name, strings.Join(matches, " or "))
	}
}

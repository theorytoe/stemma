package cli

import (
	"flag"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/theorytoe/stemma/internal/kb"
)

// renameReport is what --json says about a rename.
type renameReport struct {
	From      string `json:"from"`
	To        string `json:"to"`
	OldTitle  string `json:"old_title"`
	NewTitle  string `json:"new_title"`
	Rewritten int    `json:"links_rewritten"`
	Files     int    `json:"files_rewritten"`
	LeftAlone int    `json:"links_left_alone"`
}

// renameCommand implements `stemma rename`.
//
// A page is identified by its title, so retitling one breaks every link that
// named it. This command exists to answer whether that can be done reliably,
// and it is the reason the foundation was ordered the way it was.
//
// The steps are ordered so that an interrupted run can be completed by running
// the same command again:
//
//  1. move the file to the name the new title slugs to, if that name is free;
//  2. rewrite every link that currently resolves to this page by its title;
//  3. retitle the page;
//  4. read the KB back and check that nothing that resolved before fails now.
//
// Links by alias are left alone. They name the page correctly and they still
// resolve, and rewriting them would be editing prose that is not broken. Links
// whose target is ambiguous are left alone too, because such a link did not
// mean this page and must not be silently retargeted to it.
var renameCommand = &command{
	name:    "rename",
	summary: "retitle a page and rewrite every link that named it",
	args:    "PAGE TITLE",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) != 2 {
				return w.fail(fmt.Errorf("rename takes a page and a new title"))
			}
			from, newTitle := args[0], strings.TrimSpace(args[1])

			k, code := o.load(w)
			if k == nil {
				return code
			}
			start, err := resolve(k, from)
			if err != nil {
				return w.fail(err)
			}
			page, ok := k.Graph.Page(start)
			if !ok {
				return w.fail(fmt.Errorf("%s is not a page", start))
			}
			oldTitle := page.Title()

			destination := path.Join(path.Dir(start), kb.Normalize(newTitle)+".md")
			if err := checkNewTitle(k, start, newTitle); err != nil {
				return w.fail(err)
			}

			// Anything that resolved before must still resolve afterwards.
			// Collecting this first is what makes the check at the end a
			// comparison rather than a guess.
			brokenBefore := unresolvedLinks(k, start, destination)

			// 1. Move the file, unless the name it wants is already taken by
			// another page, in which case the file keeps its name: nothing
			// depends on it.
			moved := false
			if destination != start {
				if err := k.MovePage(start, destination); err != nil {
					if _, taken := k.Graph.Page(destination); taken {
						destination = start
					} else {
						return w.fail(err)
					}
				} else {
					moved = true
				}
			}

			// 2. Rewrite the links that resolve here by title.
			report := renameReport{From: start, To: destination, OldTitle: oldTitle, NewTitle: newTitle}
			var leftAlone []string
			for _, referrer := range k.Graph.Paths() {
				referrerPage, ok := k.Graph.Page(referrer)
				if !ok {
					continue
				}
				rewritten := 0
				changed := referrerPage.RewriteLinks(func(l kb.Link) (string, bool) {
					if l.Name != kb.Normalize(oldTitle) {
						// Not a link by the old title. A link by an alias still
						// names this page and is left as the author wrote it.
						return "", false
					}
					switch k.Graph.Resolve(l.Name).Kind {
					case kb.Resolved:
						rewritten++
						return newTitle, true
					case kb.Ambiguous:
						// The link did not mean this page, so it must not be
						// quietly made to.
						leftAlone = append(leftAlone, fmt.Sprintf("%s:%d", referrer, l.Line))
					}
					return "", false
				})
				if !changed {
					continue
				}
				// The page may have been moved already, so its own body is
				// written to where it now is rather than to where it was.
				where := referrer
				if referrer == start {
					where = destination
				}
				if err := k.WritePage(where, referrerPage); err != nil {
					return unfinished(w, err, report, moved)
				}
				report.Files++
				report.Rewritten += rewritten
			}

			// 3. Retitle the page.
			if err := page.Set(kb.FieldTitle, newTitle); err != nil {
				return unfinished(w, err, report, moved)
			}
			if err := k.WritePage(destination, page); err != nil {
				return unfinished(w, err, report, moved)
			}

			// 4. Resolve again and check.
			after, err := kb.Load(k.Root)
			if err != nil {
				return w.fail(fmt.Errorf("the rename may be incomplete: %w", err))
			}
			if r := after.Graph.Resolve(newTitle); r.Kind != kb.Resolved || r.Path != destination {
				return w.fail(fmt.Errorf("%q does not resolve to %s now that it is renamed", newTitle, destination))
			}
			if broke := difference(unresolvedLinks(after, start, destination), brokenBefore); len(broke) > 0 {
				return w.fail(fmt.Errorf("renaming broke %s", strings.Join(broke, ", ")))
			}

			report.LeftAlone = len(leftAlone)
			if w.json {
				return w.emit(report)
			}
			if moved {
				fmt.Fprintf(w.stdout, "%s -> %s\n", start, destination)
			}
			fmt.Fprintf(w.stdout, "renamed %q to %q, rewriting %s in %s\n",
				oldTitle, newTitle, count(report.Rewritten, "link"), count(report.Files, "file"))
			for _, where := range leftAlone {
				fmt.Fprintf(w.stderr, "stemma: left the ambiguous link at %s alone; it did not name this page\n", where)
			}
			return ExitOK
		}
	},
}

// checkNewTitle refuses a title that would break the KB rather than make it.
func checkNewTitle(k *kb.KB, start, newTitle string) error {
	if kb.Normalize(newTitle) == "" {
		return fmt.Errorf("%q has no letters or digits in it, so nothing could link to it", newTitle)
	}
	for _, claimant := range k.Graph.Claimants(newTitle) {
		if claimant != start {
			return fmt.Errorf("%q is already the name of %s", newTitle, claimant)
		}
	}
	return nil
}

// unresolvedLinks is the set of links that resolve to nothing, as a set so that
// two runs can be compared. The page and line are enough to identify one: a
// rewrite never changes how many lines a file has.
//
// The page is named by where it was before the rename for the one page whose
// path the rename changes, so that a link it already had dangling compares
// equal to itself. Without that, the moved page's own unresolved links look
// like new ones wherever the rename is reported.
func unresolvedLinks(k *kb.KB, renamedFrom, renamedTo string) map[string]bool {
	out := map[string]bool{}
	for _, f := range k.Graph.Findings(kb.Lenient) {
		if f.Code != kb.CodeUnresolvedLink {
			continue
		}
		at := f.Path
		if at == renamedTo {
			at = renamedFrom
		}
		out[fmt.Sprintf("%s:%d", at, f.Line)] = true
	}
	return out
}

func difference(after, before map[string]bool) []string {
	var out []string
	for key := range after {
		if !before[key] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// unfinished says what did get done, because a caller who is told only that
// something failed cannot tell how much of it failed.
func unfinished(w *output, err error, report renameReport, moved bool) int {
	fmt.Fprintf(w.stderr, "stemma: %v\n", err)
	fmt.Fprintf(w.stderr, "stemma: %s rewritten in %s; run the same command again to finish\n",
		count(report.Rewritten, "link"), count(report.Files, "file"))
	if moved {
		fmt.Fprintf(w.stderr, "stemma: the file is already at %s\n", report.To)
	}
	return ExitError
}

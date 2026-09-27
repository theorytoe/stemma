// Package kb owns the knowledge base itself: the manifest, the page model and
// frontmatter handling, link and citation resolution, and the invariants lint
// enforces.
//
// Everything else is a surface over this package. The CLI (cmd/stemma) and the
// MCP server are thin, and neither holds format knowledge of its own (D10).
//
// The package is empty at the moment. Its tasks, in order: the page model
// (foundation Task 3), the wikilink resolver (Task 4), and citation-key
// resolution (Task 5).
package kb

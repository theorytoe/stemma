// Package kb owns the knowledge base itself: the manifest, the page model and
// frontmatter handling, link and citation resolution, and the invariants lint
// enforces.
//
// Everything else is a surface over this package. The CLI (cmd/stemma) and the
// MCP server are thin, and neither holds format knowledge of its own (D10).
//
// The central promise is preservation. A page the tool has no reason to change
// comes back byte for byte, including key order, comments and quoting style, and
// a page it cannot read is refused rather than guessed at. Pages are read through
// yaml.v3 but written by splicing lines into the original block, because
// re-encoding the block could not make that promise.
package kb

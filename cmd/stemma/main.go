// Command stemma is the universal surface for a knowledge base: the one
// interface that every other surface falls back to (D8, D56).
//
// The command tree is not built yet. This is the scaffolding from Task 1 of
// delegates/foundation.md; the core verbs arrive with Task 7 of that delegate
// and the rest of the surface with delegates/cli-surface.md.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "stemma: not implemented yet")
	os.Exit(2)
}

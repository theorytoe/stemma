// Command stemma is the universal surface for a knowledge base: the one
// interface that every other surface falls back to.
//
// The command tree is still growing. This is the plumbing from the foundation
// delegate; the core verbs arrive next and the rest of the surface after that.
package main

import (
	"os"

	"github.com/theorytoe/stemma/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

// Command stemma is the universal surface for a knowledge base: the one
// interface that every other surface falls back to.
//
// The command table, the help text and the generated reference all live in
// internal/cli. This is the entry point and nothing else.
package main

import (
	"os"

	"github.com/theorytoe/stemma/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

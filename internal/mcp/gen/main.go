// Command gen writes the schema set and the tool reference the MCP
// documentation is rendered from: the JSON document a program reads, and the
// markdown reference a person reads, both projections of the registry in
// internal/mcp. The Makefile's mcp-docs target runs this, and the freshness
// test in internal/mcp fails when the files on disk no longer match what the
// registry renders, so a schema and its documentation cannot drift.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/theorytoe/stemma/internal/mcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	jsonOut := flag.String("json", "", "write the schema set here")
	referenceOut := flag.String("reference", "", "write the tool reference here")
	flag.Parse()
	if *jsonOut == "" || *referenceOut == "" {
		return fmt.Errorf("give both -json and -reference")
	}

	schema, err := mcp.JSONBytes()
	if err != nil {
		return err
	}
	if err := os.WriteFile(*jsonOut, schema, 0o644); err != nil {
		return err
	}
	reference, err := mcp.ReferenceBytes()
	if err != nil {
		return err
	}
	return os.WriteFile(*referenceOut, reference, 0o644)
}

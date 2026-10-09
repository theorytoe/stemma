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
	"path/filepath"

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
	manOut := flag.String("man", "", "write the server's manual page here")
	flag.Parse()
	if *jsonOut == "" && *referenceOut == "" && *manOut == "" {
		return fmt.Errorf("give -json, -reference or -man")
	}

	// Each artifact is written only when its path is given, so a target that
	// needs one page does not regenerate the rest.
	if *jsonOut != "" || *referenceOut != "" {
		schema, err := mcp.JSONBytes()
		if err != nil {
			return err
		}
		reference, err := mcp.ReferenceBytes()
		if err != nil {
			return err
		}
		// The writer owns its outputs: docs/ is generated and never committed
		// (P8), so a fresh checkout does not have the directory to write into.
		for _, out := range []struct {
			path string
			data []byte
		}{
			{*jsonOut, schema},
			{*referenceOut, reference},
		} {
			if out.path == "" {
				continue
			}
			if dir := filepath.Dir(out.path); dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
			}
			if err := os.WriteFile(out.path, out.data, 0o644); err != nil {
				return err
			}
		}
	}
	if *manOut == "" {
		return nil
	}
	man, err := mcp.ManBytes()
	if err != nil {
		return err
	}
	if dir := filepath.Dir(*manOut); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(*manOut, man, 0o644)
}

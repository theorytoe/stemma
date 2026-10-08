// Command gen writes the curated MCP tool set as one JSON document: each tool
// with its input schema and the shape of its result, the envelope every result
// uses, the outcomes an error maps to, and the commands deliberately left on
// the CLI with their reasons. The Makefile's mcp-schema target regenerates the
// artifact, and the documentation renders the tool reference from it, so a
// schema and its description cannot drift.
package main

import (
	"encoding/json"
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
	out := flag.String("out", "", "write the artifact here instead of standard output")
	flag.Parse()

	tools := make([]map[string]any, 0, len(mcp.Tools))
	for _, t := range mcp.Tools {
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"command":     t.Command,
			"description": t.Description,
			"input":       t.Input,
			"result":      mcp.Envelope(mcp.Shape(t.Payload)),
		})
	}

	omitted := make([]map[string]string, 0, len(mcp.Omissions))
	for _, o := range mcp.Omissions {
		omitted = append(omitted, map[string]string{"command": o.Command, "reason": o.Reason})
	}

	doc := map[string]any{
		"contract": "every result is the CLI verb's --json envelope, carrying the CLI's own payload types (D57)",
		"envelope": mcp.Envelope(map[string]any{}),
		"errors": []map[string]string{
			{"when": "the tool ran clean", "result": "ok true with data; the twin of exit code 0"},
			{"when": "the KB has findings", "result": "ok false with findings; the twin of exit code 1, returned as a result rather than a protocol error"},
			{"when": "the run could not complete", "result": "ok false with error; the twin of exit code 2, returned as a result rather than a protocol error"},
			{"when": "the arguments do not match the input schema", "result": "a JSON-RPC invalid-params error, refused before the tool runs"},
		},
		"tools":   tools,
		"omitted": omitted,
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	if *out == "" {
		_, err = os.Stdout.Write(b)
		return err
	}
	if dir := filepath.Dir(*out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(*out, b, 0o644)
}

package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/theorytoe/stemma/internal/version"
)

// This file assembles the schema set and renders its two artifacts: the JSON
// document a program reads and the markdown reference a person reads. Both
// are written by `make mcp-docs` through internal/mcp/gen, and neither is
// committed (P8); the freshness test in this package renders them again and
// fails when what is on disk no longer matches the registry.

// BuildDoc assembles the schema set: the envelope every result uses, the
// outcomes an error maps to, each tool with its input schema and result
// shape, and the commands deliberately left on the CLI. It is the whole truth
// about the surface, and both artifacts are projections of it.
func BuildDoc() map[string]any {
	tools := make([]map[string]any, 0, len(Tools))
	for _, t := range Tools {
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"command":     t.Command,
			"description": t.Description,
			"input":       t.Input,
			"result":      Envelope(Shape(t.Payload)),
		})
	}

	omitted := make([]map[string]string, 0, len(Omissions))
	for _, o := range Omissions {
		omitted = append(omitted, map[string]string{"command": o.Command, "reason": o.Reason})
	}

	return map[string]any{
		"contract": "Every result is the CLI verb's --json envelope, carrying the CLI's own payload types (D57)",
		"server": map[string]any{
			"name":    "stemma-mcp",
			"version": version.Version,
			"invoke":  "stemma-mcp --kb /path/to/kb",
		},
		"envelope": Envelope(map[string]any{}),
		"errors": []map[string]string{
			{"when": "the tool ran clean", "result": "ok true with data; the twin of exit code 0"},
			{"when": "the KB has findings", "result": "ok false with findings; the twin of exit code 1, returned as a result rather than a protocol error"},
			{"when": "the run could not complete", "result": "ok false with error; the twin of exit code 2, returned as a result rather than a protocol error"},
			{"when": "the arguments do not match the input schema", "result": "a JSON-RPC invalid-params error, refused before the tool runs"},
		},
		"tools":   tools,
		"omitted": omitted,
	}
}

// JSONBytes renders the schema set as it is written to disk.
func JSONBytes() ([]byte, error) {
	b, err := json.MarshalIndent(BuildDoc(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// ReferenceBytes renders the tool reference as markdown. It is rendered from
// the schema set as marshalled — through a round trip through the artifact's
// own bytes — so what the reference describes is what the JSON says, never
// something the registry meant.
func ReferenceBytes() ([]byte, error) {
	b, err := JSONBytes()
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	var sb strings.Builder
	writeReference(&sb, doc)
	return []byte(sb.String()), nil
}

// writeReference renders the whole reference: the contract, the envelope, the
// outcomes, the tools in registry order, and the omissions.
func writeReference(sb *strings.Builder, doc map[string]any) {
	fmt.Fprint(sb, "# The stemma-mcp tool reference\n\n")
	fmt.Fprint(sb, "This file is generated from the tool registry; `make mcp-docs` writes it beside\n")
	fmt.Fprint(sb, "`docs/mcp-tools.json`, and the tests fail when either is stale. Do not edit.\n\n")
	fmt.Fprintf(sb, "The server is `%s`, started as `%s`. It speaks protocol %s over\n",
		doc["server"].(map[string]any)["name"],
		doc["server"].(map[string]any)["invoke"],
		ProtocolVersion)
	fmt.Fprint(sb, "newline-delimited JSON-RPC on stdin and stdout; its own working goes to stderr.\n\n")
	fmt.Fprintf(sb, "%s.\n\n", doc["contract"])

	sb.WriteString("## The envelope\n\n")
	fmt.Fprint(sb, "Every result carries the same envelope a `--json` run prints:\n\n")
	writeShapeTable(sb, "Field", doc["envelope"].(map[string]any)["properties"].(map[string]any), nil)
	sb.WriteString("\nA finding, as it appears in `findings`:\n\n")
	findings := doc["envelope"].(map[string]any)["properties"].(map[string]any)["findings"].(map[string]any)
	writeBullets(sb, findings["items"].(map[string]any), "")

	sb.WriteString("\n## Outcomes\n\n")
	sb.WriteString("| When | What a caller reads |\n| --- | --- |\n")
	for _, e := range doc["errors"].([]any) {
		err := e.(map[string]any)
		fmt.Fprintf(sb, "| %s | %s |\n", err["when"], err["result"])
	}

	sb.WriteString("\n## Tools\n")
	for _, t := range doc["tools"].([]any) {
		tool := t.(map[string]any)
		fmt.Fprintf(sb, "\n### %s\n\n", tool["name"])
		fmt.Fprintf(sb, "Mirrors `%s`.\n\n%s\n\n", tool["command"], tool["description"])
		sb.WriteString("#### Arguments\n\n")
		input := tool["input"].(map[string]any)
		props := input["properties"].(map[string]any)
		if len(props) == 0 {
			sb.WriteString("Takes no arguments.\n")
			continue
		}
		var required []string
		if raw, ok := input["required"].([]any); ok {
			for _, r := range raw {
				required = append(required, r.(string))
			}
		}
		writeShapeTable(sb, "Argument", props, required)
		sb.WriteString("\n#### Result\n\n")
		sb.WriteString("`data` carries:\n\n")
		data := tool["result"].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)
		if props := data["properties"]; props != nil && len(props.(map[string]any)) > 0 {
			writeBullets(sb, data, "")
		} else {
			sb.WriteString("nothing; the outcome is in `ok`, `findings`, and `error`.\n")
		}
	}

	sb.WriteString("\n## Commands left on the CLI\n\n")
	fmt.Fprintf(sb, "The surface is a curated subset (%s), and a subset whose gaps are\n", "D58")
	sb.WriteString("unstated is a smaller mirror. Every verb the command table defines that no\n")
	sb.WriteString("tool mirrors is named here with its reason:\n\n")
	sb.WriteString("| Command | Why it stays on the CLI |\n| --- | --- |\n")
	for _, o := range doc["omitted"].([]any) {
		om := o.(map[string]any)
		fmt.Fprintf(sb, "| `%s` | %s |\n", om["command"], om["reason"])
	}
}

// writeShapeTable renders one level of a schema's properties as a table,
// with the required list marking which rows bind. label names the first
// column, which is arguments in an input schema and fields in an envelope.
func writeShapeTable(sb *strings.Builder, label string, props map[string]any, required []string) {
	sb.WriteString("| " + label + " | Type | Required | Description |\n| --- | --- | --- | --- |\n")
	for _, name := range sortedKeys(props) {
		prop := props[name].(map[string]any)
		req := "no"
		for _, r := range required {
			if r == name {
				req = "**yes**"
			}
		}
		desc := ""
		if d, ok := prop["description"].(string); ok {
			desc = d
		}
		fmt.Fprintf(sb, "| `%s` | %s | %s | %s |\n", name, typeName(prop), req, desc)
	}
}

// writeBullets renders a shape as a nested bullet list, one level of fields
// per indent, recursing into objects and arrays of objects so a payload's
// whole tree is in the reference.
func writeBullets(sb *strings.Builder, shape map[string]any, indent string) {
	props, _ := shape["properties"].(map[string]any)
	var required []string
	// After the round trip through the artifact, a required list is []any;
	// before it, the renderer built []string. Both arrive here.
	switch raw := shape["required"].(type) {
	case []string:
		required = raw
	case []any:
		for _, r := range raw {
			required = append(required, r.(string))
		}
	}
	for _, name := range sortedKeys(props) {
		prop := props[name].(map[string]any)
		always := "optional"
		for _, r := range required {
			if r == name {
				always = "always present"
			}
		}
		if indent == "" {
			fmt.Fprintf(sb, "%s- `%s` — *%s, %s*\n", indent, name, typeName(prop), always)
		} else {
			fmt.Fprintf(sb, "%s- `%s` — *%s*\n", indent, name, typeName(prop))
		}
		switch prop["type"] {
		case "object":
			writeBullets(sb, prop, indent+"    ")
		case "array":
			if items, ok := prop["items"].(map[string]any); ok && items["type"] == "object" {
				writeBullets(sb, items, indent+"    ")
			}
		}
	}
}

// typeName names a shape the way a reader says it: string, integer, array of
// string, object.
func typeName(shape map[string]any) string {
	switch shape["type"] {
	case "array":
		if items, ok := shape["items"].(map[string]any); ok {
			return "array of " + typeName(items)
		}
		return "array"
	case "object":
		if props, ok := shape["properties"].(map[string]any); ok && len(props) == 0 {
			return "any"
		}
		return "object"
	default:
		if t, ok := shape["type"].(string); ok {
			return t
		}
		return "any"
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ManBytes renders the server's manual page in roff, the way the command
// surface's pages are rendered from the command table, so `make man` writes a
// page for the second binary without anyone hand-writing one. Like those
// pages it is generated and never committed.
func ManBytes() ([]byte, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, ".TH STEMMA-MCP 1 %q "+"\"stemma %s\" \"stemma Manual\"\n",
		time.Now().Format("2006-01-02"), roff(version.Version))
	sb.WriteString(".SH NAME\n")
	sb.WriteString("stemma\\-mcp \\- serve a knowledge base to agents over the Model Context Protocol\n")
	sb.WriteString(".SH SYNOPSIS\n")
	sb.WriteString("\\fBstemma\\-mcp\\fR [\\fB\\-\\-kb\\fR \\fIpath\\fR]\n")
	sb.WriteString(".SH DESCRIPTION\n")
	sb.WriteString("stemma\\-mcp serves a knowledge base to agents as native tools on the\n")
	sb.WriteString("Model Context Protocol. It reads newline\\-delimited JSON\\-RPC from\n")
	sb.WriteString("standard input and writes every response to standard output; its own\n")
	sb.WriteString("working goes to standard error, which a harness may capture or ignore.\n")
	sb.WriteString("The protocol is the 2026\\-07\\-28 specification: stateless, with every\n")
	sb.WriteString("request naming its version, and server/discover advertising the surface.\n\n")
	sb.WriteString("The tools are a curated subset of the command surface: the read core\n")
	sb.WriteString("(status, list, search, show, graph, cite_show) plus the mutations new,\n")
	sb.WriteString("promote and cite_add, and lint as the guard. Every result is the matching\n")
	sb.WriteString("verb's \\-\\-json envelope, with the payload the command would print and the\n")
	sb.WriteString("findings the command would report; a refusal on the command line is a\n")
	sb.WriteString("result here, never a protocol error. The commands deliberately left on the\n")
	sb.WriteString("CLI are named, with reasons, in the generated reference.\n\n")
	sb.WriteString("A call may name the knowledge base it wants in _meta as stemma/kb; with\n")
	sb.WriteString("neither that nor \\-\\-kb, the KB is found as the CLI finds one: STEMMA_KB,\n")
	sb.WriteString("then a walk up from the working directory.\n")
	sb.WriteString(".SH OPTIONS\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B \\-\\-kb \fIpath\fR\n")
	sb.WriteString("the KB root; when not given, STEMMA_KB, else discovered by walking up\n")
	sb.WriteString(".SH EXIT STATUS\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B 0\n")
	sb.WriteString("the session ended cleanly\n")
	sb.WriteString(".TP\n")
	sb.WriteString(".B 2\n")
	sb.WriteString("the server itself failed; findings travel inside tool results, never as\n")
	sb.WriteString("an exit code\n")
	sb.WriteString(".SH SEE ALSO\n")
	sb.WriteString("\\fBstemma\\fR(1)\n\n")
	sb.WriteString("The full tool reference, generated from the same registry, is\n")
	sb.WriteString("docs/mcp\\-tools.md, written by make mcp\\-docs.\n")
	return []byte(sb.String()), nil
}

// roff escapes a string for roff running text: a hyphen reads as a minus
// sign unless it is escaped, and a literal backslash starts an escape.
func roff(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "-", "\\-")
}

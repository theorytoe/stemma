package mcp

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/theorytoe/stemma/internal/cli"
)

// curated is the tool set D80 names. The test holds Tools to it exactly: the
// curation is itself the decision, so a tool added or dropped has to land
// here and in the decision record together.
var curated = []string{
	"status", "list", "search", "show", "graph",
	"new", "promote", "cite_add", "cite_show", "lint",
}

func TestToolsAreTheCuratedSet(t *testing.T) {
	if len(Tools) != len(curated) {
		t.Fatalf("the surface carries %d tools; D80 curates %d", len(Tools), len(curated))
	}
	for i, tool := range Tools {
		if tool.Name != curated[i] {
			t.Errorf("tool %d is %q, want %q", i, tool.Name, curated[i])
		}
	}
}

func TestToolsMirrorRealCommands(t *testing.T) {
	names := cli.CommandNames()
	seen := map[string]bool{}
	for _, tool := range Tools {
		if tool.Name == "" || tool.Command == "" || tool.Description == "" {
			t.Errorf("tool %q is missing a name, a command, or a description", tool.Name)
		}
		if seen[tool.Name] {
			t.Errorf("two tools are named %q", tool.Name)
		}
		seen[tool.Name] = true
		if !slices.Contains(names, tool.Command) {
			t.Errorf("tool %q mirrors %q, which the command table does not define", tool.Name, tool.Command)
		}
		if tool.Payload == nil {
			t.Errorf("tool %q carries no payload", tool.Name)
		}
		if got := tool.Input["type"]; got != "object" {
			t.Errorf("tool %q input schema has type %v, want object", tool.Name, got)
		}
	}
}

func TestOmissionsCoverTheRest(t *testing.T) {
	names := cli.CommandNames()
	covered := map[string]bool{}
	for _, tool := range Tools {
		covered[tool.Command] = true
	}
	for _, o := range Omissions {
		if o.Reason == "" {
			t.Errorf("omission %q states no reason", o.Command)
		}
		if !slices.Contains(names, o.Command) {
			t.Errorf("omission %q names a command the table does not define", o.Command)
		}
		if covered[o.Command] {
			t.Errorf("%q is both a tool and an omission", o.Command)
		}
		covered[o.Command] = true
	}
	// Every verb the table defines is either curated or omitted. A verb that
	// is neither has fallen between the two records, and the reference would
	// say nothing about it.
	for _, name := range names {
		if !covered[name] {
			t.Errorf("command %q is neither curated nor omitted; the curation is incomplete", name)
		}
	}
}

func TestShapesReadOffThePayloads(t *testing.T) {
	show := Shape(cli.ShowReport{})
	props, ok := show["properties"].(map[string]any)
	if !ok {
		t.Fatalf("the show shape is not an object: %v", show)
	}
	for _, want := range []string{"path", "title", "type", "status", "body", "links", "citations"} {
		if _, ok := props[want]; !ok {
			t.Errorf("the show shape is missing %q", want)
		}
	}

	// A field is required exactly when its tag lacks omitempty, which is when
	// encoding/json always writes it. ShowLink.Matches carries omitempty, so a
	// consumer must be told it may be absent.
	item := props["links"].(map[string]any)["items"].(map[string]any)
	required := item["required"].([]string)
	if !slices.Contains(required, "target") {
		t.Errorf("the link shape does not require target: %v", required)
	}
	if slices.Contains(required, "matches") {
		t.Errorf("the link shape requires matches, which carries omitempty: %v", required)
	}
}

func TestEnvelopeCarriesTheOutcome(t *testing.T) {
	props := Envelope(Shape(cli.LintSummary{}))["properties"].(map[string]any)
	for _, want := range []string{"command", "ok", "data", "findings", "error"} {
		if _, ok := props[want]; !ok {
			t.Errorf("the envelope is missing %q", want)
		}
	}
	findings := props["findings"].(map[string]any)
	if findings["type"] != "array" {
		t.Errorf("findings is %v, want an array", findings["type"])
	}
	item := findings["items"].(map[string]any)
	fprops := item["properties"].(map[string]any)
	for _, want := range []string{"severity", "code", "path", "message"} {
		if _, ok := fprops[want]; !ok {
			t.Errorf("the finding shape is missing %q", want)
		}
	}

	// The result a tool advertises is what tools/list will send, so every one
	// of them has to marshal.
	for _, tool := range Tools {
		if _, err := json.Marshal(Envelope(Shape(tool.Payload))); err != nil {
			t.Errorf("tool %q result shape does not marshal: %v", tool.Name, err)
		}
	}
}

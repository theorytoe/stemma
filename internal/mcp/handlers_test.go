package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// writeKB builds a small, clean KB: two linked pages citing one source, and
// one draft waiting in the inbox.
func writeKB(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"stemma.toml":        "title = \"Test KB\"\ntypes = [\"note\"]\ndefault_type = \"note\"\n",
		"bibliography.bib":   "@article{knuth1974,\n  title = {Computer Programming as an Art},\n  author = {Knuth, Donald E.},\n  year = {1974},\n}\n",
		"pages/one.md":       page("One", "type: note", "See [[Two]] and [@knuth1974].\n"),
		"pages/two.md":       page("Two", "type: note", "Back to [[One]].\n"),
		"inbox/draft-one.md": page("Draft One", "type: note", "A draft.\n"),
	}
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func page(title, extra, body string) string {
	s := "---\ntitle: " + title + "\n"
	if extra != "" {
		s += extra + "\n"
	}
	return s + "---\n" + body
}

// call runs one tools/call through the server with the real handlers and
// returns the envelope it answered with.
func call(t *testing.T, root, name, args string) map[string]any {
	t.Helper()
	params := `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `","stemma/kb":"` + root + `"},"name":"` + name + `","arguments":` + args + `}`
	responses, _ := serve(t, ToolHandlers(), ask(1, "tools/call", params))
	result, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("%s returned no result: %v", name, responses[0])
	}
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("%s returned no envelope: %v", name, result)
	}
	return structured
}

func TestEveryToolIsWired(t *testing.T) {
	handlers := ToolHandlers()
	if len(handlers) != len(Tools) {
		t.Fatalf("%d handlers for %d tools", len(handlers), len(Tools))
	}
	for _, tool := range Tools {
		if handlers[tool.Name] == nil {
			t.Errorf("tool %q is advertised but not wired", tool.Name)
		}
	}
}

func TestStatusAndListReportTheKB(t *testing.T) {
	root := writeKB(t)

	env := call(t, root, "status", `{}`)
	if env["ok"] != true || env["command"] != "status" {
		t.Fatalf("status envelope: %v", env)
	}
	data := env["data"].(map[string]any)
	if data["pages"] != float64(2) {
		t.Errorf("status counts %v pages, want 2", data["pages"])
	}
	if data["drafts"] != float64(1) {
		t.Errorf("status counts %v drafts, want 1", data["drafts"])
	}
	byType := data["by_type"].(map[string]any)
	if byType["note"] != float64(2) {
		t.Errorf("by_type = %v, want two notes", byType)
	}

	env = call(t, root, "list", `{"type":"note"}`)
	data = env["data"].(map[string]any)
	if data["count"] != float64(2) {
		t.Errorf("list counts %v, want 2", data["count"])
	}
}

func TestShowResolvesLinksAndCitations(t *testing.T) {
	root := writeKB(t)

	env := call(t, root, "show", `{"page":"One"}`)
	if env["ok"] != true {
		t.Fatalf("show One: %v", env)
	}
	data := env["data"].(map[string]any)
	if data["title"] != "One" {
		t.Errorf("show returned %v", data["title"])
	}
	links := data["links"].([]any)
	link := links[0].(map[string]any)
	if link["resolved"] != "pages/two.md" {
		t.Errorf("the link to Two resolved to %v", link["resolved"])
	}
	citations := data["citations"].([]any)
	citation := citations[0].(map[string]any)
	if citation["key"] != "knuth1974" || citation["defined"] != true {
		t.Errorf("the citation did not resolve: %v", citation)
	}

	// A page that does not exist is the envelope's error, not a protocol
	// error: the tool ran and the KB had no such page.
	env = call(t, root, "show", `{"page":"No Such Page"}`)
	if env["ok"] != false || env["error"] == nil {
		t.Errorf("a missing page is not an envelope error: %v", env)
	}
}

func TestSearchAndGraphWalkTheKB(t *testing.T) {
	root := writeKB(t)

	env := call(t, root, "search", `{"query":"back"}`)
	data := env["data"].(map[string]any)
	if data["tier"] != "kb" {
		t.Errorf("a KB with no index answered from %v", data["tier"])
	}
	results := data["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["path"] != "pages/two.md" {
		t.Errorf("searching \"back\" found %v", data["results"])
	}

	env = call(t, root, "graph", `{"page":"One","out":true}`)
	data = env["data"].(map[string]any)
	if data["start"] != "pages/one.md" {
		t.Errorf("the walk started at %v", data["start"])
	}
	neighbors := data["neighbors"].([]any)
	if len(neighbors) != 1 || neighbors[0].(map[string]any)["path"] != "pages/two.md" {
		t.Errorf("walking out of One reached %v", data["neighbors"])
	}
}

func TestLintReportsFindings(t *testing.T) {
	root := writeKB(t)
	// A page whose type the vocabulary does not know warns by default and
	// errors under strict (D55), which is the one finding this test can
	// dictate the severity of. Two picks up the link, so Three is no orphan.
	if err := os.WriteFile(filepath.Join(root, "pages", "three.md"),
		[]byte(page("Three", "type: essay", "Linked from [[Two]].\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pages", "two.md"),
		[]byte(page("Two", "type: note", "Back to [[One]] and [[Three]].\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	env := call(t, root, "lint", `{}`)
	if env["ok"] != false {
		t.Fatalf("a KB with an unknown type is not ok: %v", env)
	}
	data := env["data"].(map[string]any)
	if data["warnings"] != float64(1) || data["errors"] != float64(0) {
		t.Errorf("lenient lint = %v, want one warning", data)
	}
	findings := env["findings"].([]any)
	if len(findings) != 1 {
		t.Errorf("lint carried %d findings, want 1", len(findings))
	}

	env = call(t, root, "lint", `{"strict":true}`)
	data = env["data"].(map[string]any)
	if data["errors"] != float64(1) || data["clean"] != false {
		t.Errorf("strict lint = %v, want one error", data)
	}
}

func TestNewAndPromoteMoveADraftIntoThePages(t *testing.T) {
	root := writeKB(t)

	// An unknown type warns by default, and the page is written anyway.
	env := call(t, root, "new", `{"title":"Three","type":"essay"}`)
	if env["ok"] != false {
		t.Fatalf("an unknown type is a finding: %v", env)
	}
	if findings := env["findings"].([]any); len(findings) != 1 {
		t.Errorf("new carried %v findings, want 1", env["findings"])
	}

	// The same request under strict leaves the page unwritten.
	env = call(t, root, "new", `{"title":"Four","type":"essay","strict":true}`)
	if env["ok"] != false {
		t.Fatalf("strict new is not ok: %v", env)
	}
	env = call(t, root, "list", `{}`)
	if env["data"].(map[string]any)["count"] != float64(3) {
		t.Errorf("the strict page was written anyway: %v", env["data"])
	}

	// A title another page answers to is refused outright.
	env = call(t, root, "new", `{"title":"One"}`)
	if env["ok"] != false || env["error"] == nil {
		t.Errorf("a claimed title is not an envelope error: %v", env)
	}

	// The draft promotes: it becomes a page, and the report says where.
	env = call(t, root, "promote", `{"draft":"Draft One"}`)
	if env["ok"] != true {
		t.Fatalf("promote: %v", env)
	}
	data := env["data"].(map[string]any)
	if data["from"] != "inbox/draft-one.md" || data["to"] != "pages/draft-one.md" {
		t.Errorf("promote moved %v -> %v", data["from"], data["to"])
	}

	// A draft the format will not accept as a page fails with findings, and
	// the partial report names the draft it refused.
	if err := os.WriteFile(filepath.Join(root, "inbox", "draft-two.md"),
		[]byte(page("Draft Two", "type: essay", "A draft.\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	env = call(t, root, "promote", `{"draft":"Draft Two"}`)
	if env["ok"] != false {
		t.Fatalf("promoting a draft the format rejects is not ok: %v", env)
	}
	if findings := env["findings"].([]any); len(findings) != 1 {
		t.Errorf("promote carried %v findings, want 1", env["findings"])
	}
	if data := env["data"].(map[string]any); data["from"] != "inbox/draft-two.md" {
		t.Errorf("the refusal does not name the draft: %v", data)
	}
}

func TestCiteAddAndCiteShowRoundTrip(t *testing.T) {
	root := writeKB(t)
	bib := filepath.Join(root, "bibliography.bib")
	before, err := os.ReadFile(bib)
	if err != nil {
		t.Fatal(err)
	}

	// A dry run reports the entry and leaves the file alone.
	env := call(t, root, "cite_add",
		`{"title":"Structure and Interpretation of Computer Programs","authors":["Abelson, Harold","Sussman, Gerald Jay"],"year":"1985","dry_run":true}`)
	if env["ok"] != true {
		t.Fatalf("a hand-entered record: %v", env)
	}
	data := env["data"].(map[string]any)
	if data["action"] != "added" || data["dry_run"] != true {
		t.Errorf("the dry run reports %v / %v", data["action"], data["dry_run"])
	}
	if after, _ := os.ReadFile(bib); string(after) != string(before) {
		t.Errorf("a dry run wrote the bibliography")
	}

	// The same record without dry_run lands in the file.
	env = call(t, root, "cite_add",
		`{"title":"Structure and Interpretation of Computer Programs","authors":["Abelson, Harold","Sussman, Gerald Jay"],"year":"1985"}`)
	if env["ok"] != true || env["data"].(map[string]any)["dry_run"] == true {
		t.Fatalf("the written record: %v", env)
	}
	if after, _ := os.ReadFile(bib); string(after) == string(before) {
		t.Errorf("the record never landed")
	}

	// An identifier that cannot be resolved without the network is an
	// operational failure, the twin of exit 2, not a finding.
	env = call(t, root, "cite_add", `{"identifier":"10.1234/nope","offline":true}`)
	if env["ok"] != false || env["error"] == nil {
		t.Errorf("an offline resolution is not an envelope error: %v", env)
	}

	// cite_show reads the record back, with the pages that cite it.
	env = call(t, root, "cite_show", `{"key":"knuth1974"}`)
	if env["ok"] != true {
		t.Fatalf("cite_show: %v", env)
	}
	data = env["data"].(map[string]any)
	if data["entry"] == "" {
		t.Errorf("cite_show carried no entry bytes: %v", data)
	}
	citedBy := data["cited_by"].([]any)
	if len(citedBy) != 1 || citedBy[0] != "pages/one.md" {
		t.Errorf("knuth1974 is cited by %v", data["cited_by"])
	}

	env = call(t, root, "cite_show", `{"key":"no-such-key"}`)
	if env["ok"] != false || env["error"] == nil {
		t.Errorf("a missing key is not an envelope error: %v", env)
	}
}

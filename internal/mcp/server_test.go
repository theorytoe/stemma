package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

// serve runs the server over the given lines and returns every response it
// writes, decoded in order, alongside what it logged. The handlers are the
// caller's, so a test can watch what a tool call delivered.
func serve(t *testing.T, handlers Handlers, lines ...string) ([]map[string]any, []string) {
	return serveAt(t, "", handlers, lines...)
}

// serveAt is serve with the server launched with a root, the way the binary
// launches it with --kb.
func serveAt(t *testing.T, root string, handlers Handlers, lines ...string) ([]map[string]any, []string) {
	t.Helper()
	var out, log strings.Builder
	srv := &Server{In: strings.NewReader(strings.Join(lines, "\n") + "\n"), Out: &out, Log: &log, Handlers: handlers, Root: root}
	if err := srv.Serve(); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var responses []map[string]any
	s := bufio.NewScanner(strings.NewReader(out.String()))
	for s.Scan() {
		var m map[string]any
		if err := json.Unmarshal(s.Bytes(), &m); err != nil {
			t.Fatalf("stdout line is not JSON: %v\n%s", err, s.Bytes())
		}
		responses = append(responses, m)
	}
	return responses, strings.Split(strings.TrimSpace(log.String()), "\n")
}

// ask builds one request line with the given id and method.
func ask(id int, method, params string) string {
	return `{"jsonrpc":"2.0","id":` + strconv.Itoa(id) + `,"method":"` + method + `","params":` + params + `}`
}

// meta builds the params object every modern client sends: the version, and
// optionally a named KB root.
func meta(version, root string) string {
	m := `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + version + `"`
	if root != "" {
		m += `,"stemma/kb":"` + root + `"`
	}
	return m + "}}"
}

func TestDiscoverAdvertisesTheSurface(t *testing.T) {
	responses, _ := serve(t, Handlers{}, ask(1, "server/discover", meta(ProtocolVersion, "")))
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1", len(responses))
	}
	result, ok := responses[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("discover returned no result: %v", responses[0])
	}
	versions, ok := result["supportedVersions"].([]any)
	if !ok || len(versions) != 1 || versions[0] != ProtocolVersion {
		t.Errorf("supportedVersions = %v, want [%s]", result["supportedVersions"], ProtocolVersion)
	}
	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("discover names no capabilities: %v", result)
	}
	if _, ok := caps["tools"]; !ok {
		t.Errorf("capabilities do not mention tools: %v", caps)
	}
	if result["ttlMs"] == nil || result["cacheScope"] == nil {
		t.Errorf("discover does not say how long its answer may be cached: %v", result)
	}
	if _, ok := result["instructions"].(string); !ok {
		t.Errorf("discover carries no instructions for a model: %v", result)
	}
}

func TestRequestNamesItsProtocolVersion(t *testing.T) {
	// A request with no version in _meta is malformed, refused with invalid
	// params rather than the version error.
	responses, _ := serve(t, Handlers{}, ask(1, "tools/list", `{}`))
	errMap := responses[0]["error"].(map[string]any)
	if errMap["code"].(float64) != codeInvalidParams {
		t.Errorf("a versionless request got %v, want %d", errMap["code"], codeInvalidParams)
	}

	// A version the server does not speak gets the version error, naming both
	// sides: what was asked and what is supported.
	responses, _ = serve(t, Handlers{}, ask(2, "tools/list", meta("2025-06-18", "")))
	errMap = responses[0]["error"].(map[string]any)
	if errMap["code"].(float64) != codeUnsupportedProto {
		t.Errorf("an old version got %v, want %d", errMap["code"], codeUnsupportedProto)
	}
	data := errMap["data"].(map[string]any)
	if data["requested"] != "2025-06-18" {
		t.Errorf("the error does not name the requested version: %v", data)
	}
	supported, ok := data["supported"].([]any)
	if !ok || len(supported) != 1 || supported[0] != ProtocolVersion {
		t.Errorf("the error does not name the supported versions: %v", data)
	}
}

func TestInitializeIsRetired(t *testing.T) {
	params := `{"protocolVersion":"` + ProtocolVersion + `","capabilities":{},"clientInfo":{"name":"old","version":"1"}}`
	responses, _ := serve(t, Handlers{}, ask(1, "initialize", params))
	errMap := responses[0]["error"].(map[string]any)
	if errMap["code"].(float64) != codeMethodNotFound {
		t.Fatalf("initialize got %v, want %d", errMap["code"], codeMethodNotFound)
	}
	data := errMap["data"].(map[string]any)
	versions, ok := data["supportedVersions"].([]any)
	if !ok || len(versions) != 1 || versions[0] != ProtocolVersion {
		t.Errorf("the refusal does not name the supported versions: %v", data)
	}
}

func TestToolsListMatchesTheRegistry(t *testing.T) {
	responses, _ := serve(t, Handlers{}, ask(7, "tools/list", meta(ProtocolVersion, "")))
	result := responses[0]["result"].(map[string]any)
	if result["resultType"] != "complete" {
		t.Errorf("tools/list resultType = %v", result["resultType"])
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != len(curated) {
		t.Fatalf("tools/list returned %v tools, want %d", result["tools"], len(curated))
	}
	for i, raw := range tools {
		tool := raw.(map[string]any)
		if tool["name"] != curated[i] {
			t.Errorf("tool %d is %v, want %s", i, tool["name"], curated[i])
		}
		for _, field := range []string{"description", "inputSchema", "outputSchema"} {
			if tool[field] == nil {
				t.Errorf("tool %s carries no %s", tool["name"], field)
			}
		}
	}
}

func TestToolCallRunsTheHandler(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "stemma.toml"), []byte("title = \"Test KB\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "pages"), 0o755); err != nil {
		t.Fatal(err)
	}

	var got Call
	handlers := Handlers{
		"show": func(ctx context.Context, call Call) (any, []kb.Finding, error) {
			got = call
			return map[string]string{"path": "pages/one.md"}, nil, nil
		},
	}
	params := `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `","stemma/kb":"` + root + `"},"name":"show","arguments":{"page":"one"}}`
	responses, _ := serve(t, handlers, ask(1, "tools/call", params))

	result := responses[0]["result"].(map[string]any)
	if result["resultType"] != "complete" {
		t.Errorf("resultType = %v", result["resultType"])
	}
	if result["isError"] != false {
		t.Errorf("a clean run is an error: %v", result["isError"])
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["command"] != "show" || structured["ok"] != true {
		t.Errorf("the envelope is not the CLI's: %v", structured)
	}
	if structured["data"].(map[string]any)["path"] != "pages/one.md" {
		t.Errorf("the payload did not travel: %v", structured["data"])
	}
	content := result["content"].([]any)[0].(map[string]any)
	if content["type"] != "text" || !strings.Contains(content["text"].(string), "pages/one.md") {
		t.Errorf("the text content does not carry the payload: %v", content)
	}
	if got.Tool.Name != "show" || got.Root != root {
		t.Errorf("the handler saw %+v", got)
	}
	var args map[string]any
	json.Unmarshal(got.Args, &args)
	if args["page"] != "one" {
		t.Errorf("the handler saw arguments %v", args)
	}
}

func TestToolCallWithoutAWiredHandler(t *testing.T) {
	// A tool the registry advertises but no build wired is a server bug, and
	// it is an internal error rather than a silent absence.
	responses, _ := serve(t, Handlers{}, ask(1, "tools/call",
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"`+ProtocolVersion+`"},"name":"show","arguments":{"page":"one"}}`))
	errMap := responses[0]["error"].(map[string]any)
	if errMap["code"].(float64) != codeInternalError {
		t.Errorf("an unwired tool got %v, want %d", errMap["code"], codeInternalError)
	}

	// A tool the registry does not know at all is the caller's mistake.
	responses, _ = serve(t, Handlers{}, ask(2, "tools/call",
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"`+ProtocolVersion+`"},"name":"no_such_tool"}`))
	errMap = responses[0]["error"].(map[string]any)
	if errMap["code"].(float64) != codeInvalidParams {
		t.Errorf("an unknown tool got %v, want %d", errMap["code"], codeInvalidParams)
	}
}

func TestToolCallArgumentsAreHeldToTheSchema(t *testing.T) {
	nop := func(ctx context.Context, call Call) (any, []kb.Finding, error) {
		return nil, nil, nil
	}
	handlers := Handlers{"lint": nop, "show": nop}
	cases := []struct {
		name    string
		args    string
		problem string
	}{
		{"unknown argument", `{"strict":true,"extra":1}`, "unknown argument"},
		{"wrong type", `{"strict":"yes"}`, "not a boolean"},
		{"missing required", ``, "missing required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `"},"name":"lint","arguments":` + tc.args + `}`
			// lint takes no required arguments; show does, so the missing
			// case asks show for nothing.
			if tc.problem == "missing required" {
				params = `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `"},"name":"show"}`
			}
			responses, _ := serve(t, handlers, ask(1, "tools/call", params))
			errMap, ok := responses[0]["error"].(map[string]any)
			if !ok {
				t.Fatalf("the call was not refused: %v", responses[0])
			}
			if errMap["code"].(float64) != codeInvalidParams {
				t.Errorf("got code %v, want %d", errMap["code"], codeInvalidParams)
			}
			if !strings.Contains(errMap["message"].(string), tc.problem) {
				t.Errorf("message %q does not mention %q", errMap["message"], tc.problem)
			}
		})
	}
}

func TestToolCallNamesARootOrDiscoversOne(t *testing.T) {
	// A KB the call names by path is used as-is when it is one, and a call
	// that names nothing discovers from the working directory or falls back
	// to the root the server was launched with, the way the CLI does (D41).
	root := t.TempDir()
	manifest := filepath.Join(root, "stemma.toml")
	if err := os.WriteFile(manifest, []byte("title = \"Test KB\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "pages"), 0o755); err != nil {
		t.Fatal(err)
	}

	var got Call
	handlers := Handlers{
		"status": func(ctx context.Context, call Call) (any, []kb.Finding, error) {
			got = call
			return nil, nil, nil
		},
	}

	params := `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `","stemma/kb":"` + root + `"},"name":"status"}`
	serve(t, handlers, ask(1, "tools/call", params))
	if got.Root != root {
		t.Errorf("a named root was not used: got %q", got.Root)
	}

	t.Chdir(root)
	serve(t, handlers, ask(2, "tools/call",
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"`+ProtocolVersion+`"},"name":"status"}`))
	if got.Root != root {
		t.Errorf("discovery did not find the KB: got %q", got.Root)
	}

	// The root the server was launched with answers a call that names none,
	// ahead of discovery: from a directory with no KB of its own, only the
	// launch root can answer.
	launched := t.TempDir()
	if err := os.WriteFile(filepath.Join(launched, "stemma.toml"), []byte("title = \"Launched\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(launched, "pages"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	serveAt(t, launched, handlers, ask(3, "tools/call",
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"`+ProtocolVersion+`"},"name":"status"}`))
	if got.Root != launched {
		t.Errorf("the launch root was not used: got %q", got.Root)
	}

	// A KB that is nowhere to be found is the tool's finding, a result in the
	// envelope's terms, not a protocol error.
	elsewhere := filepath.Join(t.TempDir(), "nowhere")
	responses, _ := serve(t, handlers, ask(4, "tools/call",
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"`+ProtocolVersion+`","stemma/kb":"`+elsewhere+`"},"name":"status"}`))
	result := responses[0]["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("a missing KB is not an error result: %v", result)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["ok"] != false || structured["error"] == nil {
		t.Errorf("the envelope does not carry the failure: %v", structured)
	}
}

func TestTheWireRejectsWhatIsNotARequest(t *testing.T) {
	responses, _ := serve(t, Handlers{},
		`{not json`,
		`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`,
		`{"jsonrpc":"1.0","id":2,"method":"ping"}`,
		ask(3, "no/such/method", meta(ProtocolVersion, "")),
	)
	if len(responses) != 4 {
		t.Fatalf("got %d responses, want 4", len(responses))
	}
	wantCodes := []float64{codeParseError, codeInvalidRequest, codeInvalidRequest, codeMethodNotFound}
	for i, want := range wantCodes {
		errMap, ok := responses[i]["error"].(map[string]any)
		if !ok {
			t.Fatalf("response %d carries no error: %v", i, responses[i])
		}
		if errMap["code"].(float64) != want {
			t.Errorf("response %d got code %v, want %v", i, errMap["code"], want)
		}
	}
	if got := responses[0]["id"]; got != nil {
		t.Errorf("a parse error's id is %v, want null", got)
	}
}

func TestNotificationsGetNoReply(t *testing.T) {
	responses, _ := serve(t, Handlers{},
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		ask(1, "ping", meta(ProtocolVersion, "")),
	)
	if len(responses) != 1 {
		t.Fatalf("a notification drew a reply: %v", responses)
	}
	if responses[0]["id"].(float64) != 1 {
		t.Errorf("the only response is not the ping's: %v", responses[0])
	}
}

func TestStdoutCarriesProtocolAndNothingElse(t *testing.T) {
	responses, log := serve(t, Handlers{},
		ask(1, "server/discover", meta(ProtocolVersion, "")),
		ask(2, "tools/list", meta(ProtocolVersion, "")),
		ask(3, "ping", meta(ProtocolVersion, "")),
	)
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3", len(responses))
	}
	if len(log) == 0 || !strings.HasPrefix(log[0], "stemma-mcp:") {
		t.Errorf("the server logged nothing recognisable: %v", log)
	}
}

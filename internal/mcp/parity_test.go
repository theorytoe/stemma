package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/cli"
)

// parityCorpus builds the one corpus every parity case runs against: two
// linked pages carrying a resolved and an unresolved link, a defined and a
// dangling citation, tags, and a draft waiting in the inbox. A corpus that
// exercises nothing finds no disagreement.
func parityCorpus(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"stemma.toml":        "title = \"Parity KB\"\ntypes = [\"note\"]\ndefault_type = \"note\"\n",
		"bibliography.bib":   "@article{knuth1974,\n  title = {Computer Programming as an Art},\n  author = {Knuth, Donald E.},\n  year = {1974},\n}\n",
		"pages/one.md":       page("One", "type: note\ntags: [seed, parity]", "See [[Two]] and [[Ghost Page]]; read [@knuth1974] and [@ghost2020].\n"),
		"pages/two.md":       page("Two", "type: note\ntags: [seed]", "Back to [[One]], citing [@knuth1974].\n"),
		"inbox/draft-one.md": page("Draft One", "type: note", "A draft.\n"),
		"inbox/draft-two.md": page("Draft Two", "type: essay", "The inbox permitted this type; promotion will not.\n"),
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

// runCLI runs one verb the way a shell would and returns the envelope its
// --json run printed, alongside the exit code. The flags trail the verb, as
// the dispatcher reads them.
func runCLI(t *testing.T, root string, args ...string) (map[string]any, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	// The flags go last: the dispatcher reads the verb first, and permute
	// moves the flags ahead of the positional arguments within the command.
	full := append(append([]string{}, args...), "--kb", root, "--json")
	code := cli.Run(full, &stdout, &stderr)
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &envelope); err != nil {
		t.Fatalf("cli %v wrote %q (%s): %v", args, stdout.String(), stderr.String(), err)
	}
	return envelope, code
}

// runTool runs one tool call against a corpus and returns the envelope the
// result carried.
func runTool(t *testing.T, root, name, args string) map[string]any {
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

// expectCode holds the CLI exit code to the envelope's story: an error is
// exit 2, findings are exit 1, a clean run is exit 0. The MCP result carries
// no code, which is exactly why the exit-code twins are asserted here.
func expectCode(t *testing.T, name string, code int, envelope map[string]any) {
	t.Helper()
	want := cli.ExitOK
	switch {
	case envelope["error"] != nil:
		want = cli.ExitError
	case envelope["findings"] != nil:
		want = cli.ExitFindings
	}
	if code != want {
		t.Errorf("%s exited %d, want %d (envelope: %v)", name, code, want, envelope)
	}
}

func TestToolsMatchTheirCommands(t *testing.T) {
	cases := []struct {
		name     string
		tool     string
		toolArgs string
		cli      []string
		mutating bool
	}{
		{"status", "status", `{}`, []string{"status"}, false},
		{"list", "list", `{}`, []string{"list"}, false},
		{"list filtered", "list", `{"type":"note","tags":["seed"]}`, []string{"list", "--type", "note", "--tag", "seed"}, false},
		{"search", "search", `{"query":"back"}`, []string{"search", "back"}, false},
		{"show", "show", `{"page":"One"}`, []string{"show", "One"}, false},
		{"graph", "graph", `{"page":"One","in":true,"out":true}`, []string{"graph", "--in", "--out", "One"}, false},
		{"lint", "lint", `{"strict":true}`, []string{"lint", "--strict"}, false},
		{"cite show", "cite_show", `{"key":"knuth1974"}`, []string{"cite", "show", "knuth1974"}, false},
		{"new", "new", `{"title":"Parity Page","type":"note"}`, []string{"new", "Parity Page", "--type", "note"}, true},
		{"promote", "promote", `{"draft":"Draft One"}`, []string{"promote", "Draft One"}, true},
		{"cite add", "cite_add",
			`{"title":"Structure and Interpretation of Computer Programs","authors":["Abelson, Harold","Sussman, Gerald Jay"],"year":"1985"}`,
			[]string{"cite", "add", "--title", "Structure and Interpretation of Computer Programs",
				"--author", "Abelson, Harold", "--author", "Sussman, Gerald Jay", "--year", "1985"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cliEnv, mcpEnv map[string]any
			if tc.mutating {
				// A mutating run changes the corpus, so each surface works on
				// its own fresh copy and only the envelopes are compared.
				var code int
				cliEnv, code = runCLI(t, parityCorpus(t), tc.cli...)
				expectCode(t, "cli "+tc.name, code, cliEnv)
				mcpEnv = runTool(t, parityCorpus(t), tc.tool, tc.toolArgs)
			} else {
				root := parityCorpus(t)
				var code int
				cliEnv, code = runCLI(t, root, tc.cli...)
				expectCode(t, "cli "+tc.name, code, cliEnv)
				mcpEnv = runTool(t, root, tc.tool, tc.toolArgs)
			}
			if !reflect.DeepEqual(cliEnv, mcpEnv) {
				t.Errorf("envelopes differ\ncli: %#v\nmcp: %#v", cliEnv, mcpEnv)
			}
		})
	}
}

func TestValidationFailuresAgree(t *testing.T) {
	cases := []struct {
		name     string
		tool     string
		toolArgs string
		cli      []string
		// adapted marks the one refusal whose message names the arguments,
		// which the two surfaces spell differently (Task 4's record): the
		// surfaces must both refuse, not say the same words.
		adapted bool
	}{
		{"show missing", "show", `{"page":"Nobody"}`, []string{"show", "Nobody"}, false},
		{"new claimed title", "new", `{"title":"One"}`, []string{"new", "One"}, false},
		{"promote missing draft", "promote", `{"draft":"Nobody"}`, []string{"promote", "Nobody"}, false},
		{"cite show missing key", "cite_show", `{"key":"nobody"}`, []string{"cite", "show", "nobody"}, false},
		{"cite add unresolved offline", "cite_add", `{"identifier":"10.1234/nope","offline":true}`, []string{"cite", "add", "--offline", "10.1234/nope"}, false},
		// An identifier beside an identifier argument is the one refusal whose
		// words differ by design (Task 4's record); it is refused before any
		// resolver runs, so the case never touches the network.
		{"cite add identifier and fields", "cite_add", `{"identifier":"10.1234/nope","doi":"10.5678/x"}`, []string{"cite", "add", "10.1234/nope", "--doi", "10.5678/x"}, true},
		// The inbox permitted this type; the promotion refuses it with a
		// finding — the exit-1 twin, not the exit-2 twin the rest carry.
		{"promote invalid draft", "promote", `{"draft":"Draft Two"}`, []string{"promote", "Draft Two"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := parityCorpus(t)
			cliEnv, code := runCLI(t, root, tc.cli...)
			mcpEnv := runTool(t, root, tc.tool, tc.toolArgs)

			if cliEnv["ok"] != false || mcpEnv["ok"] != false {
				t.Fatalf("the surfaces did not both refuse\ncli: %#v\nmcp: %#v", cliEnv, mcpEnv)
			}
			wantCode := cli.ExitError
			if tc.name == "promote invalid draft" {
				wantCode = cli.ExitFindings
			}
			if code != wantCode {
				t.Errorf("the CLI exited %d, want %d", code, wantCode)
			}
			if tc.adapted {
				// The one refusal whose words differ by design: both surfaces
				// must still name what was wrong.
				if cliEnv["error"] == nil || mcpEnv["error"] == nil {
					t.Fatalf("a refusal without a message\ncli: %#v\nmcp: %#v", cliEnv, mcpEnv)
				}
				return
			}
			if !reflect.DeepEqual(cliEnv, mcpEnv) {
				t.Errorf("envelopes differ\ncli: %#v\nmcp: %#v", cliEnv, mcpEnv)
			}
		})
	}
}

package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBinaryHoldsAConversation drives the shipped binary as a subprocess, the
// way a harness does: real pipes, a real process, requests in, responses
// back, and a clean exit when the input ends. It skips when the binary has
// not been built, because the Go suite has to pass on a machine that has not
// run make build; `make check` builds before it tests.
func TestBinaryHoldsAConversation(t *testing.T) {
	bin := filepath.Join("..", "..", "bin", "stemma-mcp")
	if _, err := os.Stat(bin); err != nil {
		t.Skip("bin/stemma-mcp is not built; run make build first")
	}
	root := writeKB(t)

	cmd := exec.Command(bin, "--kb", root)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	version := meta(ProtocolVersion, "")
	for _, line := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":` + version + `}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":` + version + `}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + ProtocolVersion + `"},"name":"status"}}`,
	} {
		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}

	var responses []map[string]any
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var m map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatalf("the binary wrote a non-JSON line: %v\n%s", err, scanner.Bytes())
		}
		responses = append(responses, m)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("the binary did not exit cleanly: %v\nstderr: %s", err, stderr.String())
	}
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3: %v", len(responses), responses)
	}

	if _, ok := responses[0]["result"].(map[string]any)["supportedVersions"]; !ok {
		t.Errorf("discover answered %v", responses[0])
	}
	tools, ok := responses[1]["result"].(map[string]any)["tools"].([]any)
	if !ok || len(tools) != len(Tools) {
		t.Errorf("tools/list answered %v tools", responses[1]["result"])
	}
	structured, ok := responses[2]["result"].(map[string]any)["structuredContent"].(map[string]any)
	if !ok || structured["ok"] != true {
		t.Errorf("the status call answered %v", responses[2])
	}
	if data := structured["data"].(map[string]any); data["pages"] != float64(2) {
		t.Errorf("the launch root was not used; status saw %v", data)
	}
}

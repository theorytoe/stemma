// Command probe drives the stemma-mcp binary the way a harness does — one
// JSON-RPC message at a time over stdio, no assumptions beyond the protocol —
// and walks a full turn of the curated set against a scratch KB. It is the
// repeatable half of the two-harness exercise the MCP plan's Task 6 asks for;
// the other half is running the server from a real client, which stays out of
// CI. The probe never mutates anything but its own scratch KB.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const protocolVersion = "2026-07-28"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "probe: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	bin := flag.String("bin", "bin/stemma-mcp", "the server binary to drive")
	flag.Parse()
	if _, err := os.Stat(*bin); err != nil {
		return fmt.Errorf("the binary is not built: %w", err)
	}

	kb, err := os.MkdirTemp("", "stemma-probe")
	if err != nil {
		return err
	}
	defer os.RemoveAll(kb)
	if err := scratch(kb); err != nil {
		return err
	}

	cmd := exec.Command(*bin, "--kb", kb)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	reader := bufio.NewReader(stdout)
	var turns int
	ask := func(method, params string) map[string]any {
		turns++
		req, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      turns,
			"method":  method,
			"params":  json.RawMessage(params),
		})
		fmt.Fprintf(stdin, "%s\n", req)
		return expect(turns, reader)
	}

	meta := `{"_meta":{"io.modelcontextprotocol/protocolVersion":"` + protocolVersion + `"}}`

	res := ask("server/discover", meta)
	versions := res["supportedVersions"].([]any)
	if len(versions) != 1 || versions[0] != protocolVersion {
		return fmt.Errorf("discover offered %v", res["supportedVersions"])
	}
	fmt.Println("probe: discover offered", protocolVersion)

	res = ask("tools/list", meta)
	tools := res["tools"].([]any)
	if len(tools) != 10 {
		return fmt.Errorf("tools/list offered %d tools", len(tools))
	}
	fmt.Println("probe: tools/list offered", len(tools), "tools")

	res = call(stdin, reader, &turns, "new", `{"title":"Probe Page","type":"note"}`)
	if res["ok"] != true {
		return fmt.Errorf("new did not write the page: %v", res)
	}
	res = call(stdin, reader, &turns, "status", `{}`)
	data := res["data"].(map[string]any)
	if data["pages"] != float64(1) || data["drafts"] != float64(1) {
		return fmt.Errorf("status saw %v pages and %v drafts", data["pages"], data["drafts"])
	}
	fmt.Println("probe: new wrote a page; status counts it")

	res = call(stdin, reader, &turns, "promote", `{"draft":"Probe Draft"}`)
	if res["ok"] != true || res["data"].(map[string]any)["to"] != "pages/probe-draft.md" {
		return fmt.Errorf("promote answered %v", res)
	}
	res = call(stdin, reader, &turns, "status", `{}`)
	if res["data"].(map[string]any)["drafts"] != float64(0) {
		return fmt.Errorf("the draft is still in the inbox: %v", res)
	}
	fmt.Println("probe: promote moved the draft; status agrees")

	res = call(stdin, reader, &turns, "lint", `{"strict":true}`)
	// The scratch KB keeps one orphan: the promoted page is linked but links
	// to its neighbour, and nothing links back to it. Strict lint says so,
	// and the probe holds the answer to what is true of the KB.
	if res["ok"] != false {
		return fmt.Errorf("strict lint missed the orphan: %v", res)
	}
	lintData := res["data"].(map[string]any)
	if lintData["errors"] != float64(1) {
		return fmt.Errorf("strict lint counted %v errors, want 1: %v", lintData["errors"], res)
	}
	fmt.Println("probe: lint reported the one orphan the scratch KB has")

	res = call(stdin, reader, &turns, "cite_add",
		`{"title":"Probe Source","authors":["Author, A. N."],"year":"2020","key":"probe2020","dry_run":true}`)
	if res["data"].(map[string]any)["action"] != "added" {
		return fmt.Errorf("cite add answered %v", res)
	}
	// The dry run wrote nothing, so the record is entered for real before it
	// is read back.
	res = call(stdin, reader, &turns, "cite_add",
		`{"title":"Probe Source","authors":["Author, A. N."],"year":"2020","key":"probe2020"}`)
	if res["data"].(map[string]any)["dry_run"] == true {
		return fmt.Errorf("the record never landed: %v", res)
	}
	res = call(stdin, reader, &turns, "cite_show", `{"key":"probe2020"}`)
	if res["data"].(map[string]any)["entry"] == "" {
		return fmt.Errorf("cite show answered %v", res)
	}
	fmt.Println("probe: cite add and cite show round-tripped a record")

	if err := stdin.Close(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("the server did not exit cleanly: %w", err)
	}
	fmt.Printf("probe: %d turns answered; server exited cleanly\n", turns)
	return nil
}

// call sends one tools/call and returns its envelope.
func call(stdin io.WriteCloser, reader *bufio.Reader, turns *int, name, args string) map[string]any {
	*turns++
	params := fmt.Sprintf(`{"_meta":{"io.modelcontextprotocol/protocolVersion":"%s"},"name":%q,"arguments":%s}`,
		protocolVersion, name, args)
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      *turns,
		"method":  "tools/call",
		"params":  json.RawMessage(params),
	})
	fmt.Fprintf(stdin, "%s\n", req)
	res := expect(*turns, reader)
	structured, ok := res["structuredContent"].(map[string]any)
	if !ok {
		fmt.Fprintf(os.Stderr, "probe: %s got no envelope: %v\n", name, res)
		os.Exit(1)
	}
	return structured
}

// expect reads one response and holds it to the id it was asked with. A
// response without a result is a refusal, and the probe reports it instead of
// dereferencing it.
func expect(id int, reader *bufio.Reader) map[string]any {
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: the server stopped answering at turn %d: %v\n", id, err)
		os.Exit(1)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(line), &res); err != nil {
		fmt.Fprintf(os.Stderr, "probe: the server wrote %q: %v\n", line, err)
		os.Exit(1)
	}
	if res["id"] != float64(id) {
		fmt.Fprintf(os.Stderr, "probe: response %v answers turn %d\n", res["id"], id)
		os.Exit(1)
	}
	result, ok := res["result"].(map[string]any)
	if !ok {
		fmt.Fprintf(os.Stderr, "probe: turn %d was refused: %s\n", id, line)
		os.Exit(1)
	}
	return result
}

// scratch writes the KB the probe works on: a manifest with one type, a draft
// in the inbox, and nothing else.
func scratch(root string) error {
	if err := os.MkdirAll(filepath.Join(root, "pages"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		return err
	}
	manifest := "title = \"Probe KB\"\ntypes = [\"note\"]\ndefault_type = \"note\"\n"
	if err := os.WriteFile(filepath.Join(root, "stemma.toml"), []byte(manifest), 0o644); err != nil {
		return err
	}
	draft := "---\ntitle: Probe Draft\ntype: note\n---\nA draft the probe promotes. See [[Probe Page]].\n"
	if err := os.WriteFile(filepath.Join(root, "inbox", "probe-draft.md"), []byte(draft), 0o644); err != nil {
		return err
	}
	// The bibliography a fresh KB starts with, so cite add has somewhere to
	// put a record.
	bib := "% The bibliography for this KB, as BibTeX.\n"
	return os.WriteFile(filepath.Join(root, "bibliography.bib"), []byte(bib), 0o644)
}

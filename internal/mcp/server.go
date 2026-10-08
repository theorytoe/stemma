package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/theorytoe/stemma/internal/cli"
	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/version"
)

// ProtocolVersion is the MCP revision this server speaks: the 2026-07-28
// specification, which made the protocol stateless. There is no handshake and
// no session: every request carries the protocol version it wants in its
// _meta, and a request that names a version this server does not support is
// refused with the versions it does. The version is tracked deliberately
// (D81) because the server is hand-rolled — no dependency inherits the
// tracking for it.
const ProtocolVersion = "2026-07-28"

// Keys a request may carry in its params._meta. The io.modelcontextprotocol
// namespace belongs to the specification; stemma/kb is this server's own, the
// way a call names the KB root it wants to work on. A call that names no root
// is resolved the way the CLI resolves one: the named root, then STEMMA_KB,
// then a walk up from the working directory (D41).
const (
	metaProtocolVersion = "io.modelcontextprotocol/protocolVersion"
	metaServerInfo      = "io.modelcontextprotocol/serverInfo"
	metaKB              = "stemma/kb"
)

// JSON-RPC error codes. The first five are JSON-RPC 2.0's own; the last is
// the 2026-07-28 specification's unsupported-protocol-version code, whose
// data names the versions the server does support.
const (
	codeParseError       = -32700
	codeInvalidRequest   = -32600
	codeMethodNotFound   = -32601
	codeInvalidParams    = -32602
	codeInternalError    = -32603
	codeUnsupportedProto = -32022
)

// Handler runs one curated tool against a KB. It returns what the envelope
// carries: the payload on a clean or found-something run, the findings about
// what was asked, or the error when the run could not complete. The server
// composes the envelope, so a handler answers the KB's question and the
// wire-level shape stays in one place.
type Handler func(ctx context.Context, call Call) (data any, findings []kb.Finding, err error)

// Call is one invocation of a tool: its registry entry, its arguments as they
// arrived, and the KB root the call resolved to. The root is resolved before
// the handler runs, so a handler reads a KB without knowing how the caller
// named it.
type Call struct {
	Tool Tool
	Args json.RawMessage
	Root string
}

// Handlers maps a tool's wire name to the code that runs it. A name in the
// registry without a handler here, or a handler without a registry entry, is
// a wiring mistake the tests catch.
type Handlers map[string]Handler

// request is one incoming line. The id is kept raw so it is echoed back
// exactly as it arrived, number or string.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// response is one outgoing line. A parse error answers a message that never
// decoded, so its id is the literal null JSON-RPC prescribes.
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the error member of a JSON-RPC response.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Serve runs one session: it reads newline-delimited JSON-RPC from in,
// answers each request on out, and reports its own working on log. Requests
// are answered in order, one at a time — the protocol allows more, and one
// invocation working one KB has no reason to race itself (D41).
//
// Stdout carries protocol traffic and nothing else: every byte written there
// is one marshalled response. Anything the server has to say about itself
// goes to log, which a harness may capture, forward, or ignore.
func Serve(in io.Reader, out, log io.Writer, handlers Handlers) error {
	r := bufio.NewReader(in)
	logf(log, "stemma-mcp %s serving protocol %s", version.Version, ProtocolVersion)
	for {
		line, err := r.ReadString('\n')
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			serveLine(out, log, handlers, trimmed)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// serveLine decodes and answers one message. A line that is not a JSON
// object, or an object that is not one request, is refused the way JSON-RPC
// prescribes: a parse error for undecodable text, an invalid-request error
// for a batch, each with a null id.
func serveLine(out, log io.Writer, handlers Handlers, line string) {
	var probe any
	if err := json.Unmarshal([]byte(line), &probe); err != nil {
		logf(log, "undecodable message: %.80s", line)
		write(out, response{ID: json.RawMessage("null"), Error: &rpcError{
			Code:    codeParseError,
			Message: "the message is not valid JSON",
		}})
		return
	}
	// A batch is well-formed JSON but not part of the protocol, so it is an
	// invalid request rather than a parse failure, refused with a null id.
	if _, batch := probe.([]any); batch {
		write(out, response{ID: json.RawMessage("null"), Error: &rpcError{
			Code:    codeInvalidRequest,
			Message: "batch requests are not part of the protocol",
		}})
		return
	}
	var req request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		write(out, response{ID: json.RawMessage("null"), Error: &rpcError{
			Code:    codeInvalidRequest,
			Message: "not a JSON-RPC 2.0 request",
		}})
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		write(out, response{ID: req.ID, Error: &rpcError{
			Code:    codeInvalidRequest,
			Message: "not a JSON-RPC 2.0 request",
		}})
		return
	}
	// A message without an id is a notification: it gets no answer, whatever
	// it asks for. The stateless specification retired the one notification
	// the handshake era had, so there is nothing to act on either.
	if len(req.ID) == 0 || string(req.ID) == "null" {
		logf(log, "ignoring notification %s", req.Method)
		return
	}

	start := time.Now()
	res, rpcErr := dispatch(log, handlers, &req)
	if rpcErr != nil {
		logf(log, "%s refused: %s", req.Method, rpcErr.Message)
		write(out, response{ID: req.ID, Error: rpcErr})
		return
	}
	logf(log, "%s answered in %s", req.Method, time.Since(start).Round(time.Microsecond))
	write(out, response{ID: req.ID, Result: res})
}

// dispatch routes one request by method and runs the version check first:
// under the stateless specification every request names its version, and one
// that names another is refused before anything about the method is read.
func dispatch(log io.Writer, handlers Handlers, req *request) (any, *rpcError) {
	// The handshake was retired by the version this server speaks. A client
	// that sends it is speaking an older revision, so the answer comes before
	// the version check: the refusal names what this server does support,
	// which is what the specification asks of a modern-only server, and it is
	// the one thing a client that old can act on.
	if req.Method == "initialize" {
		return nil, &rpcError{
			Code:    codeMethodNotFound,
			Message: "initialize was retired by protocol " + ProtocolVersion + "; call server/discover",
			Data:    map[string]any{"supportedVersions": []string{ProtocolVersion}},
		}
	}
	requested, rpcErr := protocolVersion(req.Params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	if requested != ProtocolVersion {
		return nil, &rpcError{
			Code:    codeUnsupportedProto,
			Message: fmt.Sprintf("protocol version %q is not supported", requested),
			Data: map[string]any{
				"supported": []string{ProtocolVersion},
				"requested": requested,
			},
		}
	}

	switch req.Method {
	case "server/discover":
		return discover(), nil
	case "tools/list":
		return listTools(), nil
	case "tools/call":
		return callTool(log, handlers, req.Params)
	case "ping":
		return map[string]any{}, nil
	default:
		return nil, &rpcError{
			Code:    codeMethodNotFound,
			Message: fmt.Sprintf("no such method %q", req.Method),
		}
	}
}

// protocolVersion reads the version a request names. Its absence is a
// malformed request, not a version mismatch: the specification makes the key
// mandatory on every request and sends malformed ones away with invalid
// params, reserving the version error for a version that is present and
// unsupported.
func protocolVersion(params json.RawMessage) (string, *rpcError) {
	meta, rpcErr := requestMeta(params)
	if rpcErr != nil {
		return "", rpcErr
	}
	raw, ok := meta[metaProtocolVersion]
	if !ok {
		return "", &rpcError{
			Code:    codeInvalidParams,
			Message: "every request names its protocol version in _meta." + metaProtocolVersion,
		}
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil || v == "" {
		return "", &rpcError{
			Code:    codeInvalidParams,
			Message: "_meta." + metaProtocolVersion + " is not a version string",
		}
	}
	return v, nil
}

// requestMeta reads a request's params._meta as a raw key-value map, without
// deciding what any key means.
func requestMeta(params json.RawMessage) (map[string]json.RawMessage, *rpcError) {
	if len(params) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var p struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "params is not an object"}
	}
	if p.Meta == nil {
		return map[string]json.RawMessage{}, nil
	}
	return p.Meta, nil
}

// callTool runs one tool: its arguments are held to the input schema the
// registry advertises, its KB root is resolved, and its handler runs. An
// unknown tool is a protocol error — the client asked for something this
// server does not have — while everything the tool itself finds, including a
// KB that is not there, is a result in the envelope's own terms.
func callTool(log io.Writer, handlers Handlers, params json.RawMessage) (any, *rpcError) {
	meta, rpcErr := requestMeta(params)
	if rpcErr != nil {
		return nil, rpcErr
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "params is not an object"}
	}
	var tool *Tool
	for i := range Tools {
		if Tools[i].Name == p.Name {
			tool = &Tools[i]
			break
		}
	}
	if tool == nil {
		return nil, &rpcError{
			Code:    codeInvalidParams,
			Message: fmt.Sprintf("unknown tool: %q", p.Name),
		}
	}
	handler, wired := handlers[tool.Name]
	if !wired {
		return nil, &rpcError{
			Code:    codeInternalError,
			Message: fmt.Sprintf("tool %q is advertised but not wired", tool.Name),
		}
	}
	if err := validateArgs(tool.Input, p.Arguments); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}

	// The root the call named, or the one discovery finds. A root that cannot
	// be resolved is the tool's finding to report, not a protocol error: it
	// becomes the envelope's error, the twin of the CLI's exit 2, with the
	// same message the command would print.
	var named string
	if raw, ok := meta[metaKB]; ok {
		if err := json.Unmarshal(raw, &named); err != nil {
			return nil, &rpcError{
				Code:    codeInvalidParams,
				Message: "_meta." + metaKB + " is not a path",
			}
		}
	}
	root, err := cli.Discover(named)
	if err != nil {
		logf(log, "tools/call %s: %v", tool.Name, err)
		return toolResult(cli.Response{Command: tool.Command, Error: err.Error()}), nil
	}

	data, findings, herr := handler(context.Background(), Call{Tool: *tool, Args: p.Arguments, Root: root})
	resp := cli.Response{Command: tool.Command}
	switch {
	case herr != nil:
		resp.Error = herr.Error()
	default:
		resp.OK = len(findings) == 0
		resp.Data = data
		if len(findings) > 0 {
			resp.Findings = make([]cli.JSONFinding, len(findings))
			for i, f := range findings {
				resp.Findings[i] = cli.JSONFindingOf(f)
			}
		}
	}
	return toolResult(resp), nil
}

// toolResult frames one envelope for the wire. The envelope is the payload
// the CLI would print, so it travels as the structured content whole; the
// text content carries the same JSON for a client that reads only text; and
// isError mirrors ok, so a harness that renders errors sees what the exit
// codes would have said.
func toolResult(resp cli.Response) map[string]any {
	pretty, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		pretty = []byte("{}")
	}
	return map[string]any{
		"resultType":        "complete",
		"content":           []map[string]any{{"type": "text", "text": string(pretty)}},
		"structuredContent": resp,
		"isError":           !resp.OK,
		"_meta":             map[string]any{metaServerInfo: serverInfo()},
	}
}

// discover answers server/discover: the versions this server speaks, its
// capabilities, and the guidance a client can hand its model. The set is
// fixed at build time, so the answer is cacheable for as long as the server
// process is the one running.
func discover() map[string]any {
	return map[string]any{
		"supportedVersions": []string{ProtocolVersion},
		"capabilities":      map[string]any{"tools": map[string]any{}},
		"instructions": "Stemma serves a knowledge base of markdown pages linked by " +
			"[[wikilinks]] that cite sources as [@key] from a BibTeX bibliography. " +
			"Start with status to see what the KB holds, list or search to find pages, " +
			"and show to read one with its links and citations resolved. Every result " +
			"is the matching CLI verb's --json envelope: check ok, read data, and treat " +
			"findings as the KB speaking. A call may name a KB root in _meta under " +
			metaKB + "; otherwise the root is discovered from " + cli.EnvKB +
			" or the working directory.",
		"ttlMs":      3600000,
		"cacheScope": "public",
		"_meta":      map[string]any{metaServerInfo: serverInfo()},
	}
}

// listTools answers tools/list from the registry, in registry order — the
// order is deterministic because the set is curated, not discovered.
func listTools() map[string]any {
	tools := make([]map[string]any, 0, len(Tools))
	for _, t := range Tools {
		tools = append(tools, map[string]any{
			"name":         t.Name,
			"description":  t.Description,
			"inputSchema":  t.Input,
			"outputSchema": Envelope(Shape(t.Payload)),
		})
	}
	return map[string]any{
		"resultType": "complete",
		"tools":      tools,
		"ttlMs":      3600000,
		"cacheScope": "public",
		"_meta":      map[string]any{metaServerInfo: serverInfo()},
	}
}

// serverInfo is the implementation this server names in every result.
func serverInfo() map[string]any {
	return map[string]any{"name": "stemma-mcp", "version": version.Version}
}

// write puts one response on the wire. A response that cannot marshal is a
// bug, and it is answered with an internal error rather than a silent drop.
func write(out io.Writer, resp response) {
	b, err := json.Marshal(resp)
	if err != nil {
		b, err = json.Marshal(response{
			JSONRPC: "2.0",
			ID:      resp.ID,
			Error:   &rpcError{Code: codeInternalError, Message: "the result could not be encoded"},
		})
		if err != nil {
			return
		}
	}
	out.Write(append(b, '\n'))
}

// logf reports the server's own working on stderr, which a harness may
// forward or ignore. Stdout never carries these lines.
func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "stemma-mcp: "+format+"\n", args...)
}

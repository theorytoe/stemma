// Command stemma-mcp serves a knowledge base to agents as native tools on
// the Model Context Protocol, over stdin and stdout. It reads newline-
// delimited JSON-RPC until its input ends, answers each request on stdout,
// and logs its own working on stderr, which a harness may capture or ignore.
//
// The tools are the curated set internal/mcp advertises; every result is the
// matching stemma verb's --json envelope. A call may name the KB it wants in
// _meta; the --kb flag sets the one this server answers with when a call
// names none, and with neither, the KB is found the way the CLI finds one —
// STEMMA_KB, then a walk up from the working directory (D41).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/theorytoe/stemma/internal/mcp"
)

func main() {
	os.Exit(run())
}

// run keeps main free of logic. The exit codes are the CLI's: 0 when the
// session ended cleanly, 2 when the server itself failed. Findings are the
// tools' business and travel inside their results, never as an exit code.
func run() int {
	kb := flag.String("kb", "", "the KB root; when not given, STEMMA_KB, else discovered by walking up")
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "stemma-mcp: takes no arguments, got %q\n", flag.Arg(0))
		return 2
	}

	server := &mcp.Server{
		In:       os.Stdin,
		Out:      os.Stdout,
		Log:      os.Stderr,
		Handlers: mcp.ToolHandlers(),
		Root:     *kb,
	}
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "stemma-mcp: %v\n", err)
		return 2
	}
	return 0
}

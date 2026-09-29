package kb

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The core must not reach for the extraction package.
//
// Python is a leaf (`P3`, `D6`): Stemma has to build, test and run with no
// interpreter anywhere, and only the commands that read a document may depend on
// one. The compiler enforces this today, because extraction imports this package
// and a cycle would not build — this test states the invariant, so that it
// survives a refactor which breaks the cycle without noticing what the cycle was
// holding up.
func TestTheCoreDoesNotImportExtraction(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no Go files here, so this test would pass by checking nothing")
	}

	for _, name := range files {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, imported := range file.Imports {
			if strings.Contains(imported.Path.Value, "/internal/extract") {
				t.Errorf("%s imports %s, and the core cannot depend on extraction",
					name, imported.Path.Value)
			}
		}
	}
}

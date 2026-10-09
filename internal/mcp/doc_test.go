package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratedDocsAreFresh regenerates both artifacts in memory and holds
// them to what `make mcp-docs` last wrote, so a registry change without a
// regeneration fails the suite instead of quietly shipping stale
// documentation. It skips on a tree that has never generated them, the way
// the binary test skips without a build.
func TestGeneratedDocsAreFresh(t *testing.T) {
	jsonPath := filepath.Join("..", "..", "docs", "mcp-tools.json")
	referencePath := filepath.Join("..", "..", "docs", "mcp-tools.md")
	if _, err := os.Stat(referencePath); os.IsNotExist(err) {
		t.Skip("docs/mcp-tools.md is not generated; run make mcp-docs first")
	}

	want, err := JSONBytes()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("docs/mcp-tools.json is stale; run make mcp-docs")
	}

	wantRef, err := ReferenceBytes()
	if err != nil {
		t.Fatal(err)
	}
	gotRef, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRef) != string(wantRef) {
		t.Errorf("docs/mcp-tools.md is stale; run make mcp-docs")
	}
}

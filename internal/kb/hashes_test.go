package kb

import (
	"os"
	"path/filepath"
	"testing"
)

// Hashes and Load must agree on which pages exist: if they did not, an index
// built from one would look stale to the other on every run.
func TestHashesMatchThePagesLoadReads(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"stemma.toml":        "ignore = [\"pages/attic/**\"]\n",
		"pages/index.md":     "---\ntitle: Index\ntype: index\n---\n",
		"pages/a.md":         "---\ntitle: A\ntype: concept\n---\nbody\n",
		"pages/attic/old.md": "---\ntitle: Old\ntype: note\n---\n",
		"pages/notes.txt":    "not a page\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	hashes, err := Hashes(root)
	if err != nil {
		t.Fatalf("Hashes: %v", err)
	}
	k, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(hashes) != k.Graph.Len() {
		t.Errorf("Hashes has %d pages, Load has %d", len(hashes), k.Graph.Len())
	}
	for _, p := range k.Graph.Paths() {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		if hashes[p] != HashOf(raw) {
			t.Errorf("hash of %s = %q, want %q", p, hashes[p], HashOf(raw))
		}
	}
	if _, ok := hashes["pages/attic/old.md"]; ok {
		t.Error("an ignored page was hashed")
	}
	if _, ok := hashes["pages/notes.txt"]; ok {
		t.Error("a non-page was hashed")
	}
}

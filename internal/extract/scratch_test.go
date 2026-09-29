package extract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/kb"
)

func TestTextPathIsAFunctionOfThePointer(t *testing.T) {
	root := "/kb"
	first := TextPath(root, "https://example.com/a")

	if first != TextPath(root, "https://example.com/a") {
		t.Error("the same pointer gave two paths")
	}
	if first == TextPath(root, "https://example.com/b") {
		t.Error("two pointers gave one path")
	}
	if dir := filepath.Dir(first); dir != ScratchDir(root) {
		t.Errorf("directory = %q, want %q", dir, ScratchDir(root))
	}
	if !strings.HasSuffix(first, ".txt") {
		t.Errorf("path = %q, want a .txt", first)
	}
}

// The slug is lossy on purpose; the digest beside it is what keeps two sources
// apart, and this is the property that has to hold because of it.
func TestTextPathSeparatesFilesWithOneName(t *testing.T) {
	one := filepath.Base(TextPath("/kb", "/first/smith2020.pdf"))
	two := filepath.Base(TextPath("/kb", "/second/smith2020.pdf"))

	if one == two {
		t.Errorf("two files with one name share the path %q", one)
	}
}

func TestTextPathSlugsAreReadable(t *testing.T) {
	for _, tc := range []struct{ pointer, want string }{
		{"https://go.dev/blog/go1.22", "go.dev-blog-go1.22"},
		{"/home/me/papers/Smith 2020.pdf", "smith-2020.pdf"},
		{"http://example.com/a/b/", "example.com-a-b"},
	} {
		name := filepath.Base(TextPath("/kb", tc.pointer))
		if !strings.HasPrefix(name, tc.want+"-") {
			t.Errorf("TextPath(%q) = %q, want it to start with %q", tc.pointer, name, tc.want)
		}
	}
}

func TestClearEmptiesOnlyTheScratchArea(t *testing.T) {
	root := t.TempDir()

	// Something else in the generated directory, which is not fetch's to remove.
	keep := filepath.Join(root, kb.GeneratedDir, "shim", "extract.py")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("the script"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := ScratchDir(root)
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", filepath.Join("nested", "b.txt")} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("text"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := Clear(root)
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if files != 2 {
		t.Errorf("cleared %d files, want 2", files)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("the scratch area is still there")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("Clear removed something that was not scratch: %v", err)
	}
}

func TestClearOnAScratchAreaThatWasNeverThere(t *testing.T) {
	files, err := Clear(t.TempDir())
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if files != 0 {
		t.Errorf("cleared %d files, want 0", files)
	}
}

func TestClearRefusesAnEmptyRoot(t *testing.T) {
	if _, err := Clear(""); err == nil {
		t.Error("Clear with no root succeeded")
	}
}

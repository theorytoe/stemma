package extract

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestScriptForPrefersTheKBsCopy(t *testing.T) {
	root := t.TempDir()

	got, err := ScriptFor(root)
	if err != nil {
		t.Fatalf("ScriptFor: %v", err)
	}
	defer got.Cleanup()

	if got.Temporary {
		t.Error("a copy was made for the occasion even though the KB could take one")
	}
	if got.Path != ScriptPath(root) {
		t.Errorf("Path = %q, want %q", got.Path, ScriptPath(root))
	}
	if ConditionOf(got.Path) != Current {
		t.Errorf("the script at %q is not the one this binary carries", got.Path)
	}
}

func TestScriptForMakesATemporaryCopyWithNoKB(t *testing.T) {
	got, err := ScriptFor("")
	if err != nil {
		t.Fatalf("ScriptFor: %v", err)
	}

	if !got.Temporary {
		t.Error("there is no KB and the copy is not marked temporary")
	}
	have, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatalf("the temporary copy was not written: %v", err)
	}
	if !bytes.Equal(have, script) {
		t.Error("the temporary copy is not the script this binary carries")
	}

	got.Cleanup()
	if _, err := os.Stat(got.Path); !os.IsNotExist(err) {
		t.Error("Cleanup left the temporary copy behind")
	}
}

func TestScriptForMakesATemporaryCopyWhenTheKBCannotBeWritten(t *testing.T) {
	// A root that is a file rather than a directory cannot hold a script, which is
	// the same situation as a read-only KB from this side.
	root := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ScriptFor(root)
	if err != nil {
		t.Fatalf("ScriptFor: %v", err)
	}
	defer got.Cleanup()

	if !got.Temporary {
		t.Errorf("Path = %q, want a temporary copy when the KB cannot take one", got.Path)
	}
}

func TestScriptForHonoursTheOverride(t *testing.T) {
	t.Setenv(EnvShim, "/elsewhere/extract.py")

	got, err := ScriptFor(t.TempDir())
	if err != nil {
		t.Fatalf("ScriptFor: %v", err)
	}
	defer got.Cleanup()

	if got.Path != "/elsewhere/extract.py" {
		t.Errorf("Path = %q, want the override", got.Path)
	}
	if got.Temporary {
		t.Error("an override is not a copy made for the occasion")
	}
}

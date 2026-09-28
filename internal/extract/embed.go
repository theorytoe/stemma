package extract

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"

	"github.com/theorytoe/stemma/internal/kb"
)

// EnvShim points the tool at a script other than the one it carries. It exists
// for working on an extractor without rebuilding the binary each time; `env`
// reports when it is in force.
const EnvShim = "STEMMA_SHIM"

// script is the shim as compiled into this binary. The bytes that print the JSON
// object are therefore the bytes compiled into the version that parses it, which
// is what stops the two drifting apart.
//
//go:embed extract.py
var script []byte

// ScriptPath is where the script lives for a KB. It is inside the generated
// directory because it is a derived artifact: it is written by the tool, never
// committed, and rebuilt whenever it does not match the binary.
func ScriptPath(root string) string {
	return filepath.Join(root, kb.GeneratedDir, "shim", "extract.py")
}

// Path is the script this KB would run: whatever STEMMA_SHIM names, and the KB's
// own copy when nothing overrides it.
func Path(root string) string {
	if override := os.Getenv(EnvShim); override != "" {
		return override
	}
	return ScriptPath(root)
}

// Materialize makes sure the KB holds this binary's copy of the script, and
// returns where it is.
//
// The comparison is what makes three different situations one case: a cleared
// `.stemma/`, an upgrade that changed the script, and a KB received by clone all
// end in the same place, which is a copy that matches the binary asking for it.
func Materialize(root string) (string, error) {
	path := ScriptPath(root)
	if ConditionOf(path) == Current {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := kb.WriteFileAtomic(path, script); err != nil {
		return "", err
	}
	return path, nil
}

// Condition is how the copy on disk stands against the one in the binary.
type Condition int

const (
	// Absent means there is no copy to check.
	Absent Condition = iota
	// Current means the copy is the one this binary carries.
	Current
	// Stale means a copy is there and it is not the one this binary carries.
	Stale
)

func (c Condition) String() string {
	switch c {
	case Current:
		return "current"
	case Stale:
		return "stale"
	default:
		return "absent"
	}
}

// ConditionOf reports how the copy at path stands. It only ever reads, so a
// diagnostic command can ask the question without creating anything.
func ConditionOf(path string) Condition {
	have, err := os.ReadFile(path)
	if err == nil {
		if bytes.Equal(have, script) {
			return Current
		}
		return Stale
	}
	if os.IsNotExist(err) {
		return Absent
	}
	// Something is there and cannot be read. That is not the same as its being
	// absent, but it is equally not ours to vouch for.
	return Stale
}

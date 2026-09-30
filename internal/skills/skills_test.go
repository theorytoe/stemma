package skills_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theorytoe/stemma/internal/skills"
)

// The suite is the deliverable of the skills delegate, so the gate is that it
// conforms to the open standard a harness will judge it by.
func TestSkillSuiteConformsToTheStandard(t *testing.T) {
	problems, err := skills.Validate(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Errorf("%s", p)
	}
}

// One umbrella and five workflows is the decomposition the design settled on.
// A stray sixth directory or a renamed skill should be a deliberate change, not
// an accident the gate waves through.
func TestTheSuiteIsTheSixExpectedSkills(t *testing.T) {
	suite, err := skills.Load(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"stemma":          true,
		"stemma-research": true,
		"stemma-author":   true,
		"stemma-maintain": true,
		"stemma-query":    true,
		"stemma-publish":  true,
	}
	if len(suite) != len(want) {
		t.Errorf("found %d skills, want %d", len(suite), len(want))
	}
	for _, s := range suite {
		if !want[s.Dir] {
			t.Errorf("unexpected skill %q", s.Dir)
		}
		delete(want, s.Dir)
	}
	for name := range want {
		t.Errorf("missing skill %q", name)
	}
}

// A gate that cannot fail is decoration. This drives the validator with skills
// that break the rules and confirms each one is reported.
func TestTheValidatorCatchesABrokenSkill(t *testing.T) {
	root := t.TempDir()
	body := strings.Repeat("filler\n", skills.MaxBodyLines+1)

	// A name with the right characters that does not match its directory, an
	// empty description, an over-long compatibility field, and an over-long body.
	writeSkill(t, root, "broken", "---\nname: other-name\ndescription: \"\"\ncompatibility: "+
		strings.Repeat("x", skills.MaxCompatibility+1)+"\n---\n"+body)

	// A name that is not lowercase letters, digits, and single hyphens.
	writeSkill(t, root, "also-bad", "---\nname: Bad_Name\ndescription: fine\n---\nshort\n")

	problems, err := skills.Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, p := range problems {
		joined += p.String() + "\n"
	}
	for _, want := range []string{"does not match", "description", "compatibility", "lines", "not lowercase"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the validator missed %q:\n%s", want, joined)
		}
	}
}

func writeSkill(t *testing.T, root, dir, content string) {
	t.Helper()
	path := filepath.Join(root, "skills", dir)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// repoRoot walks up from the test's package directory to the module root, so
// the gate reads the real skills/ tree wherever the test is run from.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test")
		}
		dir = parent
	}
}

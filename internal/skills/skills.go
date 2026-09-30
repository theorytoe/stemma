// Package skills validates the agent-facing skill suite under skills/ against
// the open Agent Skills standard.
//
// It checks the rules the standard fixes -- the name, the description, and the
// size of a SKILL.md -- because those are what a harness reads before it decides
// whether to load a skill. It does not check whether the instructions are any
// good; that is a judgement recorded by the delegate, not asserted by a test.
package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// MaxName is the standard's limit on the name field.
	MaxName = 64
	// MaxDescription is the standard's limit on the description field.
	MaxDescription = 1024
	// MaxCompatibility is the standard's limit on an optional compatibility field.
	MaxCompatibility = 500
	// MaxBodyLines is the size the standard recommends a SKILL.md stays under,
	// so that activating a skill does not flood the context window.
	MaxBodyLines = 500
)

// namePattern is the standard's rule for a skill name: lowercase letters,
// digits, and single hyphens, with no leading, trailing, or doubled hyphen.
var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Skill is one parsed SKILL.md.
type Skill struct {
	// Dir is the directory the skill lives in, which the name must match.
	Dir string
	// Name is the frontmatter name field.
	Name string
	// Description is the frontmatter description field.
	Description string
	// Compatibility is the optional frontmatter compatibility field.
	Compatibility string
	// BodyLines is how many lines follow the frontmatter.
	BodyLines int
}

// Problem is one failure against the standard.
type Problem struct {
	Skill  string
	Detail string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s: %s", p.Skill, p.Detail)
}

// Load reads every SKILL.md directly under root/skills/. A directory without a
// SKILL.md is skipped rather than treated as a broken skill, because skill
// directories may hold resources beside the instruction file.
func Load(root string) ([]Skill, error) {
	dir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var found []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		s, err := parse(e.Name(), data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		found = append(found, s)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Dir < found[j].Dir })
	return found, nil
}

// Validate returns every problem the skill suite has, and nil when it conforms.
func Validate(root string) ([]Problem, error) {
	suite, err := Load(root)
	if err != nil {
		return nil, err
	}
	if len(suite) == 0 {
		return []Problem{{Skill: "skills/", Detail: "no SKILL.md found"}}, nil
	}
	var problems []Problem
	for _, s := range suite {
		problems = append(problems, validate(s)...)
	}
	return problems, nil
}

func validate(s Skill) []Problem {
	var problems []Problem
	add := func(format string, args ...any) {
		problems = append(problems, Problem{Skill: s.Dir, Detail: fmt.Sprintf(format, args...)})
	}

	switch {
	case s.Name == "":
		add("the name field is required")
	case len(s.Name) > MaxName:
		add("name is %d characters, over the %d limit", len(s.Name), MaxName)
	case !namePattern.MatchString(s.Name):
		add("name %q is not lowercase letters, digits, and single hyphens", s.Name)
	case s.Name != s.Dir:
		add("name %q does not match the directory %q", s.Name, s.Dir)
	}

	switch {
	case strings.TrimSpace(s.Description) == "":
		add("the description field is required")
	case len(s.Description) > MaxDescription:
		add("description is %d characters, over the %d limit", len(s.Description), MaxDescription)
	}

	if len(s.Compatibility) > MaxCompatibility {
		add("compatibility is %d characters, over the %d limit", len(s.Compatibility), MaxCompatibility)
	}
	if s.BodyLines > MaxBodyLines {
		add("SKILL.md is %d lines, over the %d-line recommendation", s.BodyLines, MaxBodyLines)
	}
	return problems
}

// parse splits the frontmatter from the body and reads the fields that matter.
func parse(dir string, data []byte) (Skill, error) {
	fm, body, err := splitFrontmatter(string(data))
	if err != nil {
		return Skill{}, err
	}
	var meta struct {
		Name          string `yaml:"name"`
		Description   string `yaml:"description"`
		Compatibility string `yaml:"compatibility"`
	}
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return Skill{}, fmt.Errorf("frontmatter: %w", err)
	}
	return Skill{
		Dir:           dir,
		Name:          meta.Name,
		Description:   meta.Description,
		Compatibility: meta.Compatibility,
		BodyLines:     bodyLines(body),
	}, nil
}

// splitFrontmatter returns the YAML between the opening and closing --- lines,
// and everything after the closing line.
func splitFrontmatter(s string) (fm, body string, err error) {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return "", "", errors.New("SKILL.md does not start with --- frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", errors.New("frontmatter is not closed with ---")
}

// bodyLines counts the body without a trailing blank line inflating the count.
func bodyLines(body string) int {
	return len(strings.Split(strings.TrimRight(body, "\n"), "\n"))
}

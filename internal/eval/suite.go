// Package eval runs per-skill evals.json cases through a build.AgentAdapter and
// grades each with an LLM judge, so skill confidence can be measured rather than
// asserted. One eval = one adapter spawn in a fresh fixture directory.
package eval

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Case is one eval from a skill's evals.json. Expectations is optional finer-
// grained criteria some suites carry alongside the prose ExpectedOutput; the
// judge is handed both.
type Case struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Prompt         string     `json:"prompt"`
	ExpectedOutput string     `json:"expected_output"`
	Files          []FileSeed `json:"files"`
	Expectations   []string   `json:"expectations,omitempty"`
}

// FileSeed is a fixture file written into the eval's working directory before
// the agent runs. Both current suites declare empty file sets, but the runner
// honours seeds so a case can stage brownfield input.
type FileSeed struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Suite is the parsed evals.json for one skill.
type Suite struct {
	SkillName string `json:"skill_name"`
	Evals     []Case `json:"evals"`
}

// LoadSuite reads <skill>/evals/evals.json from the embedded skills bundle.
// skillsFS is anvil/skills.FS — the single source of truth the installed
// binary materialises from, so evals travel with the skill they grade.
func LoadSuite(skillsFS fs.FS, skill string) (Suite, error) {
	if strings.ContainsAny(skill, `/\`) || strings.Contains(skill, "..") {
		return Suite{}, fmt.Errorf("invalid skill name %q", skill)
	}
	b, err := fs.ReadFile(skillsFS, path.Join(skill, "evals", "evals.json"))
	if err != nil {
		return Suite{}, fmt.Errorf("skill %q has no evals.json: %w", skill, err)
	}
	var s Suite
	if err := json.Unmarshal(b, &s); err != nil {
		return Suite{}, fmt.Errorf("parsing %s evals.json: %w", skill, err)
	}
	if len(s.Evals) == 0 {
		return Suite{}, fmt.Errorf("skill %q evals.json declares no evals", skill)
	}
	return s, nil
}

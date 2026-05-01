package core

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Plan is an executable Anvil plan artifact (80-plans/<id>-<slug>.md).
type Plan struct {
	Path         string
	ID           string
	Slug         string
	Title        string
	Status       string
	PlanVersion  int
	Issue        string
	Tasks        []Task
	Verification PlanVerification
	Raw          *Artifact
}

// Task is one DAG node inside a Plan.
type Task struct {
	ID              string
	Title           string
	Kind            string
	Model           string
	Effort          string
	Files           []string
	DependsOn       []string
	SkillsToLoad    []string
	Verify          string
	SuccessCriteria []string
	Body            string
}

// PlanVerification holds plan-level pre/post commands.
type PlanVerification struct {
	PreBuild  []string
	PostBuild []string
}

var taskHeaderRe = regexp.MustCompile(`(?m)^## Task:\s*(T[0-9]+)\s*$`)

// LoadPlan parses a plan file at path. It does NOT run schema or DAG validation —
// callers compose with schema.Validate and ValidatePlan as needed.
func LoadPlan(path string) (*Plan, error) {
	a, err := LoadArtifact(path)
	if err != nil {
		return nil, fmt.Errorf("loading plan: %w", err)
	}
	fmBytes, err := yaml.Marshal(a.FrontMatter)
	if err != nil {
		return nil, fmt.Errorf("re-marshal frontmatter: %w", err)
	}
	var typed struct {
		ID          string `yaml:"id"`
		Slug        string `yaml:"slug"`
		Title       string `yaml:"title"`
		Status      string `yaml:"status"`
		PlanVersion int    `yaml:"plan_version"`
		Issue       string `yaml:"issue"`
		Tasks       []struct {
			ID              string   `yaml:"id"`
			Title           string   `yaml:"title"`
			Kind            string   `yaml:"kind"`
			Model           string   `yaml:"model"`
			Effort          string   `yaml:"effort"`
			Files           []string `yaml:"files"`
			DependsOn       []string `yaml:"depends_on"`
			SkillsToLoad    []string `yaml:"skills_to_load"`
			Verify          string   `yaml:"verify"`
			SuccessCriteria []string `yaml:"success_criteria"`
		} `yaml:"tasks"`
		Verification struct {
			PreBuild  []string `yaml:"pre_build"`
			PostBuild []string `yaml:"post_build"`
		} `yaml:"verification"`
	}
	if err := yaml.Unmarshal(fmBytes, &typed); err != nil {
		return nil, fmt.Errorf("decode plan frontmatter: %w", err)
	}

	p := &Plan{
		Path: path, Raw: a,
		ID: typed.ID, Slug: typed.Slug, Title: typed.Title,
		Status: typed.Status, PlanVersion: typed.PlanVersion,
		Issue: typed.Issue,
		Verification: PlanVerification{
			PreBuild:  typed.Verification.PreBuild,
			PostBuild: typed.Verification.PostBuild,
		},
	}
	bodies := sliceTaskBodies(a.Body)
	for _, t := range typed.Tasks {
		p.Tasks = append(p.Tasks, Task{
			ID: t.ID, Title: t.Title, Kind: t.Kind,
			Model: t.Model, Effort: t.Effort,
			Files: t.Files, DependsOn: t.DependsOn,
			SkillsToLoad: t.SkillsToLoad, Verify: t.Verify,
			SuccessCriteria: t.SuccessCriteria,
			Body:            bodies[t.ID],
		})
	}
	return p, nil
}

// sliceTaskBodies walks body and returns task-id -> section text. A task
// section runs from one "## Task: T<n>" header until the next "## " header
// (Task or otherwise) or EOF.
func sliceTaskBodies(body string) map[string]string {
	out := map[string]string{}
	matches := taskHeaderRe.FindAllStringSubmatchIndex(body, -1)
	for i, m := range matches {
		id := body[m[2]:m[3]]
		start := m[1]
		end := len(body)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		} else {
			if next := strings.Index(body[start:], "\n## "); next >= 0 {
				end = start + next
			}
		}
		out[id] = strings.TrimSpace(body[start:end])
	}
	return out
}

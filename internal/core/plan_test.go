package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const planFixture = `---
type: plan
id: ANV-142
slug: streaming-token-counter
title: "Stream-aware token counter"
created: 2026-04-30
updated: 2026-04-30
status: draft
plan_version: 1
issue: "[[issue.ANV-142]]"
tasks:
  - id: T1
    title: "Define TokenUsage type"
    kind: tdd
    files: ["a.go", "a_test.go"]
    depends_on: []
    verify: "go test ./..."
  - id: T2
    title: "Streaming reader"
    kind: tdd
    files: ["b.go", "b_test.go"]
    depends_on: [T1]
    verify: "go test ./..."
---

## Goal

Stream-aware counter.

## Task: T1

Define the TokenUsage type in a.go. Write the RED test in a_test.go first to
assert that zero-value fields are sane and that accumulation arithmetic is
correct. Run "go test ./..." to confirm RED, implement the type, then run again
to confirm GREEN. Commit once verify passes.

## Task: T2

Implement the streaming reader in b.go. Write the RED test in b_test.go first
to assert that the reader correctly accumulates tokens across multiple chunks
and returns an error on malformed input. Run "go test ./..." to confirm RED,
implement the reader, then run again to confirm GREEN. Commit once verify passes.
`

func writePlanFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ANV-142-streaming-token-counter.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadPlan_ParsesFrontmatterAndTaskBodies(t *testing.T) {
	p, err := LoadPlan(writePlanFile(t, planFixture))
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}
	if p.ID != "ANV-142" {
		t.Errorf("ID = %q", p.ID)
	}
	if len(p.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(p.Tasks))
	}
	if p.Tasks[0].ID != "T1" || p.Tasks[1].ID != "T2" {
		t.Errorf("task IDs = %q,%q", p.Tasks[0].ID, p.Tasks[1].ID)
	}
	if p.Tasks[0].Verify != "go test ./..." {
		t.Errorf("T1 verify = %q", p.Tasks[0].Verify)
	}
	if got := p.Tasks[1].DependsOn; len(got) != 1 || got[0] != "T1" {
		t.Errorf("T2 depends_on = %v", got)
	}
	if p.Tasks[0].Body == "" || p.Tasks[1].Body == "" {
		t.Errorf("task bodies empty: %q | %q", p.Tasks[0].Body, p.Tasks[1].Body)
	}
}

func TestLoadPlan_AcceptsTrailingTitleOnTaskHeading(t *testing.T) {
	body := `---
type: plan
id: anvil.x
slug: x
title: x
created: 2026-05-07
updated: 2026-05-07
status: draft
plan_version: 1
issue: "[[issue.anvil.x]]"
tasks:
  - id: T1
    title: x
    kind: tdd
    files: [a.go]
    depends_on: []
    verify: "go test ./..."
  - id: T2
    title: y
    kind: tdd
    files: [b.go]
    depends_on: [T1]
    verify: "go test ./..."
  - id: T3
    title: z
    kind: tdd
    files: [c.go]
    depends_on: [T2]
    verify: "go test ./..."
---

## Task: T1 — Em-dash title

` + strings.Repeat("body. ", 60) + `

## Task: T2: Colon title

` + strings.Repeat("body. ", 60) + `

## Task: T3 - Hyphen title

` + strings.Repeat("body. ", 60) + `
`
	p, err := LoadPlan(writePlanFile(t, body))
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}
	for _, id := range []string{"T1", "T2", "T3"} {
		var task *Task
		for i := range p.Tasks {
			if p.Tasks[i].ID == id {
				task = &p.Tasks[i]
				break
			}
		}
		if task == nil || task.Body == "" {
			t.Errorf("task %s body missing — heading parser dropped trailing title", id)
		}
	}
}

func TestLoadPlan_ParsesModelEffort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	body := `---
type: plan
id: anvil.x
slug: x
title: x
created: 2026-04-30
updated: 2026-04-30
status: draft
plan_version: 1
issue: "[[issue.anvil.x]]"
tasks:
  - id: T1
    title: x
    kind: tdd
    model: claude-opus-4-7
    effort: high
    files: [a.go]
    depends_on: []
    verify: "go test ./..."
---

## Task: T1

` + strings.Repeat("body. ", 60) + `
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Tasks[0].Model != "claude-opus-4-7" || p.Tasks[0].Effort != "high" {
		t.Errorf("got Model=%q Effort=%q", p.Tasks[0].Model, p.Tasks[0].Effort)
	}
}

func TestLoadPlan_ParsesContextToLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	body := `---
type: plan
id: anvil.x
slug: x
title: x
created: 2026-05-07
updated: 2026-05-07
status: draft
plan_version: 1
issue: "[[issue.anvil.x]]"
tasks:
  - id: T1
    title: x
    kind: tdd
    files: [a.go]
    depends_on: []
    skills_to_load: [tdd, code-review]
    context_to_load:
      - docs/code-design.md
      - docs/go-conventions.md
    verify: "go test ./..."
---

## Task: T1

` + strings.Repeat("body. ", 60) + `
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPlan(path)
	if err != nil {
		t.Fatal(err)
	}
	gotSkills := p.Tasks[0].SkillsToLoad
	wantSkills := []string{"tdd", "code-review"}
	if len(gotSkills) != len(wantSkills) || gotSkills[0] != wantSkills[0] || gotSkills[1] != wantSkills[1] {
		t.Errorf("SkillsToLoad = %v, want %v", gotSkills, wantSkills)
	}
	gotCtx := p.Tasks[0].ContextToLoad
	wantCtx := []string{"docs/code-design.md", "docs/go-conventions.md"}
	if len(gotCtx) != len(wantCtx) || gotCtx[0] != wantCtx[0] || gotCtx[1] != wantCtx[1] {
		t.Errorf("ContextToLoad = %v, want %v", gotCtx, wantCtx)
	}
}

// TestPlanLoad_CorruptFrontmatterDoesNotLeakStdlib verifies that LoadArtifact
// (used by the facet walk and plan loader) wraps corrupt-YAML errors with
// ErrFrontmatterParse rather than leaking raw "strconv.ParseInt" or
// "yaml: line N: found character that cannot start any token" strings to
// callers. This is the acceptance test for the stdlib-error-leak sweep.
func TestPlanLoad_CorruptFrontmatterDoesNotLeakStdlib(t *testing.T) {
	// A file whose YAML frontmatter contains a leading backtick in an unquoted
	// value — the same class of failure documented in the issue body
	// ("found character that cannot start any token").
	corrupt := "---\ntype: issue\nid: test.corrupt\ntitle: test corrupt\ntags:\n  - `bad-token\n---\nBody\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.md")
	if err := os.WriteFile(path, []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadArtifact(path)
	if err == nil {
		t.Fatal("expected error for corrupt YAML, got nil")
	}

	// Must carry ErrFrontmatterParse so callers (e.g. facet walk) can tolerate it.
	if !errors.Is(err, ErrFrontmatterParse) {
		t.Errorf("error does not wrap ErrFrontmatterParse: %v", err)
	}

	// Must NOT leak raw stdlib strings — agents should never see these.
	rawLeaks := []string{
		"strconv.ParseInt",
		"strconv.Atoi",
	}
	msg := err.Error()
	for _, leak := range rawLeaks {
		if strings.Contains(msg, leak) {
			t.Errorf("error leaks raw stdlib string %q: %s", leak, msg)
		}
	}

	// The yaml error itself may still appear in the enriched message, but it
	// must be accompanied by an actionable hint — not the bare "yaml: line N:"
	// format. Check that enrichYAMLError fired (hint or field context present).
	if strings.Contains(msg, "yaml: line") && !strings.Contains(msg, "hint:") && !strings.Contains(msg, "near line") {
		t.Errorf("yaml error appears unenriched (no hint/near-line context): %s", msg)
	}
}

func TestLoadPlan_DefaultsEffortToMedium(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	// effort: omitted; loader should default to "medium".
	body := `---
type: plan
id: anvil.x
slug: x
title: x
created: 2026-05-07
updated: 2026-05-07
status: draft
plan_version: 1
issue: "[[issue.anvil.x]]"
tasks:
  - id: T1
    title: x
    kind: tdd
    files: [a.go]
    depends_on: []
    verify: "go test ./..."
  - id: T2
    title: y
    kind: tdd
    effort: high
    files: [b.go]
    depends_on: []
    verify: "go test ./..."
---

## Task: T1

` + strings.Repeat("body. ", 60) + `

## Task: T2

` + strings.Repeat("body. ", 60) + `
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Tasks[0].Effort; got != "medium" {
		t.Errorf("T1 Effort = %q, want %q (default)", got, "medium")
	}
	if got := p.Tasks[1].Effort; got != "high" {
		t.Errorf("T2 Effort = %q, want %q (explicit)", got, "high")
	}
}

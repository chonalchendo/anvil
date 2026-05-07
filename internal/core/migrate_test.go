package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateVault_RewritesMilestone(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	old := `---
type: milestone
title: "M3"
created: 2026-04-29
updated: 2026-04-29
status: planned
target_date: 2026-05-15
horizon: month
project: anvil
plans: ["[[plan.anvil.x]]"]
issues: ["[[issue.anvil.y]]"]
objectives: ["A", "B"]
risks: ["R1"]
---

## Body
`
	path := filepath.Join(v.Root, "85-milestones", "anvil.m3.md")
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateVault(v); err != nil {
		t.Fatal(err)
	}

	a, err := LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"target_date", "horizon", "plans", "issues", "objectives", "risks"} {
		if _, ok := a.FrontMatter[gone]; ok {
			t.Errorf("expected %q to be cut", gone)
		}
	}
	for _, want := range []string{"## Objectives", "## Risks"} {
		if !strings.Contains(a.Body, want) {
			t.Errorf("expected body to contain %q", want)
		}
	}
}

func TestMigrateVault_Idempotent(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	old := `---
type: milestone
title: "M3"
created: 2026-04-29
updated: 2026-04-29
status: planned
target_date: 2026-05-15
project: anvil
objectives: ["A"]
---

## Body
`
	path := filepath.Join(v.Root, "85-milestones", "anvil.m3.md")
	os.WriteFile(path, []byte(old), 0o644)

	if err := MigrateVault(v); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if err := MigrateVault(v); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Errorf("not idempotent — second pass changed content")
	}
}

func TestMigrateVault_SplitsPlanSkillsToLoad(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	old := `---
type: plan
id: anvil.demo
slug: demo
title: demo
description: demo
created: 2026-05-07
updated: 2026-05-07
status: draft
plan_version: 1
issue: "[[issue.anvil.demo]]"
tags: [domain/dev-tools]
tasks:
  - id: T1
    title: x
    kind: tdd
    files: [a.go]
    depends_on: []
    skills_to_load: [tdd, docs/code-design.md, code-review, docs/go-conventions.md, README.md]
    verify: "go test ./..."
  - id: T2
    title: y
    kind: tdd
    files: [b.go]
    depends_on: []
    skills_to_load: [brainstorming]
    verify: "go test ./..."
---

## Task: T1

body
`
	path := filepath.Join(v.Root, "80-plans", "anvil.demo.md")
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateVault(v); err != nil {
		t.Fatalf("MigrateVault: %v", err)
	}

	a, err := LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	tasks, ok := a.FrontMatter["tasks"].([]any)
	if !ok {
		t.Fatalf("tasks: got %T", a.FrontMatter["tasks"])
	}

	t1 := tasks[0].(map[string]any)
	gotSkills := toStringSlice(t, t1["skills_to_load"])
	wantSkills := []string{"tdd", "code-review"}
	if !equalStrings(gotSkills, wantSkills) {
		t.Errorf("T1 skills_to_load = %v, want %v", gotSkills, wantSkills)
	}
	gotCtx := toStringSlice(t, t1["context_to_load"])
	wantCtx := []string{"docs/code-design.md", "docs/go-conventions.md", "README.md"}
	if !equalStrings(gotCtx, wantCtx) {
		t.Errorf("T1 context_to_load = %v, want %v", gotCtx, wantCtx)
	}

	t2 := tasks[1].(map[string]any)
	gotSkills2 := toStringSlice(t, t2["skills_to_load"])
	if !equalStrings(gotSkills2, []string{"brainstorming"}) {
		t.Errorf("T2 skills_to_load = %v, want [brainstorming]", gotSkills2)
	}
	if _, present := t2["context_to_load"]; present {
		t.Errorf("T2 context_to_load should be absent (no path entries), got %v", t2["context_to_load"])
	}
}

func TestMigrateVault_SplitsPlanSkillsToLoad_Idempotent(t *testing.T) {
	v := &Vault{Root: t.TempDir()}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	old := `---
type: plan
id: anvil.demo
slug: demo
title: demo
description: demo
created: 2026-05-07
updated: 2026-05-07
status: draft
plan_version: 1
issue: "[[issue.anvil.demo]]"
tags: [domain/dev-tools]
tasks:
  - id: T1
    title: x
    kind: tdd
    files: [a.go]
    depends_on: []
    skills_to_load: [tdd, docs/code-design.md]
    verify: "go test ./..."
---

## Task: T1

body
`
	path := filepath.Join(v.Root, "80-plans", "anvil.demo.md")
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateVault(v); err != nil {
		t.Fatalf("first MigrateVault: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateVault(v); err != nil {
		t.Fatalf("second MigrateVault: %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("migration not idempotent:\nFIRST:\n%s\nSECOND:\n%s", first, second)
	}
}

// toStringSlice converts a yaml-decoded []any to []string, failing the test on
// any non-string element.
func toStringSlice(t *testing.T, v any) []string {
	t.Helper()
	if v == nil {
		return nil
	}
	raw, ok := v.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", v)
	}
	out := make([]string, 0, len(raw))
	for i, e := range raw {
		s, ok := e.(string)
		if !ok {
			t.Fatalf("element %d: expected string, got %T", i, e)
		}
		out = append(out, s)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

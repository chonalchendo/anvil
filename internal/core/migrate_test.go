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

package core

import (
	"os"
	"path/filepath"
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
milestone: "[[milestone.telemetry.m3]]"
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

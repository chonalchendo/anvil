package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func createDemoIssue(t *testing.T) {
	t.Helper()
	execCmd(t, "create", "issue",
		"--project", "demo",
		"--title", "foo",
		"--description", "foo desc",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
}

func TestTransitionHappyPathWritesFrontmatter(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	out := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if got["status"] != "transitioned" || got["from"] != "open" || got["to"] != "in-progress" || got["owner"] != "claude" {
		t.Fatalf("envelope: %v", got)
	}

	row, err := openIndex(t, vault).GetArtifact("demo.foo")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "in-progress" {
		t.Fatalf("index status: %q", row.Status)
	}
}

func TestTransitionIdempotentWhenAlreadyInState(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	out := execCmd(t, "transition", "issue", "demo.foo", "open", "--json")
	if !strings.Contains(out, `"already_in_state"`) {
		t.Fatalf("expected already_in_state, got %s", out)
	}
}

func TestTransitionIllegalReturnsErr(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected illegal_transition error; output: %s", out.String())
	}
	if !strings.Contains(out.String(), "illegal_transition") {
		t.Fatalf("expected error code in output: %s", out.String())
	}
}

func TestTransitionMissingRequiredFlag(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected transition_flag_required; output: %s", out.String())
	}
	if !strings.Contains(out.String(), "owner") {
		t.Fatalf("expected `owner` mentioned: %s", out.String())
	}
}

func TestTransitionReverseRequiresReason(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "open"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected reason required; output: %s", out.String())
	}
}

func TestTransitionOwnerSurvivesValidate(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate failed after transition --owner: %v\noutput: %s", err, out.String())
	}
}

func TestTransitionPlanToLocked_RejectsPlaceholderPlan(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv("HOME", t.TempDir())
	execCmd(t, "init", vault)
	repo := setupGitRepo(t, "git@github.com:acme/demo.git")
	t.Chdir(repo)

	execCmd(t, "create", "issue", "--title", "I", "--description", "d",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "plan", "--title", "P", "--description", "d",
		"--issue", "[[issue.demo.i]]", "--tags", "domain/dev-tools",
		"--allow-new-facet=domain")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "plan", "demo.p", "locked"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected plan validator to reject placeholder; output: %s", out.String())
	}
	msg := err.Error() + out.String()
	if !strings.Contains(msg, "no-op") {
		t.Errorf("expected no-op-verify diagnostic, got: %s", msg)
	}

	// File status should still be draft (transition aborted).
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "demo.p.md"))
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "draft" {
		t.Errorf("status = %v, want draft", a.FrontMatter["status"])
	}
}

func TestTransitionPlanToLocked_AcceptsRealVerify(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv("HOME", t.TempDir())
	execCmd(t, "init", vault)
	repo := setupGitRepo(t, "git@github.com:acme/demo.git")
	t.Chdir(repo)

	execCmd(t, "create", "issue", "--title", "I", "--description", "d",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "plan", "--title", "P", "--description", "d",
		"--issue", "[[issue.demo.i]]", "--tags", "domain/dev-tools",
		"--allow-new-facet=domain")

	// Rewrite the plan with a real verify and well-formed task body.
	planPath := filepath.Join(vault, "80-plans", "demo.p.md")
	realPlan := `---
type: plan
id: demo.p
slug: p
title: "P"
description: "d"
created: 2026-05-12
updated: 2026-05-12
status: draft
plan_version: 1
issue: "[[issue.demo.i]]"
tags: [domain/dev-tools]
project: demo
tasks:
  - id: T1
    title: "Real task"
    kind: tdd
    files: ["a.go", "a_test.go"]
    depends_on: []
    verify: "go test ./..."
    success_criteria: []
---

## Task: T1

Real task body. This body has to be at least 200 characters long for the plan
validator to accept it, so we write a few sentences explaining the work the
agent would do. Add the type in a.go, RED test in a_test.go, run verify.
`
	if err := os.WriteFile(planPath, []byte(realPlan), 0o644); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")

	execCmd(t, "transition", "plan", "demo.p", "locked")

	a, err := core.LoadArtifact(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "locked" {
		t.Errorf("status = %v, want locked", a.FrontMatter["status"])
	}
}

func TestTransitionReverseAppendsAuditLine(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")
	execCmd(t, "transition", "issue", "demo.foo", "open", "--reason", "regression found")

	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "regression found") {
		t.Fatalf("audit line missing in body:\n%s", body)
	}
}

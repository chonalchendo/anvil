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

// createDemoIssue writes a legacy-format fixture issue demo.foo directly so
// tests that depend on the stable ID "demo.foo" continue to work regardless of
// the numbered-filename scheme applied to `create issue`.
func createDemoIssue(t *testing.T) {
	t.Helper()
	vault := os.Getenv("ANVIL_VAULT")
	writeFixtureIssueDated(t, vault, "demo", "foo", "foo", "2026-01-01")
	// Fixtures are written directly via Artifact.Save (no write-through), so
	// refresh the index to match — callers expect a fresh index, as they got
	// when this helper went through `create`.
	execCmd(t, "reindex")
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

	// Without --json: error surfaces on stderr via fang; stdout is empty.
	c := newRootCmd()
	c.SetArgs([]string{"transition", "issue", "demo.foo", "resolved"})
	var stdout, stderr bytes.Buffer
	c.SetOut(&stdout)
	c.SetErr(&stderr)
	if err := c.Execute(); err == nil {
		t.Fatalf("expected illegal_transition error; stderr: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout must be empty without --json, got: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), `{"code"`) {
		t.Fatalf("stderr must not contain JSON without --json, got: %s", stderr.String())
	}
}

func TestTransitionIllegalJSON(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	// With --json: JSON envelope on stdout, stderr empty, no error returned.
	c := newRootCmd()
	c.SetArgs([]string{"transition", "issue", "demo.foo", "resolved", "--json"})
	var stdout, stderr bytes.Buffer
	c.SetOut(&stdout)
	c.SetErr(&stderr)
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error with --json: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &env); err != nil {
		t.Fatalf("stdout must be valid JSON with --json; stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
	}
	if env["code"] != "illegal_transition" {
		t.Fatalf("expected code=illegal_transition, got: %v", env)
	}
	if strings.Contains(stderr.String(), `{"code"`) {
		t.Fatalf("stderr must not contain JSON with --json, got: %s", stderr.String())
	}
}

func TestTransitionMissingRequiredFlag(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "owner") {
		t.Fatalf("expected `owner` mentioned in JSON stdout: %s", stdout.String())
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

func TestTransitionClaimRecordsSession(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv(envSessionID, "session-a")
	execCmd(t, "init", vault)
	createDemoIssue(t)

	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	a, err := core.LoadArtifact(filepath.Join(vault, core.TypeIssue.Dir(), "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := a.FrontMatter["claim_session"].(string); got != "session-a" {
		t.Fatalf("claim_session = %q, want session-a", got)
	}

	// A recorded claim_session must survive validate (additionalProperties:false).
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate rejected claim_session: %v\n%s", err, out.String())
	}
}

func TestTransitionReclaimSameSessionIsIdempotent(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv(envSessionID, "session-a")
	execCmd(t, "init", vault)
	createDemoIssue(t)

	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	out := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--json")
	if !strings.Contains(out, `"already_in_state"`) {
		t.Fatalf("same-session re-claim: expected already_in_state, got %s", out)
	}
}

func TestTransitionReclaimDifferentSessionRefused(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv(envSessionID, "session-a")
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	// A different session under the same owner is refused; with --json the
	// envelope lands on stdout naming the holding session.
	t.Setenv(envSessionID, "session-b")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "claim_held_by_other_session") || !strings.Contains(stdout.String(), "session-a") {
		t.Fatalf("refusal must name the holding session; stdout: %s", stdout.String())
	}

	// --force overrides the refusal.
	cmd = newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--force", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--force should override the refusal: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "already_in_state") {
		t.Fatalf("forced re-claim should report already_in_state; output: %s", out.String())
	}
}

func TestTransitionForceTakeoverTransfersClaimSession(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	t.Setenv(envSessionID, "session-a")
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	// session-b takes over with --force.
	t.Setenv(envSessionID, "session-b")
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--force")

	// claim_session must now equal session-b (genuine takeover, not a no-op).
	a, err := core.LoadArtifact(filepath.Join(vault, core.TypeIssue.Dir(), "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := a.FrontMatter["claim_session"].(string); got != "session-b" {
		t.Fatalf("claim_session = %q after --force takeover, want session-b", got)
	}

	// session-b can now re-claim idempotently without --force.
	out := execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--json")
	if !strings.Contains(out, `"already_in_state"`) {
		t.Fatalf("new holder re-claim: expected already_in_state, got %s", out)
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
		"--goal", "I is done",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "plan", "--title", "P", "--description", "d",
		"--issue", "[[issue.demo.i]]", "--tags", "domain/dev-tools",
		"--allow-new-facet=domain")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "plan", "demo.i", "locked"})
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
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "demo.i.md"))
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "draft" {
		t.Errorf("status = %v, want draft", a.FrontMatter["status"])
	}

	// Index must reflect the same draft state — guards against a future
	// reorder that writes the index before validation.
	row, ierr := openIndex(t, vault).GetArtifact("demo.i")
	if ierr != nil {
		t.Fatalf("loading index row: %v", ierr)
	}
	if row.Status != "draft" {
		t.Errorf("index status = %q, want draft", row.Status)
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
		"--goal", "I is done",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "plan", "--title", "P", "--description", "d",
		"--issue", "[[issue.demo.i]]", "--tags", "domain/dev-tools",
		"--allow-new-facet=domain")

	// Rewrite the plan with a real verify and well-formed task body.
	planPath := filepath.Join(vault, "80-plans", "demo.i.md")
	realPlan := `---
type: plan
id: demo.i
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
	if err := os.WriteFile(planPath, []byte(realPlan), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
	execCmd(t, "reindex")

	execCmd(t, "transition", "plan", "demo.i", "locked")

	a, err := core.LoadArtifact(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "locked" {
		t.Errorf("status = %v, want locked", a.FrontMatter["status"])
	}
}

// TestTransitionIllegalLeavesDiskUnchanged covers the bug a user reported as
// "transition silently no-ops on backward moves". The CLI in fact rejects the
// move with exit 1, but the error envelope read as success-shaped to a fast
// scan. This test pins both halves: the error fires AND disk state is
// preserved. It also asserts the rejection now surfaces an `anvil set`
// escape-hatch hint so agents know how to force the move when intended.
func TestTransitionIllegalLeavesDiskUnchanged(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	execCmd(t, "transition", "issue", "demo.foo", "resolved")

	// resolved → in-progress is not in the transitions table.
	// Use --json so the error envelope lands on stdout for inspection.
	c := newRootCmd()
	c.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--json"})
	var stdout, stderr bytes.Buffer
	c.SetOut(&stdout)
	c.SetErr(&stderr)
	if err := c.Execute(); err != nil {
		t.Fatalf("expected nil error with --json; err: %v stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "illegal_transition") {
		t.Errorf("stdout should mention illegal_transition: %s stdout=%s stderr=%s", "", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "anvil set") {
		t.Errorf("stdout should point at `anvil set` escape hatch: stdout=%s", stdout.String())
	}

	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v after illegal transition, want resolved (unchanged)", a.FrontMatter["status"])
	}
}

// TestTransitionIssueStateGraphAllEdgesPersist walks every legal edge in the
// issue state machine and asserts the file on disk reflects the move after
// each transition. Guards against future regressions where a transition
// reports success but never writes (the original bug framing).
func TestTransitionIssueStateGraphAllEdgesPersist(t *testing.T) {
	type step struct {
		to         string
		flags      []string
		wantOnDisk string
	}
	cases := []struct {
		name  string
		steps []step
	}{
		{
			name: "forward to resolved",
			steps: []step{
				{to: "in-progress", flags: []string{"--owner", "claude"}, wantOnDisk: "in-progress"},
				{to: "resolved", wantOnDisk: "resolved"},
			},
		},
		{
			name: "in-progress audit edge back to open",
			steps: []step{
				{to: "in-progress", flags: []string{"--owner", "claude"}, wantOnDisk: "in-progress"},
				{to: "open", wantOnDisk: "open"},
			},
		},
		{
			name: "open to abandoned",
			steps: []step{
				{to: "abandoned", wantOnDisk: "abandoned"},
			},
		},
		{
			name: "in-progress to abandoned",
			steps: []step{
				{to: "in-progress", flags: []string{"--owner", "claude"}, wantOnDisk: "in-progress"},
				{to: "abandoned", wantOnDisk: "abandoned"},
			},
		},
		{
			name: "reverse resolved to open with reason",
			steps: []step{
				{to: "in-progress", flags: []string{"--owner", "claude"}, wantOnDisk: "in-progress"},
				{to: "resolved", wantOnDisk: "resolved"},
				{to: "open", flags: []string{"--reason", "regression"}, wantOnDisk: "open"},
			},
		},
		{
			name: "reverse abandoned to open with reason",
			steps: []step{
				{to: "abandoned", wantOnDisk: "abandoned"},
				{to: "open", flags: []string{"--reason", "back on the plate"}, wantOnDisk: "open"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := t.TempDir()
			t.Setenv("ANVIL_VAULT", vault)
			execCmd(t, "init", vault)
			createDemoIssue(t)
			path := filepath.Join(vault, "70-issues", "demo.foo.md")

			for i, s := range tc.steps {
				args := append([]string{"transition", "issue", "demo.foo", s.to}, s.flags...)
				execCmd(t, args...)
				a, err := core.LoadArtifact(path)
				if err != nil {
					t.Fatalf("step %d (%s): load: %v", i, s.to, err)
				}
				if a.FrontMatter["status"] != s.wantOnDisk {
					t.Fatalf("step %d (%s): disk status = %v, want %v", i, s.to, a.FrontMatter["status"], s.wantOnDisk)
				}
			}
		})
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

	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md")) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "regression found") {
		t.Fatalf("audit line missing in body:\n%s", body)
	}
}

// TestTransition_Issue_ByOrdinal pins the write-path counterpart to
// show_test.go's TestShow_Issue_ByOrdinal: a bare ordinal ("1") and a
// project-qualified ordinal ("foo.0001") both resolve to the full issue ID on
// the transition write path, matching the read path the fix unified them with.
func TestTransition_Issue_ByOrdinal(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	path := createIssueGetPath(t,
		"create", "issue",
		"--title", "Transition me by ordinal",
		"--description", "d",
		"--goal", "g",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
	id := strings.TrimSuffix(filepath.Base(path), ".md")
	if !strings.HasPrefix(id, "foo.0001.") {
		t.Fatalf("expected first issue at ordinal 0001, got %q", id)
	}

	// Bare ordinal on the write path.
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "1", "in-progress", "--owner", "claude"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("transition issue 1: %v\n%s", err, out.String())
	}
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := a.FrontMatter["status"].(string); got != "in-progress" {
		t.Fatalf("bare ordinal: status = %q, want in-progress", got)
	}

	// Project-qualified ordinal resolves the same artifact.
	cmd = newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "foo.0001", "resolved"})
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("transition issue foo.0001: %v\n%s", err, out.String())
	}
	a, err = core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := a.FrontMatter["status"].(string); got != "resolved" {
		t.Fatalf("project-qualified ordinal: status = %q, want resolved", got)
	}
}

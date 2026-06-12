package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// TestDoctorMergedPRIssue verifies that an in-progress issue with a MERGED PR
// in external_links produces a merged-pr-issue finding.
func TestDoctorMergedPRIssue(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "foo.stale-0001"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":           "issue",
			"title":          "stale issue",
			"status":         "in-progress",
			"project":        "foo",
			"created":        "2026-06-01",
			"updated":        "2026-06-01",
			"severity":       "medium",
			"external_links": []any{"https://github.com/org/repo/pull/42"},
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	old := ghPRStateByURLFn
	t.Cleanup(func() { ghPRStateByURLFn = old })
	ghPRStateByURLFn = func(_ string) (string, error) { return "MERGED", nil }

	findings, err := runDoctor(v)
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding, got none")
	}
	found := false
	for _, f := range findings {
		if f.Kind == "merged-pr-issue" && f.ID == id {
			found = true
			if f.Fix == "" {
				t.Error("finding has empty fix")
			}
		}
	}
	if !found {
		t.Errorf("no merged-pr-issue finding for %s; got %v", id, findings)
	}
}

// TestDoctorDeadClaim verifies that an in-progress issue with claim_session
// but no worktree and no open PR produces a dead-claim finding.
func TestDoctorDeadClaim(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "foo.dead-0002"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "dead claim",
			"status":        "in-progress",
			"project":       "foo",
			"created":       "2026-06-01",
			"updated":       "2026-06-01",
			"severity":      "medium",
			"claim_session": "dead-session-uuid",
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	// No worktrees, no open PRs.
	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	// No external_links on this issue — dead claim with no PR.
	findings, err := runDoctor(v)
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == id {
			found = true
			if f.Fix == "" {
				t.Error("finding has empty fix")
			}
		}
	}
	if !found {
		t.Errorf("no dead-claim finding for %s; got %v", id, findings)
	}
}

// TestDoctorDeadClaim_LiveWorktreeSuppresses verifies that a live worktree
// prevents a dead-claim finding.
func TestDoctorDeadClaim_LiveWorktreeSuppresses(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "foo.live-0003"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "live claim",
			"status":        "in-progress",
			"project":       "foo",
			"created":       "2026-06-01",
			"updated":       "2026-06-01",
			"severity":      "medium",
			"claim_session": "live-session-uuid",
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	// Live worktree on the conventional branch: anvil/<slug-from-issue-id>.
	// The slug is the part after the first "." in the issue id.
	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) {
		return map[string]worktreeInfo{
			"anvil/live-0003": {path: "/tmp/live-0003"},
		}, nil
	}

	findings, err := runDoctor(v)
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == id {
			t.Errorf("unexpected dead-claim finding for issue with live worktree")
		}
	}
}

// TestDoctorFinishedMilestone verifies that an in-progress milestone whose
// child issues are all resolved produces a finished-milestone finding.
func TestDoctorFinishedMilestone(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	msSlug := "anvil.test-milestone"
	msPath := filepath.Join(vault, "85-milestones", msSlug+".md")
	ms := &core.Artifact{
		Path: msPath,
		FrontMatter: map[string]any{
			"type":    "milestone",
			"title":   "test milestone",
			"status":  "in-progress",
			"project": "anvil",
			"created": "2026-06-01",
			"updated": "2026-06-01",
		},
		Body: "## Goal\n\nAll done.\n",
	}
	if err := ms.Save(); err != nil {
		t.Fatal(err)
	}

	// One resolved child.
	childPath := filepath.Join(vault, "70-issues", "anvil.done-issue.md")
	child := &core.Artifact{
		Path: childPath,
		FrontMatter: map[string]any{
			"type":      "issue",
			"title":     "done issue",
			"status":    "resolved",
			"project":   "anvil",
			"created":   "2026-06-01",
			"updated":   "2026-06-01",
			"severity":  "medium",
			"milestone": "[[milestone." + msSlug + "]]",
		},
		Body: fixtureIssueBody,
	}
	if err := child.Save(); err != nil {
		t.Fatal(err)
	}

	// No worktrees.
	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	findings, err := runDoctor(v)
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Kind == "finished-milestone" && f.ID == msSlug {
			found = true
			if f.Fix == "" {
				t.Error("finding has empty fix")
			}
		}
	}
	if !found {
		t.Errorf("no finished-milestone finding for %s; got %v", msSlug, findings)
	}
}

// TestDoctorOrphanWorktree verifies that an anvil/ worktree whose branch is
// gone on origin produces an orphan-worktree finding.
func TestDoctorOrphanWorktree(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) {
		return map[string]worktreeInfo{
			"anvil/orphaned-slug": {path: "/tmp/orphaned"},
		}, nil
	}

	oldBranch := gitBranchExistsFn
	t.Cleanup(func() { gitBranchExistsFn = oldBranch })
	gitBranchExistsFn = func(_ string) (bool, error) { return false, nil }

	findings, err := runDoctor(v)
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Kind == "orphan-worktree" && f.ID == "anvil/orphaned-slug" {
			found = true
			if f.Fix == "" {
				t.Error("finding has empty fix")
			}
		}
	}
	if !found {
		t.Errorf("no orphan-worktree finding; got %v", findings)
	}
}

// TestDoctorJSON_Envelope verifies the --json output shape required by the
// Indirect verification: has("findings") and each item has kind, id, fix.
func TestDoctorJSON_Envelope(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)

	// Seed a merged-PR issue so findings is non-empty (exercises the full path).
	id := "foo.json-test"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":           "issue",
			"title":          "json test",
			"status":         "in-progress",
			"project":        "foo",
			"created":        "2026-06-01",
			"updated":        "2026-06-01",
			"severity":       "medium",
			"external_links": []any{"https://github.com/org/repo/pull/99"},
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	old := ghPRStateByURLFn
	t.Cleanup(func() { ghPRStateByURLFn = old })
	ghPRStateByURLFn = func(_ string) (string, error) { return "MERGED", nil }

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	oldBranch := gitBranchExistsFn
	t.Cleanup(func() { gitBranchExistsFn = oldBranch })
	gitBranchExistsFn = func(_ string) (bool, error) { return true, nil }

	cmd := newRootCmd()
	stdout, _, err := runCmd(t, cmd, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v", err)
	}

	var env doctorEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("invalid JSON: %v\nout: %s", err, stdout)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("raw parse: %v", err)
	}
	if _, ok := raw["findings"]; !ok {
		t.Error("JSON envelope missing 'findings' key")
	}
	for _, f := range env.Findings {
		if f.Kind == "" {
			t.Errorf("finding missing kind: %+v", f)
		}
		if f.ID == "" {
			t.Errorf("finding missing id: %+v", f)
		}
		if f.Fix == "" {
			t.Errorf("finding missing fix: %+v", f)
		}
	}
}

// TestDoctorJSON_EmptyFindings verifies that the envelope is emitted even with
// no findings (empty array, not null).
func TestDoctorJSON_EmptyFindings(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)

	// Write a resolved issue — should not appear in findings.
	issPath := filepath.Join(vault, "70-issues", "foo.resolved.md")
	a := &core.Artifact{
		Path: issPath,
		FrontMatter: map[string]any{
			"type":     "issue",
			"title":    "resolved",
			"status":   "resolved",
			"project":  "foo",
			"created":  "2026-06-01",
			"updated":  "2026-06-01",
			"severity": "medium",
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	oldBranch := gitBranchExistsFn
	t.Cleanup(func() { gitBranchExistsFn = oldBranch })
	gitBranchExistsFn = func(_ string) (bool, error) { return true, nil }

	cmd := newRootCmd()
	stdout, _, err := runCmd(t, cmd, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("raw parse: %v", err)
	}
	if _, ok := raw["findings"]; !ok {
		t.Error("JSON envelope missing 'findings' key")
	}
	items, _ := raw["findings"].([]any)
	if items == nil {
		t.Error("findings must be an array (not null) even when empty")
	}
}

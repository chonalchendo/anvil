package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// stubGhPRList swaps ghPRListFn for the duration of a test. fn receives the
// branch passed to gh and returns (url, error).
func stubGhPRList(t *testing.T, fn func(branch string) (string, error)) {
	t.Helper()
	prev := ghPRListFn
	ghPRListFn = fn
	t.Cleanup(func() { ghPRListFn = prev })
}

// TestTransitionResolvedRefusesWhenOpenPR pins the core acceptance: an open PR
// on the issue's anvil/<slug> branch causes `resolved` to refuse with an
// error envelope that names the PR url and the --force escape hatch.
func TestTransitionResolvedRefusesWhenOpenPR(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	wantPRURL := "https://github.com/acme/demo/pull/42"
	var queried []string
	stubGhPRList(t, func(branch string) (string, error) {
		queried = append(queried, branch)
		if branch == "anvil/foo" {
			return wantPRURL, nil
		}
		return "", nil
	})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected refusal; output: %s", out.String())
	}
	got := out.String()
	if !strings.Contains(got, "open_pr_blocks_resolve") {
		t.Errorf("missing error code: %s", got)
	}
	if !strings.Contains(got, wantPRURL) {
		t.Errorf("missing PR url: %s", got)
	}
	if !strings.Contains(got, "--force") {
		t.Errorf("missing --force hint: %s", got)
	}
	if len(queried) == 0 || queried[0] != "anvil/foo" {
		t.Errorf("expected gh to be queried with anvil/foo first; got %v", queried)
	}

	// Disk state must remain in-progress — the refusal aborts before write.
	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status: in-progress") {
		t.Errorf("expected status unchanged at in-progress, got:\n%s", body)
	}
}

// TestTransitionResolvedForceOverridesAndLogsAudit pins the --force half of
// AC3: the override succeeds AND leaves a body-line audit trail naming
// --force so a later reader sees the post-merge intent.
func TestTransitionResolvedForceOverridesAndLogsAudit(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	stubGhPRList(t, func(_ string) (string, error) {
		return "https://github.com/acme/demo/pull/99", nil
	})

	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--force", "--reason", "merged out-of-band")

	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "status: resolved") {
		t.Errorf("status should be resolved:\n%s", s)
	}
	if !strings.Contains(s, "resolved --force") {
		t.Errorf("audit line missing '--force' tag:\n%s", s)
	}
	if !strings.Contains(s, "merged out-of-band") {
		t.Errorf("audit line missing reason:\n%s", s)
	}
}

// TestTransitionResolvedSucceedsWhenNoOpenPR confirms the happy path remains
// happy when gh reports nothing on every candidate branch.
func TestTransitionResolvedSucceedsWhenNoOpenPR(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	stubGhPRList(t, func(_ string) (string, error) {
		return "", nil
	})

	execCmd(t, "transition", "issue", "demo.foo", "resolved")
	body, err := os.ReadFile(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status: resolved") {
		t.Errorf("expected resolved status; got:\n%s", body)
	}
}

// TestTransitionResolvedGhMissingDowngradesToWarning ensures environments
// without gh (CI containers, fresh laptops) aren't blocked outright. The
// check warns to stderr and lets the transition proceed.
func TestTransitionResolvedGhMissingDowngradesToWarning(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	stubGhPRList(t, func(_ string) (string, error) {
		return "", errGhMissing
	})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "resolved"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success when gh missing; got err=%v output=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "gh not on PATH") {
		t.Errorf("expected gh-missing warning; got: %s", out.String())
	}
}

// TestCandidateBranchesIncludesCurrentBranch verifies the worktree-branch
// fallback: when an agent runs `anvil transition resolved` from inside a
// worktree whose branch is `anvil/<divergent-slug>`, the check still finds
// the PR — covers the fleet-dispatcher case where neither the issue id nor
// any plan frontmatter names the branch slug.
func TestCandidateBranchesIncludesCurrentBranch(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	repo := setupGitRepo(t, "git@github.com:acme/demo.git")
	if out, err := runIn(repo, "git", "commit", "--allow-empty", "-m", "init", "-q"); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
	if out, err := runIn(repo, "git", "checkout", "-q", "-b", "anvil/short-divergent"); err != nil {
		t.Fatalf("git checkout: %v %s", err, out)
	}
	t.Chdir(repo)

	branches := candidateBranchesForIssue(&core.Vault{Root: vault}, "demo.foo")
	var saw bool
	for _, b := range branches {
		if b == "anvil/short-divergent" {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected anvil/short-divergent in candidates (current-branch fallback); got %v", branches)
	}
}

// TestCandidateBranchesIncludesLinkedPlanSlug verifies a non-id-slug branch
// (the dispatcher-chosen slug case) is still discovered via the incoming
// plan link.
func TestCandidateBranchesIncludesLinkedPlanSlug(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)

	// Author a plan whose slug diverges from the issue id slug; link it.
	planPath := filepath.Join(vault, "80-plans", "demo.foo.md")
	planBody := `---
type: plan
id: demo.foo
slug: short-branch-name
title: "P"
description: "d"
created: 2026-05-15
updated: 2026-05-15
status: draft
plan_version: 1
issue: "[[issue.demo.foo]]"
tags: [domain/dev-tools]
project: demo
tasks: []
---

body
`
	if err := os.WriteFile(planPath, []byte(planBody), 0o644); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")

	branches := candidateBranchesForIssue(&core.Vault{Root: vault}, "demo.foo")

	var sawIDSlug, sawPlanSlug bool
	for _, b := range branches {
		if b == "anvil/foo" {
			sawIDSlug = true
		}
		if b == "anvil/short-branch-name" {
			sawPlanSlug = true
		}
	}
	if !sawIDSlug {
		t.Errorf("expected anvil/foo (id-slug) in candidates: %v", branches)
	}
	if !sawPlanSlug {
		t.Errorf("expected anvil/short-branch-name (plan-slug) in candidates: %v", branches)
	}
}

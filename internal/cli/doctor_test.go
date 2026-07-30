package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding, got none")
	}
	found := false
	for _, f := range findings {
		if f.Kind == "merged-pr-issue" && f.ID == core.CanonicalID(core.TypeIssue, id) {
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

	// No worktrees, no open PRs, and doctor runs from a different session.
	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	// No external_links on this issue — dead claim with no PR.
	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
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

// TestDoctorDeadClaim_MergedBranchRecommendsResolve verifies that a dead-claim
// whose conventional branch has a merged PR gets a "resolved" fix, not "open".
// This is the merged-but-unresolved shape: the work landed but the issue was
// never transitioned — doctor must not recommend reopening already-fixed work.
func TestDoctorDeadClaim_MergedBranchRecommendsResolve(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "foo.merged-fix-0020"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "fix already merged",
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

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	// The conventional branch for a `foo`-project issue is foo/<slug>; that
	// branch has a merged PR.
	oldMerged := ghMergedPRForBranchFn
	t.Cleanup(func() { ghMergedPRForBranchFn = oldMerged })
	ghMergedPRForBranchFn = func(branch string) (int, bool, error) {
		if branch == "foo/merged-fix-0020" {
			return 55, true, nil
		}
		return 0, false, nil
	}

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
			found = true
			if f.Fix != "anvil transition issue "+core.CanonicalID(core.TypeIssue, id)+" resolved" {
				t.Errorf("expected fix 'anvil transition issue %s resolved', got %q", id, f.Fix)
			}
		}
	}
	if !found {
		t.Errorf("no dead-claim finding for %s; got %v", id, findings)
	}
}

// TestDoctorDeadClaim_NoMergedBranchRecommendsOpen verifies that a dead-claim
// with no merged branch still recommends the reopen ("open") transition.
func TestDoctorDeadClaim_NoMergedBranchRecommendsOpen(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "foo.truly-abandoned-0021"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "truly abandoned",
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

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	oldMerged := ghMergedPRForBranchFn
	t.Cleanup(func() { ghMergedPRForBranchFn = oldMerged })
	ghMergedPRForBranchFn = func(_ string) (int, bool, error) { return 0, false, nil }

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
			found = true
			if f.Fix != "anvil transition issue "+core.CanonicalID(core.TypeIssue, id)+" open" {
				t.Errorf("expected fix 'anvil transition issue %s open', got %q", id, f.Fix)
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
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) {
		return map[string]worktreeInfo{
			"anvil/live-0003": {path: "/tmp/live-0003"},
		}, nil
	}

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
			t.Errorf("unexpected dead-claim finding for issue with live worktree")
		}
	}
}

// TestDoctorDeadClaim_RenamedBranchWorktreeSuppresses verifies that a live
// worktree is matched by its directory (the durable worktree↔issue mapping),
// not by branch name — a branch renamed off the issue slug must not false-
// positive a dead-claim. Regression for anvil.0146.
func TestDoctorDeadClaim_RenamedBranchWorktreeSuppresses(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "foo.renamed-0005"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "renamed branch claim",
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

	// Worktree is live on disk, keyed under a branch renamed off the issue
	// slug — only the worktree directory name still matches.
	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) {
		return map[string]worktreeInfo{
			"foo/harden-renamed-0005-orphan-filter": {path: "/tmp/foo-worktrees/renamed-0005"},
		}, nil
	}

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
			t.Errorf("unexpected dead-claim finding for issue with live worktree on renamed branch")
		}
	}
}

// TestDoctorDeadClaim_CurrentSessionSuppresses verifies that a claim held by
// the session running doctor is never flagged, even with no worktree or PR.
func TestDoctorDeadClaim_CurrentSessionSuppresses(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "foo.mine-0004"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "my own claim",
			"status":        "in-progress",
			"project":       "foo",
			"created":       "2026-06-01",
			"updated":       "2026-06-01",
			"severity":      "medium",
			"claim_session": "this-session-uuid",
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "this-session-uuid")

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
			t.Errorf("unexpected dead-claim finding for the current session's own claim")
		}
	}
}

// TestDoctorDeadClaim_OtherProjectSkipped verifies that issues bound to a
// different project are not judged against this repo's worktrees.
func TestDoctorDeadClaim_OtherProjectSkipped(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	id := "bar.elsewhere-0005"
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "other project claim",
			"status":        "in-progress",
			"project":       "bar",
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

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
			t.Errorf("unexpected dead-claim finding for another project's issue")
		}
	}
}

// writeSessionStub writes a minimal session file under 10-sessions/ with the
// given started_at, for the dead-claim liveness tests.
func writeSessionStub(t *testing.T, vault, sessionID, startedAt string) {
	t.Helper()
	path := filepath.Join(vault, "10-sessions", sessionID+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type":       "session",
			"session_id": sessionID,
			"source":     "claude-code",
			"started_at": startedAt,
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// claimedIssue saves an in-progress issue claimed by claimSession with no
// worktree or PR — the dead-claim shape — and returns its id.
func claimedIssue(t *testing.T, vault, id, claimSession string) string {
	t.Helper()
	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", id+".md"),
		FrontMatter: map[string]any{
			"type":          "issue",
			"title":         "claim",
			"status":        "in-progress",
			"project":       "foo",
			"created":       "2026-06-01",
			"updated":       "2026-06-01",
			"severity":      "medium",
			"claim_session": claimSession,
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return id
}

func hasDeadClaim(findings []doctorFinding, id string) bool {
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == core.CanonicalID(core.TypeIssue, id) {
			return true
		}
	}
	return false
}

// TestDoctorDeadClaim_LiveSessionSuppresses verifies that a claim held by a
// concurrent session that started recently — its session file exists with a
// fresh started_at — is not flagged, even with no worktree or PR. This is the
// false positive anvil.0063 fixes.
func TestDoctorDeadClaim_LiveSessionSuppresses(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	sess := "concurrent-live-session"
	writeSessionStub(t, vault, sess, time.Now().UTC().Format(time.RFC3339))
	id := claimedIssue(t, vault, "foo.live-sess-0010", sess)

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	if hasDeadClaim(findings, id) {
		t.Errorf("unexpected dead-claim for issue claimed by a recently-started session")
	}
}

// TestDoctorDeadClaim_StaleSessionFlagged verifies that a lingering session file
// whose started_at is older than the liveness window does not suppress the
// finding — a claim from a long-dead session is still reported.
func TestDoctorDeadClaim_StaleSessionFlagged(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	sess := "long-dead-session"
	stale := time.Now().UTC().Add(-2 * sessionLivenessWindow).Format(time.RFC3339)
	writeSessionStub(t, vault, sess, stale)
	id := claimedIssue(t, vault, "foo.stale-sess-0011", sess)

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	if !hasDeadClaim(findings, id) {
		t.Errorf("no dead-claim for issue claimed by a session older than the liveness window; got %v", findings)
	}
}

// TestDoctorDeadClaim_SessionWithoutStartedAtFlagged verifies that a session
// file lacking a parseable started_at does not suppress the finding — doctor
// reports rather than trust an unreadable start time (no mtime guessing).
func TestDoctorDeadClaim_SessionWithoutStartedAtFlagged(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	sess := "no-startedat-session"
	a := &core.Artifact{
		Path:        filepath.Join(vault, "10-sessions", sess+".md"),
		FrontMatter: map[string]any{"type": "session", "session_id": sess, "source": "claude-code"},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	id := claimedIssue(t, vault, "foo.no-startedat-0012", sess)

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	if !hasDeadClaim(findings, id) {
		t.Errorf("expected dead-claim for a session file without parseable started_at; got %v", findings)
	}
}

// TestClaimSessionLive_WindowBoundary pins the recency window: a session that
// started just inside the window is live; just outside, it is not.
func TestClaimSessionLive_WindowBoundary(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}
	now := time.Now().UTC()
	sess := "boundary-session"

	writeSessionStub(t, vault, sess, now.Add(-sessionLivenessWindow+time.Minute).Format(time.RFC3339))
	if !claimSessionLive(v, sess, now) {
		t.Error("session started just inside the window should be live")
	}
	writeSessionStub(t, vault, sess, now.Add(-sessionLivenessWindow-time.Minute).Format(time.RFC3339))
	if claimSessionLive(v, sess, now) {
		t.Error("session started just outside the window should not be live")
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
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	findings, err := runDoctor(v, "anvil")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Kind == "finished-milestone" && f.ID == core.CanonicalID(core.TypeMilestone, msSlug) {
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

// runFinishedMilestoneCheck builds a vault with one milestone at the given
// status/kind and a single resolved child, then reports whether doctor emits a
// finished-milestone finding for it. kind "" omits the field.
func runFinishedMilestoneCheck(t *testing.T, status, kind string) bool {
	t.Helper()
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	msSlug := "anvil.test-milestone"
	fm := map[string]any{
		"type":    "milestone",
		"title":   "test milestone",
		"status":  status,
		"project": "anvil",
		"created": "2026-06-01",
		"updated": "2026-06-01",
	}
	if kind != "" {
		fm["kind"] = kind
	}
	ms := &core.Artifact{Path: filepath.Join(vault, "85-milestones", msSlug+".md"), FrontMatter: fm, Body: "## Goal\n\nAll done.\n"}
	if err := ms.Save(); err != nil {
		t.Fatal(err)
	}

	child := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "anvil.done-issue.md"),
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

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	findings, err := runDoctor(v, "anvil")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "finished-milestone" && f.ID == core.CanonicalID(core.TypeMilestone, msSlug) {
			return true
		}
	}
	return false
}

// A planned milestone reaches done directly, so all-issues-resolved at planned
// is finished — the gap doctor missed by gating only on in-progress.
func TestDoctorFinishedMilestone_Planned(t *testing.T) {
	if !runFinishedMilestoneCheck(t, "planned", "scoped") {
		t.Error("planned milestone with all issues resolved should be flagged finished")
	}
}

// Buckets have no terminal done state; all issues resolved must not flag them.
func TestDoctorFinishedMilestone_BucketNotFlagged(t *testing.T) {
	if runFinishedMilestoneCheck(t, "planned", "bucket") {
		t.Error("bucket milestone must not be flagged finished")
	}
}

// runContractRailCheck builds a vault with one contract (given status/body)
// and, when withConvention is set, one convention artifact, then reports
// whether doctor emits a contract-empty-convention-rail finding for it.
func runContractRailCheck(t *testing.T, status, body string, withConvention bool) bool {
	t.Helper()
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	if withConvention {
		if err := os.MkdirAll(filepath.Join(vault, "35-conventions"), 0o750); err != nil {
			t.Fatal(err)
		}
		conv := &core.Artifact{
			Path: filepath.Join(vault, "35-conventions", "convention.go.md"),
			FrontMatter: map[string]any{
				"type":    "convention",
				"title":   "go convention",
				"status":  "active",
				"created": "2026-06-01",
				"updated": "2026-06-01",
			},
			Body: "## Rules\n\nSome rules.\n",
		}
		if err := conv.Save(); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.MkdirAll(filepath.Join(vault, "75-contracts"), 0o750); err != nil {
		t.Fatal(err)
	}
	ct := &core.Artifact{
		Path: filepath.Join(vault, "75-contracts", "contract.anvil.engine.md"),
		FrontMatter: map[string]any{
			"type":    "contract",
			"title":   "engine contract",
			"status":  status,
			"project": "anvil",
			"created": "2026-06-01",
			"updated": "2026-06-01",
		},
		Body: body,
	}
	if err := ct.Save(); err != nil {
		t.Fatal(err)
	}

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	findings, err := runDoctor(v, "anvil")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "contract-empty-convention-rail" && f.ID == "contract.anvil.engine" {
			return true
		}
	}
	return false
}

func TestDoctorContractEmptyConventionRail(t *testing.T) {
	if !runContractRailCheck(t, "active", "## Does\n\n- Stuff.\n", true) {
		t.Error("active contract with no convention links should be flagged")
	}
}

func TestDoctorContractConventionRail_LinkedNotFlagged(t *testing.T) {
	if runContractRailCheck(t, "active", "## Code design\n\n- Follow [[convention.go]].\n", true) {
		t.Error("contract linking a convention must not be flagged")
	}
}

func TestDoctorContractConventionRail_InactiveNotFlagged(t *testing.T) {
	if runContractRailCheck(t, "draft", "## Does\n\n- Stuff.\n", true) {
		t.Error("non-active contract must not be flagged")
	}
}

func TestDoctorContractConventionRail_NoConventionsInVault(t *testing.T) {
	if runContractRailCheck(t, "active", "## Does\n\n- Stuff.\n", false) {
		t.Error("vault with no conventions must yield no rail findings")
	}
}

// TestDoctorOrphanWorktree verifies that an anvil/ worktree whose branch has
// a merged PR produces an orphan-worktree finding.
func TestDoctorOrphanWorktree(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) {
		return map[string]worktreeInfo{
			"anvil/orphaned-slug": {path: "/tmp/orphaned"},
		}, nil
	}

	oldMerged := ghMergedPRForBranchFn
	t.Cleanup(func() { ghMergedPRForBranchFn = oldMerged })
	ghMergedPRForBranchFn = func(_ string) (int, bool, error) { return 42, true, nil }

	findings, err := runDoctor(v, "anvil")
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

// TestDoctorOrphanWorktree_NoMergedPRNotFlagged verifies that a branch with
// no merged PR — in-flight or never pushed — is not flagged, even when it is
// absent on origin.
func TestDoctorOrphanWorktree_NoMergedPRNotFlagged(t *testing.T) {
	vault := setupVault(t)
	v := &core.Vault{Root: vault}

	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) {
		return map[string]worktreeInfo{
			"anvil/in-flight-slug": {path: "/tmp/in-flight"},
		}, nil
	}

	oldMerged := ghMergedPRForBranchFn
	t.Cleanup(func() { ghMergedPRForBranchFn = oldMerged })
	ghMergedPRForBranchFn = func(_ string) (int, bool, error) { return 0, false, nil }

	findings, err := runDoctor(v, "anvil")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "orphan-worktree" {
			t.Errorf("unexpected orphan-worktree finding for branch with no merged PR: %+v", f)
		}
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
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

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
	gitWorktreeListFn = func(string) (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

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

package cli

import (
	"encoding/json"
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

	// No worktrees, no open PRs, and doctor runs from a different session.
	oldWT := gitWorktreeListFn
	t.Cleanup(func() { gitWorktreeListFn = oldWT })
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	// No external_links on this issue — dead claim with no PR.
	findings, err := runDoctor(v, "foo")
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

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == id {
			t.Errorf("unexpected dead-claim finding for issue with live worktree")
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "this-session-uuid")

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == id {
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
	t.Setenv(envSessionID, "some-other-session")

	findings, err := runDoctor(v, "foo")
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}
	for _, f := range findings {
		if f.Kind == "dead-claim" && f.ID == id {
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
		if f.Kind == "dead-claim" && f.ID == id {
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

	findings, err := runDoctor(v, "anvil")
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

// TestDoctorOrphanWorktree verifies that an anvil/ worktree whose branch has
// a merged PR produces an orphan-worktree finding.
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) {
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
	gitWorktreeListFn = func() (map[string]worktreeInfo, error) { return map[string]worktreeInfo{}, nil }

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

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// sessionLivenessWindow bounds how recently a claim_session must have started
// for its claim to count as live. Session files carry no heartbeat (written
// once at start, no end-marker), so doctor approximates liveness from the
// session's start time: a claim from a session that began within this window is
// assumed still-running and suppressed, while older claims with no worktree or
// open PR are reported. This trades a brief false negative (a claim from a
// session that died less than a day ago is not reported until the window lapses)
// for eliminating the false positive where a concurrent live session's fresh
// claims were flagged as dead.
const sessionLivenessWindow = 24 * time.Hour

// doctorFinding is one stale-lifecycle finding emitted by `anvil doctor`.
type doctorFinding struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Evidence string `json:"evidence"`
	Fix      string `json:"fix"`
}

// doctorEnvelope is the JSON envelope for `anvil doctor --json`.
type doctorEnvelope struct {
	Findings []doctorFinding `json:"findings"`
}

// Package-level indirection points so tests can swap network and filesystem calls.
var (
	ghPRStateByURLFn      = ghPRStateByURLReal
	ghMergedPRForBranchFn = ghMergedPRForBranchReal
)

func newDoctorCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Detect stale lifecycle state (merged-PR issues, dead claims, finished milestones, orphan worktrees)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			projSlug := ""
			if p, err := core.ResolveProject(); err == nil {
				projSlug = p.Slug
			}
			findings, err := runDoctor(v, projSlug)
			if err != nil {
				return err
			}
			if asJSON {
				env := doctorEnvelope{Findings: findings}
				if findings == nil {
					env.Findings = []doctorFinding{}
				}
				b, err := json.Marshal(env)
				if err != nil {
					return fmt.Errorf("marshalling json: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			if len(findings) == 0 {
				cmd.Println("doctor: no stale lifecycle state found")
				return nil
			}
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s — %s\n  fix: %s\n", f.Kind, f.ID, f.Evidence, f.Fix)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, `emit JSON envelope: {"findings":[...]}`)
	return cmd
}

// childIssue is the per-issue tuple the finished-milestone check consumes,
// collected during the single issue-loading pass in runDoctor.
type childIssue struct {
	status    string
	milestone string
}

// runDoctor checks all four stale-lifecycle shapes and returns the findings.
// Best-effort: a check that cannot shell out (gh missing, no network) skips
// rather than aborts so doctor is always usable in offline environments.
// projectSlug is the current project binding; repo-local checks (dead claim)
// only judge issues bound to it, because the worktree evidence comes from the
// cwd's repo and would be wrong for every other project in the vault.
func runDoctor(v *core.Vault, projectSlug string) ([]doctorFinding, error) {
	var findings []doctorFinding

	issuePaths, err := collectArtifactPaths(v.Root, core.TypeIssue)
	if err != nil {
		return nil, fmt.Errorf("reading issues: %w", err)
	}

	worktrees, _ := gitWorktreeListFn() // best-effort

	children := make([]childIssue, 0, len(issuePaths))
	for _, p := range issuePaths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			continue // skip unreadable
		}
		status, _ := a.FrontMatter["status"].(string)
		children = append(children, childIssue{
			status:    status,
			milestone: milestoneSlug(a.FrontMatter["milestone"]),
		})
		if status != "in-progress" {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(p), ".md")

		// Shape 1: merged-PR issue. PR state is queried by absolute URL, so
		// this is correct for every project in the vault.
		if f := checkMergedPR(id, a); f != nil {
			findings = append(findings, *f)
			continue
		}

		// Shape 2: dead claim — no open PR, no live worktree. Worktree
		// evidence is repo-local; other projects' claims are checked when
		// doctor runs from their repo.
		proj, _ := a.FrontMatter["project"].(string)
		if projectSlug == "" || proj != projectSlug {
			continue
		}
		if f := checkDeadClaim(v, id, a, worktrees); f != nil {
			findings = append(findings, *f)
		}
	}

	// Shape 3: finished milestone — in-progress with all children done.
	msPaths, err := collectArtifactPaths(v.Root, core.TypeMilestone)
	if err != nil {
		return nil, fmt.Errorf("reading milestones: %w", err)
	}
	for _, p := range msPaths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			continue
		}
		if f := checkFinishedMilestone(p, a, children); f != nil {
			findings = append(findings, *f)
		}
	}

	// Shape 4: orphan worktrees.
	for branch, wt := range worktrees {
		if f := checkOrphanWorktree(branch, wt.path); f != nil {
			findings = append(findings, *f)
		}
	}

	return findings, nil
}

// prLinks returns the GitHub pull-request URLs from an issue's
// external_links, dropping non-PR GitHub links so callers never shell out on
// queries doomed to fail.
func prLinks(a *core.Artifact) []string {
	links, _ := a.FrontMatter["external_links"].([]any)
	var out []string
	for _, raw := range links {
		url, ok := raw.(string)
		if !ok || !strings.Contains(url, "github.com") || !strings.Contains(url, "/pull/") {
			continue
		}
		out = append(out, url)
	}
	return out
}

// checkMergedPR returns a finding when the issue's external_links contains a
// GitHub PR URL whose state is MERGED. Returns nil when no PR link is present,
// gh is unavailable, or the PR is not merged.
func checkMergedPR(id string, a *core.Artifact) *doctorFinding {
	for _, url := range prLinks(a) {
		state, err := ghPRStateByURLFn(url)
		if err != nil {
			continue // gh unavailable or network error — skip
		}
		if state == "MERGED" {
			return &doctorFinding{
				Kind:     "merged-pr-issue",
				ID:       id,
				Evidence: fmt.Sprintf("PR %s is MERGED but issue is in-progress", url),
				Fix:      fmt.Sprintf("anvil transition issue %s resolved", id),
			}
		}
	}
	return nil
}

// checkDeadClaim returns a finding when an in-progress issue has no open PR
// and no live worktree — the claim is stranded from a dead session.
// Returns nil when claim_session is unset (issue never claimed via session),
// when the claim belongs to the session running doctor, when the claiming
// session started recently (see claimSessionLive), when a worktree is alive,
// or when an open PR exists.
func checkDeadClaim(v *core.Vault, id string, a *core.Artifact, worktrees map[string]worktreeInfo) *doctorFinding {
	claimSession, _ := a.FrontMatter["claim_session"].(string)
	if claimSession == "" {
		return nil // not session-claimed; not a dead claim
	}
	if claimSession == os.Getenv(envSessionID) {
		return nil // claimed by the session running doctor — alive by construction
	}
	// Alive if the claiming session started recently — a concurrent session
	// working the issue (claimed, but no worktree or PR yet) is not a dead
	// claim. Checked before the worktree/PR probes so a live concurrent claim
	// short-circuits the gh shell-out.
	if claimSessionLive(v, claimSession, time.Now().UTC()) {
		return nil
	}
	// Alive if a matching worktree exists.
	branches := fleetCandidateBranches(v, id)
	for _, b := range branches {
		if _, ok := worktrees[b]; ok {
			return nil
		}
	}
	if _, _, ok := uniqueSubstringWorktree(worktrees, id); ok {
		return nil
	}
	// Alive if there is an open PR linked via external_links.
	for _, url := range prLinks(a) {
		state, err := ghPRStateByURLFn(url)
		if err != nil {
			return nil // can't confirm — don't false-positive
		}
		if state == "OPEN" {
			return nil
		}
	}
	// Distinguish merged-but-unresolved (fix: resolve) from abandoned (fix: reopen).
	// Try the conventional branch names for this issue; if any has a merged PR the
	// fix already landed — recommend resolved, not open.
	for _, b := range branches {
		if _, merged, err := ghMergedPRForBranchFn(b); err == nil && merged {
			return &doctorFinding{
				Kind:     "dead-claim",
				ID:       id,
				Evidence: fmt.Sprintf("in-progress with claim_session %s but no live worktree or open PR; branch %s has a merged PR", claimSession, b),
				Fix:      fmt.Sprintf("anvil transition issue %s resolved", id),
			}
		}
	}
	return &doctorFinding{
		Kind:     "dead-claim",
		ID:       id,
		Evidence: fmt.Sprintf("in-progress with claim_session %s but no live worktree or open PR", claimSession),
		Fix:      fmt.Sprintf("anvil transition issue %s open", id),
	}
}

// claimSessionLive reports whether claimSession has a session file whose
// started_at is within sessionLivenessWindow of now — doctor's read-side
// liveness approximation, since session files carry no heartbeat. A claim is
// live only when the file exists and its started_at parses to a recent instant;
// a missing file (GC'd or never created) or one without a parseable started_at
// is not live, so doctor still reports it. A well-formed session always carries
// started_at as a quoted RFC3339 string (internal/templates/session.tmpl), so
// the string read is the only shape worth handling — an unreadable start time
// is treated as not-live rather than guessed at from file mtime.
func claimSessionLive(v *core.Vault, claimSession string, now time.Time) bool {
	path := filepath.Join(v.Root, core.TypeSession.Dir(), claimSession+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		return false // no session file — not a live session
	}
	s, _ := a.FrontMatter["started_at"].(string)
	started, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false // no parseable start time — can't claim it's live
	}
	return now.Sub(started) < sessionLivenessWindow
}

// checkFinishedMilestone returns a finding when an in-progress milestone has
// every child issue resolved or abandoned. Returns nil for milestones that are
// not in-progress, have open work, or have no children.
func checkFinishedMilestone(msPath string, a *core.Artifact, children []childIssue) *doctorFinding {
	status, _ := a.FrontMatter["status"].(string)
	if status != "in-progress" {
		return nil
	}
	msID := strings.TrimSuffix(filepath.Base(msPath), ".md")
	hasChild := false
	for _, c := range children {
		if c.milestone != msID {
			continue
		}
		hasChild = true
		if c.status == "open" || c.status == "in-progress" {
			return nil // still open work
		}
	}
	if !hasChild {
		return nil // no children — can't conclude it's done
	}
	return &doctorFinding{
		Kind:     "finished-milestone",
		ID:       msID,
		Evidence: "all child issues are resolved or abandoned",
		Fix:      fmt.Sprintf("anvil transition milestone %s done", msID),
	}
}

// checkOrphanWorktree returns a finding when an anvil/ worktree's branch has
// a merged PR — the work landed, the worktree is done. The merged-PR gate
// covers both stale shapes (branch deleted on origin, branch merged but not
// deleted) and never flags in-flight or never-pushed branches, which mere
// absence-on-origin would. Returns nil for non-anvil branches, branches with
// no merged PR, or when gh is unavailable.
func checkOrphanWorktree(branch, path string) *doctorFinding {
	if !strings.HasPrefix(branch, "anvil/") {
		return nil
	}
	prNum, merged, err := ghMergedPRForBranchFn(branch)
	if err != nil || !merged {
		return nil // error = can't tell; skip rather than false-positive
	}
	return &doctorFinding{
		Kind:     "orphan-worktree",
		ID:       branch,
		Evidence: fmt.Sprintf("branch %s has merged PR #%d; worktree at %s is stale", branch, prNum, path),
		Fix:      fmt.Sprintf("git worktree remove %s", path),
	}
}

// ghPRStateByURLReal queries gh for the state of a PR by URL.
// Returns the state string ("OPEN", "MERGED", "CLOSED") or an error when gh
// is unavailable or the query fails.
func ghPRStateByURLReal(url string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errGhUnavailable
	}
	out, err := exec.Command("gh", "pr", "view", url, "--json", "state").Output() //nolint:gosec // binary path resolved from trusted sources; not user input
	if err != nil {
		return "", errGhUnavailable
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return "", fmt.Errorf("gh pr view: parse: %w", err)
	}
	return v.State, nil
}

// ghMergedPRForBranchReal returns the number of a merged PR whose head is
// branch, queried via `gh pr list`. merged is false when no such PR exists;
// err signals gh is unavailable or the query failed.
func ghMergedPRForBranchReal(branch string) (prNum int, merged bool, err error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return 0, false, errGhUnavailable
	}
	out, err := exec.Command("gh", "pr", "list", "--head", branch, "--state", "merged", "--json", "number", "--limit", "1").Output() //nolint:gosec // branch is a package-controlled slug, never user input
	if err != nil {
		return 0, false, errGhUnavailable
	}
	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return 0, false, fmt.Errorf("gh pr list: parse: %w", err)
	}
	if len(prs) == 0 {
		return 0, false, nil
	}
	return prs[0].Number, true, nil
}

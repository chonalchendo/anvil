package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

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
	ghPRStateByURLFn  = ghPRStateByURLReal
	gitBranchExistsFn = gitBranchExistsReal
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
			findings, err := runDoctor(v)
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

// runDoctor checks all four stale-lifecycle shapes and returns the findings.
// Best-effort: a check that cannot shell out (gh missing, no network) skips
// rather than aborts so doctor is always usable in offline environments.
func runDoctor(v *core.Vault) ([]doctorFinding, error) {
	var findings []doctorFinding

	issuePaths, err := collectArtifactPaths(v.Root, core.TypeIssue)
	if err != nil {
		return nil, fmt.Errorf("reading issues: %w", err)
	}

	worktrees, _ := gitWorktreeListFn() // best-effort

	for _, p := range issuePaths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			continue // skip unreadable
		}
		status, _ := a.FrontMatter["status"].(string)
		if status != "in-progress" {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(p), ".md")

		// Shape 1: merged-PR issue.
		if f := checkMergedPR(id, a); f != nil {
			findings = append(findings, *f)
			continue
		}

		// Shape 2: dead claim — no open PR, no live worktree.
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
		if f := checkFinishedMilestone(v, p, a, issuePaths); f != nil {
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

// checkMergedPR returns a finding when the issue's external_links contains a
// GitHub PR URL whose state is MERGED. Returns nil when no PR link is present,
// gh is unavailable, or the PR is not merged.
func checkMergedPR(id string, a *core.Artifact) *doctorFinding {
	links, _ := a.FrontMatter["external_links"].([]any)
	for _, raw := range links {
		url, ok := raw.(string)
		if !ok || !strings.Contains(url, "github.com") {
			continue
		}
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
// when a worktree is alive, or when an open PR exists.
func checkDeadClaim(v *core.Vault, id string, a *core.Artifact, worktrees map[string]worktreeInfo) *doctorFinding {
	claimSession, _ := a.FrontMatter["claim_session"].(string)
	if claimSession == "" {
		return nil // not session-claimed; not a dead claim
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
	links, _ := a.FrontMatter["external_links"].([]any)
	for _, raw := range links {
		url, ok := raw.(string)
		if !ok || !strings.Contains(url, "github.com") {
			continue
		}
		state, err := ghPRStateByURLFn(url)
		if err != nil {
			return nil // can't confirm — don't false-positive
		}
		if state == "OPEN" {
			return nil
		}
	}
	return &doctorFinding{
		Kind:     "dead-claim",
		ID:       id,
		Evidence: fmt.Sprintf("in-progress with claim_session %s but no live worktree or open PR", claimSession),
		Fix:      fmt.Sprintf("anvil transition issue %s open", id),
	}
}

// checkFinishedMilestone returns a finding when an in-progress milestone has
// every child issue resolved or abandoned. Returns nil for milestones that are
// not in-progress, have open work, or have no children.
func checkFinishedMilestone(_ *core.Vault, msPath string, a *core.Artifact, issuePaths []string) *doctorFinding {
	status, _ := a.FrontMatter["status"].(string)
	if status != "in-progress" {
		return nil
	}
	msID := strings.TrimSuffix(filepath.Base(msPath), ".md")
	msSlug := msID
	// milestoneSlug strips the [[milestone.]] wikilink syntax; here we need the
	// bare slug so issue milestone fields can match against it.
	hasChild := false
	for _, p := range issuePaths {
		child, err := core.LoadArtifact(p)
		if err != nil {
			continue
		}
		if milestoneSlug(child.FrontMatter["milestone"]) != msSlug {
			continue
		}
		hasChild = true
		cs, _ := child.FrontMatter["status"].(string)
		if cs == "open" || cs == "in-progress" {
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
		Fix:      fmt.Sprintf("anvil transition milestone %s done", msSlug),
	}
}

// checkOrphanWorktree returns a finding when an anvil/ worktree's branch no
// longer exists on origin (was merged and deleted). Returns nil for
// non-anvil branches or when the branch still exists on origin.
func checkOrphanWorktree(branch, path string) *doctorFinding {
	if !strings.HasPrefix(branch, "anvil/") {
		return nil
	}
	exists, err := gitBranchExistsFn(branch)
	if err != nil || exists {
		return nil // error = can't tell; skip rather than false-positive
	}
	return &doctorFinding{
		Kind:     "orphan-worktree",
		ID:       branch,
		Evidence: fmt.Sprintf("branch %s not found on origin; worktree at %s may be stale", branch, path),
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

// gitBranchExistsReal checks whether branch exists on origin via
// `git ls-remote --heads origin <branch>`. Returns (false, nil) when the
// branch is absent, (true, nil) when present, and ("", err) when git fails.
func gitBranchExistsReal(branch string) (bool, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return false, err
	}
	out, err := exec.Command("git", "ls-remote", "--heads", "origin", branch).Output() //nolint:gosec // branch is a package-controlled slug, never user input
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

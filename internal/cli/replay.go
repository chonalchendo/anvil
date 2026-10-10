package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

func newReplayCmd() *cobra.Command {
	var flagWorktree string
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "replay <issue-id>",
		Short: "Cut a worktree at the base a resolved issue's merged PR started from",
		Long: "Cut a worktree on a replay/<slug> branch at the first parent of the merge commit of the resolved issue's merged PR, " +
			"and print its path. Changes no frontmatter. Grade the replay from inside it with anvil verify <issue-id> --replay --tokens <n>.",
		Example: "  anvil replay issue.anvil.0387.create-update-refuses-to-change-an --worktree /tmp/replay",
		Args:    namedArgs("anvil replay <issue-id>", []string{"<issue-id>"}, 1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			id, path, err := core.ResolveArtifact(v, core.TypeIssue, args[0])
			if err != nil {
				return err
			}
			a, err := loadIssueForVerify(path, id, args[0])
			if err != nil {
				return err
			}
			wt, base, err := cutReplayWorktree(cmd, a, id, flagWorktree)
			if err != nil {
				return printAndReturn(cmd, err)
			}
			if flagJSON {
				b, _ := json.Marshal(map[string]string{"worktree": wt, "base": base})
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), wt)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorktree, "worktree", "", "worktree path (default: beside the conventional path, as replay-<slug>)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "emit JSON: {worktree, base}")
	return cmd
}

// ghPRViewByURLFn looks a PR up by its full url, so the lookup never depends
// on the current directory's remote.
var ghPRViewByURLFn = ghPRViewByURLReal

func ghPRViewByURLReal(url, fields string) ([]byte, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errGhUnavailable
	}
	out, err := exec.Command("gh", "pr", "view", url, "--json", fields).Output() //nolint:gosec // url is an external_links entry, one argv element
	if err != nil {
		// Like ghPRListReal: any exec failure means GitHub cannot be asked now.
		return nil, errGhUnavailable
	}
	return out, nil
}

// replayBase is the first parent of the merge commit of the issue's merged PR.
func replayBase(a *core.Artifact, id, repoDir, rerun string) (string, error) {
	if s, _ := a.FrontMatter["status"].(string); s != "resolved" {
		return "", errfmt.NewStructured("replay_not_resolved").Set("issue", id).
			Set("message", id+" is "+s+"; only a resolved issue replays").
			Set("fix_hint", "replay an issue after it lands; check its status with anvil show issue "+id)
	}
	prURL := ""
	links, _ := a.FrontMatter["external_links"].([]any)
	for _, raw := range links {
		if url, ok := raw.(string); ok && prURLNumber.MatchString(url) {
			prURL = url
		}
	}
	noPR := func(why, hint string) error {
		return errfmt.NewStructured("replay_no_merged_pr").Set("issue", id).Set("message", why).Set("fix_hint", hint)
	}
	if prURL == "" {
		return "", noPR(id+" has no PR url in external_links", "an issue that landed without a PR cannot replay")
	}
	raw, err := ghPRViewByURLFn(prURL, "state,mergeCommit")
	var view struct {
		State       string `json:"state"`
		MergeCommit struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if errors.Is(err, errGhUnavailable) {
		return "", errfmt.NewStructured("replay_gh_unavailable").Set("issue", id).
			Set("message", "gh is missing, unauthenticated or has no remote; the PR state cannot be read").
			Set("fix_hint", "install and authenticate gh, then re-run "+rerun)
	}
	if err == nil {
		err = json.Unmarshal(raw, &view)
	}
	if err != nil {
		return "", noPR(fmt.Sprintf("gh pr view %s failed: %v", prURL, err), "check gh auth status and the url in external_links, then re-run "+rerun)
	}
	if view.State != "MERGED" || view.MergeCommit.Oid == "" {
		return "", noPR(fmt.Sprintf("%s is %s, not merged", prURL, view.State), "replay needs a merged PR; land it first")
	}
	base, err := gitRevParseFn(repoDir, view.MergeCommit.Oid+"^1")
	if err != nil {
		return "", errfmt.NewStructured("replay_base_unresolved").Set("issue", id).
			Set("message", err.Error()).Set("fix_hint", "git fetch origin, then re-run "+rerun)
	}
	return base, nil
}

// cutReplayWorktree provisions the worktree like a claim's cut, but at base
// rather than origin/HEAD. A failed hook leaves the worktree for the caller to see.
func cutReplayWorktree(cmd *cobra.Command, a *core.Artifact, id, override string) (string, string, error) {
	project := projectFromArtifact(a, id)
	slug := slugFromIssueID(id)
	repoDir, err := resolveProjectRepoFn(project)
	if err != nil {
		return "", "", errfmt.NewStructured("cut_worktree_repo_unresolved").Set("project", project).Set("error", err.Error()).
			Set("fix_hint", "run anvil replay from a project whose repo anvil can resolve")
	}
	if ferr := gitFetchOriginFn(repoDir); ferr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: git fetch origin failed (%v); using local objects\n", ferr)
	}
	base, err := replayBase(a, id, repoDir, "anvil replay "+id)
	if err != nil {
		return "", "", err
	}
	wt := override
	if wt == "" {
		if wt, err = defaultWorktreePath(project, "replay-"+slug); err != nil {
			return "", "", errfmt.NewStructured("cut_worktree_path_failed").Set("error", err.Error()).
				Set("fix_hint", "pass --worktree <path>")
		}
	}
	if wt, err = filepath.Abs(wt); err != nil {
		return "", "", errfmt.NewStructured("cut_worktree_path_failed").Set("error", err.Error()).
			Set("fix_hint", "pass an absolute --worktree <path>")
	}
	branch := "replay/" + slug
	if wts, _ := gitWorktreeListFn(repoDir); wts != nil {
		if live, ok := wts[branch]; ok {
			return "", "", errfmt.NewStructured("replay_worktree_exists").Set("issue", id).Set("path", live.path).
				Set("fix_hint", "remove it with git worktree remove "+live.path+", then re-run anvil replay "+id)
		}
	}
	// A replay branch outlives its removed worktree and holds nothing worth keeping.
	if gitLocalBranchExistsFn(repoDir, branch) {
		if err := gitDeleteLocalBranchFn(repoDir, branch); err != nil {
			return "", "", errfmt.NewStructured("cut_worktree_failed").Set("branch", branch).Set("error", err.Error()).
				Set("fix_hint", "fix the git error in error, then re-run anvil replay "+id)
		}
	}
	if err := gitWorktreeAddFn(repoDir, wt, branch, base); err != nil {
		return "", "", errfmt.NewStructured("cut_worktree_failed").Set("path", wt).Set("branch", branch).Set("error", err.Error()).
			Set("fix_hint", "fix the git error in error, then re-run anvil replay "+id)
	}
	if err := provisionCheckout(repoDir, wt); err != nil {
		return "", "", errfmt.NewStructured("replay_provision_failed").Set("path", wt).Set("error", err.Error()).
			Set("fix_hint", "fix the carry list or worktree hook named in error, remove "+wt+" with git worktree remove, then re-run anvil replay "+id)
	}
	return wt, base, nil
}

// buildSha7 is the build stamp's sha, "-dirty" kept, "unknown" when the stamp carries none.
func buildSha7() string {
	if m := versionSha7.FindStringSubmatch(resolveVersion()); m != nil {
		return m[1]
	}
	return "unknown"
}

// versionSha7 matches the dev-<sha7>[-dirty] stamp the build injects.
var versionSha7 = regexp.MustCompile(`(?:^|-)([0-9a-f]{7}(?:-dirty)?)$`)

// replayDiff counts the changed lines and files between base and HEAD. Binary
// files count as a file and no lines. Uncommitted work is not counted.
func replayDiff(base string) (lines, files int, err error) {
	out, err := exec.Command("git", "diff", "--numstat", base, "HEAD").Output() //nolint:gosec // base is a sha from git
	if err != nil {
		return 0, 0, fmt.Errorf("git diff: %w", err)
	}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(l)
		if len(f) < 3 {
			continue
		}
		files++
		add, _ := strconv.Atoi(f[0])
		del, _ := strconv.Atoi(f[1])
		lines += add + del
	}
	return lines, files, nil
}

// replaySection renders the Replay section. The landed cost sits beside the replay's.
func replaySection(rec verifyRecord, lines, files, tokens int, landed *costFields) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Replay — %s @ %s\n\n", time.Now().UTC().Format("2006-01-02"), buildSha7())
	fmt.Fprintf(&b, "- commit: %s\n", rec.Commit)
	fmt.Fprintf(&b, "- verdict: %s (%d checks)\n", rec.Verdict, rec.Checks)
	for _, f := range rec.Failed {
		if f.Exit != nil {
			fmt.Fprintf(&b, "- failed: %s (exit %d)\n", f.Check, *f.Exit)
		} else {
			fmt.Fprintf(&b, "- failed: %s\n", f.Check)
		}
	}
	fmt.Fprintf(&b, "- replay: diff %d, files %d, tokens %d\n", lines, files, tokens)
	if strings.HasSuffix(rec.Commit, "-dirty") {
		b.WriteString("- dirty: the tree had uncommitted changes; the diff counts commits only\n")
	}
	if landed != nil {
		fmt.Fprintf(&b, "- landed: diff %d, files %d, tokens %d\n", landed.Diff, landed.Files, landed.Tokens)
	} else {
		b.WriteString("- landed: no cost record\n")
	}
	return b.String()
}

// appendReplay reloads the issue and appends the section through the same
// write path as anvil append; no frontmatter changes beyond `updated`.
func appendReplay(cmd *cobra.Command, v *core.Vault, path, id, arg, base string, rec verifyRecord, tokens int, asJSON bool) error {
	lines, files, err := replayDiff(base)
	if err != nil {
		return errfmt.NewStructured("replay_diff_failed").Set("message", err.Error()).
			Set("fix_hint", "run anvil verify "+id+" --replay from inside the worktree anvil replay cut")
	}
	a, err := loadIssueForVerify(path, id, arg)
	if err != nil {
		return err
	}
	res, err := appendBodyCore(cmd, v, core.TypeIssue, path, id, a, replaySection(rec, lines, files, tokens, costFromFrontMatter(a.FrontMatter)))
	if err != nil {
		return err
	}
	if res.blocked {
		return emitValidationErrors(cmd, asJSON, res.failures)
	}
	return nil
}

// verifyReplay runs the blocks here and records the result as a Replay section
// only: the landed verified_* and outcome_* fields stay as they were. It runs
// only on the replay/<slug> branch, and diffs against the base the cut used.
func verifyReplay(cmd *cobra.Command, v *core.Vault, a *core.Artifact, path, id, arg string, tokens int, asJSON bool) error {
	fail := func(err error) error {
		var se *errfmt.Structured
		if errors.As(err, &se) {
			return printAndReturn(cmd, err)
		}
		return err
	}
	branch, _ := exec.Command("git", "branch", "--show-current").Output()
	want := "replay/" + slugFromIssueID(id)
	if got := strings.TrimSpace(string(branch)); got != want {
		return fail(errfmt.NewStructured("replay_not_replay_worktree").Set("issue", id).
			Set("message", fmt.Sprintf("the current branch is %q, not %s", got, want)).
			Set("fix_hint", "anvil replay "+id+", then run this verb from the worktree it prints"))
	}
	repoDir, err := resolveProjectRepoFn(projectFromArtifact(a, id))
	if err != nil {
		return fail(errfmt.NewStructured("cut_worktree_repo_unresolved").Set("error", err.Error()).
			Set("fix_hint", "run anvil verify --replay for a project whose repo anvil can resolve"))
	}
	base, err := replayBase(a, id, repoDir, "anvil verify "+id+" --replay --tokens "+strconv.Itoa(tokens))
	if err != nil {
		return fail(err)
	}
	rec, err := runVerification(cmd.ErrOrStderr(), a.Body, "")
	if err != nil {
		return fail(err)
	}
	if err := appendReplay(cmd, v, path, id, arg, base, rec, tokens, asJSON); err != nil {
		return fail(err)
	}
	if asJSON {
		b, _ := json.Marshal(rec)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
	}
	if rec.Verdict != "pass" {
		return fmt.Errorf("replay verification %s: %d of %d check(s) failed", rec.Verdict, len(rec.Failed), rec.Checks)
	}
	return nil
}

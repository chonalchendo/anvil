package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

func newReplayCmd() *cobra.Command {
	var flagWorktree string
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
			wt, err := cutReplayWorktree(cmd, a, id, flagWorktree)
			if err != nil {
				return printAndReturn(cmd, err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), wt)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorktree, "worktree", "", "worktree path (default: beside the conventional path, as replay-<slug>)")
	return cmd
}

// replayBase is the first parent of the merge commit of the issue's merged PR.
func replayBase(a *core.Artifact, id, repoDir string) (string, error) {
	if s, _ := a.FrontMatter["status"].(string); s != "resolved" {
		return "", errfmt.NewStructured("replay_not_resolved").Set("issue", id).
			Set("message", id+" is "+s+"; only a resolved issue replays")
	}
	num := 0
	links, _ := a.FrontMatter["external_links"].([]any)
	for _, raw := range links {
		if url, ok := raw.(string); ok {
			if m := prURLNumber.FindStringSubmatch(url); m != nil {
				fmt.Sscan(m[1], &num)
			}
		}
	}
	noPR := func(why string) error {
		return errfmt.NewStructured("replay_no_merged_pr").Set("issue", id).Set("message", why)
	}
	if num == 0 {
		return "", noPR(id + " has no PR url in external_links")
	}
	raw, err := ghPRViewJSONFn(num, "state,mergeCommit")
	var view struct {
		State       string `json:"state"`
		MergeCommit struct {
			Oid string `json:"oid"`
		} `json:"mergeCommit"`
	}
	if err == nil {
		err = json.Unmarshal(raw, &view)
	}
	if err != nil {
		return "", noPR(fmt.Sprintf("gh pr view %d failed: %v", num, err))
	}
	if view.State != "MERGED" || view.MergeCommit.Oid == "" {
		return "", noPR(fmt.Sprintf("PR %d is %s, not merged", num, view.State))
	}
	base, err := gitRevParseFn(repoDir, view.MergeCommit.Oid+"^1")
	if err != nil {
		return "", errfmt.NewStructured("replay_base_unresolved").Set("issue", id).
			Set("message", err.Error()).Set("fix_hint", "git fetch origin, then re-run anvil replay "+id)
	}
	return base, nil
}

// cutReplayWorktree provisions the worktree like a claim's cut, but at base
// rather than origin/HEAD. A failed hook leaves the worktree for the caller to see.
func cutReplayWorktree(cmd *cobra.Command, a *core.Artifact, id, override string) (string, error) {
	project := projectFromArtifact(a, id)
	slug := slugFromIssueID(id)
	repoDir, err := resolveProjectRepoFn(project)
	if err != nil {
		return "", errfmt.NewStructured("cut_worktree_repo_unresolved").Set("project", project).Set("error", err.Error())
	}
	if ferr := gitFetchOriginFn(repoDir); ferr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: git fetch origin failed (%v); using local objects\n", ferr)
	}
	base, err := replayBase(a, id, repoDir)
	if err != nil {
		return "", err
	}
	wt := override
	if wt == "" {
		if wt, err = defaultWorktreePath(project, "replay-"+slug); err != nil {
			return "", errfmt.NewStructured("cut_worktree_path_failed").Set("error", err.Error())
		}
	}
	if wt, err = filepath.Abs(wt); err != nil {
		return "", errfmt.NewStructured("cut_worktree_path_failed").Set("error", err.Error())
	}
	carry, err := checkCarryDeclarations(repoDir)
	if err != nil {
		return "", err
	}
	if err := gitWorktreeAddFn(repoDir, wt, "replay/"+slug, base); err != nil {
		return "", errfmt.NewStructured("cut_worktree_failed").Set("path", wt).Set("branch", "replay/"+slug).Set("error", err.Error())
	}
	if err := copyCarryFiles(repoDir, wt, carry); err != nil {
		return "", err
	}
	if err := runWorktreeHookFn(repoDir, wt); err != nil {
		return "", err
	}
	return wt, nil
}

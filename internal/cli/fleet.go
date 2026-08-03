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

// fleetRow is one issue's mechanical snapshot. Fields with no data are
// emitted as zero values (empty string, 0, false) so an orchestrator can
// branch on presence without coercion.
type fleetRow struct {
	ID                 string `json:"id"`
	Owner              string `json:"owner"`
	Worktree           string `json:"worktree,omitempty"`
	Branch             string `json:"branch,omitempty"`
	HeadSHA            string `json:"head_sha,omitempty"`
	PushState          string `json:"push_state,omitempty"`
	PRNumber           int    `json:"pr_number,omitempty"`
	PRURL              string `json:"pr_url,omitempty"`
	PRMergeable        string `json:"pr_mergeable,omitempty"`
	CIConclusion       string `json:"ci_conclusion,omitempty"`
	ReviewerState      string `json:"reviewer_state,omitempty"`
	OpenInlineComments int    `json:"open_inline_comments"`
	Note               string `json:"note,omitempty"`
}

// fleetEnvelope wraps the rows so consumers can pin on `count` without
// re-len()-ing the array.
type fleetEnvelope struct {
	Count int        `json:"count"`
	Rows  []fleetRow `json:"rows"`
}

// Indirection points for tests. Real implementations are below; tests swap.
var (
	gitWorktreeListFn = gitWorktreeListReal
	gitPushStateFn    = gitPushStateReal
	ghPRViewFn        = ghPRViewReal
	ghPRCommentsFn    = ghPRCommentsReal
)

func newFleetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet",
		Short: "Inspect in-flight issue worktrees + PRs",
	}
	cmd.AddCommand(newFleetStatusCmd())
	cmd.AddCommand(newFleetScopeAuditCmd())
	return cmd
}

// newFleetScopeAuditCmd returns a command that compares a branch's changed
// files against an issue's declared-files allowlist and names any file outside
// the allowlist. One out-of-scope file per line; "scope: clean" when none.
// Orchestrators can pipe through grep to detect violations.
func newFleetScopeAuditCmd() *cobra.Command {
	var declared, changed string
	cmd := &cobra.Command{
		Use:          "scope-audit",
		Short:        "Flag changed files that fall outside an issue's declared-files allowlist",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			declaredEntries := splitCSV(declared)
			if err := validateDeclaredGlobs(declaredEntries); err != nil {
				return err
			}
			outside := scopeViolations(declaredEntries, splitLiteralCSV(changed))
			w := cmd.OutOrStdout()
			if len(outside) == 0 {
				fmt.Fprintln(w, "scope: clean")
				return nil
			}
			for _, f := range outside {
				fmt.Fprintln(w, f)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&declared, "declared", "", "comma-separated declared-files allowlist; entries are globs — *, ?, and brace alternation {a,b} (e.g. src/{a,b}_*.sql). ** is not supported; * does not cross /; an entry naming a directory (`pkg` or `pkg/`) covers everything beneath it")
	cmd.Flags().StringVar(&changed, "changed", "", "comma-separated changed files on the branch; entries are literal paths, not globs")
	_ = cmd.MarkFlagRequired("declared")
	_ = cmd.MarkFlagRequired("changed")
	return cmd
}

func newFleetStatusCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "One-row-per-in-progress-issue snapshot of worktree + PR + CI state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			rows, err := buildFleetRows(v)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				return enc.Encode(fleetEnvelope{Count: len(rows), Rows: rows})
			}
			return emitFleetTable(cmd, rows)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON envelope")
	return cmd
}

// buildFleetRows discovers every in-progress issue, matches it to a worktree
// by branch candidate, and gathers per-worktree git + PR + CI state. Each
// shell-out is best-effort: a missing gh, detached HEAD, or no upstream
// downgrades to a blank field with a `note`, never an aborting error. The
// verb is mechanical glue — partial visibility beats no visibility.
func buildFleetRows(v *core.Vault) ([]fleetRow, error) {
	paths, err := collectArtifactPaths(v.Root, core.TypeIssue)
	if err != nil {
		return nil, err
	}

	// The vault is shared across projects, so an issue's worktree lives in
	// that issue's own project repo (`~/Development/<project>`), not
	// necessarily the repo this command happens to run from. Resolve and
	// cache one `git worktree list` per project rather than assuming cwd.
	worktreesByRepo := map[string]map[string]worktreeInfo{}
	repoByProject := map[string]string{} // "" = unresolved, ambient-repo fallback
	// Returns the project's worktrees plus whether its repo actually resolved;
	// an unresolved project degrades to the ambient repo, which the caller
	// surfaces on the row rather than reporting a bare "no matching worktree".
	worktreesForProject := func(project string) (map[string]worktreeInfo, bool) {
		repoDir, cached := repoByProject[project]
		if !cached {
			dir, err := resolveProjectRepoFn(project)
			if err != nil {
				dir = "" // best-effort: fall back to the caller's ambient repo
			}
			repoDir = dir
			repoByProject[project] = repoDir
		}
		if wts, ok := worktreesByRepo[repoDir]; ok {
			return wts, repoDir != ""
		}
		wts, _ := gitWorktreeListFn(repoDir) // best-effort; empty map is fine
		worktreesByRepo[repoDir] = wts
		return wts, repoDir != ""
	}

	var rows []fleetRow
	for _, p := range paths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			continue
		}
		if s, _ := a.FrontMatter["status"].(string); s != "in-progress" {
			continue
		}
		id := core.CanonicalID(core.TypeIssue, strings.TrimSuffix(filepath.Base(p), ".md"))
		owner, _ := a.FrontMatter["owner"].(string)

		row := fleetRow{ID: id, Owner: owner}
		project := projectFromArtifact(a, id)
		worktrees, repoResolved := worktreesForProject(project)
		matched := false
		apply := func(b string, wt worktreeInfo) {
			row.Worktree, row.Branch, row.HeadSHA = wt.path, b, wt.headSHA
			matched = true
		}
		// Projects brand worktree branches `<project>/<slug>` — `anvil/`,
		// `burgh/`, `mentat/` — so match on the branch slug (the segment
		// after the last "/") against the id-derived and linked-plan slugs,
		// whatever the prefix. >1 worktree on a slug = ambiguous, skip.
		if b, wt, ok := genericSlugWorktree(worktrees, fleetCandidateSlugs(v, id)); ok {
			apply(b, wt)
		}
		// Fallback: orchestrators frequently choose a divergent short slug
		// (e.g. issue `<proj>.long-descriptive-id` → branch
		// `<proj>/short-slug`). Match when exactly one worktree's branch
		// slug is a substring of the issue slug. >1 match = ambiguous,
		// leave unmatched rather than guess.
		if !matched {
			if b, wt, ok := uniqueSubstringWorktree(worktrees, id); ok {
				apply(b, wt)
			}
		}
		switch {
		case matched:
			fillRowFromWorktree(&row)
		case !repoResolved:
			row.Note = fmt.Sprintf("project repo not found (expected ~/Development/%s); matched against ambient repo", project)
		default:
			row.Note = "no matching worktree"
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func fillRowFromWorktree(row *fleetRow) {
	if state, err := gitPushStateFn(row.Worktree); err == nil {
		row.PushState = state
	}
	pr, err := ghPRViewFn(row.Worktree, row.Branch)
	if err != nil || pr == nil {
		return
	}
	row.PRNumber = pr.Number
	row.PRURL = pr.URL
	row.PRMergeable = pr.Mergeable
	row.CIConclusion = pr.CIConclusion
	row.ReviewerState = pr.ReviewDecision
	if pr.Number > 0 {
		if n, err := ghPRCommentsFn(row.Worktree, pr.Number); err == nil {
			row.OpenInlineComments = n
		}
	}
}

func emitFleetTable(cmd *cobra.Command, rows []fleetRow) error {
	w := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(w, "no in-progress issues")
		return nil
	}
	fmt.Fprintln(w, "ID\tOWNER\tBRANCH\tPR\tMERGEABLE\tCI\tREVIEW\tINLINE")
	for _, r := range rows {
		pr := "—"
		if r.PRNumber > 0 {
			pr = fmt.Sprintf("#%d", r.PRNumber)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
			r.ID, dashIfEmpty(r.Owner), dashIfEmpty(r.Branch),
			pr, dashIfEmpty(r.PRMergeable), dashIfEmpty(r.CIConclusion),
			dashIfEmpty(r.ReviewerState), r.OpenInlineComments,
		)
	}
	return nil
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// gitPushStateReal classifies the worktree's branch vs its upstream:
//   - "no-upstream" — no tracking branch (never pushed)
//   - "in-sync" — ahead 0, behind 0
//   - "ahead-N" / "behind-N" / "diverged-A/B" otherwise
//
// Anything we can't classify (git error, missing repo) returns ("", err).
func gitPushStateReal(dir string) (string, error) {
	upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "@{u}")
	upstreamCmd.Dir = dir
	// No-upstream is a normal state on a freshly cut worktree, not an error.
	if err := upstreamCmd.Run(); err != nil {
		return "no-upstream", nil //nolint:nilerr // intentional: missing upstream is a status, not a failure
	}
	countCmd := exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{u}")
	countCmd.Dir = dir
	out, err := countCmd.Output()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 2 {
		return "", fmt.Errorf("unexpected rev-list output: %q", string(out))
	}
	ahead, behind := fields[0], fields[1]
	switch {
	case ahead == "0" && behind == "0":
		return "in-sync", nil
	case ahead != "0" && behind == "0":
		return "ahead-" + ahead, nil
	case ahead == "0" && behind != "0":
		return "behind-" + behind, nil
	default:
		return "diverged-" + ahead + "/" + behind, nil
	}
}

// ghStatusCheck is one entry in `gh pr view --json statusCheckRollup`.
//
// The rollup mixes two shapes. GitHub Actions writes Check Runs, which carry
// Status + Conclusion. CircleCI and other legacy integrations write Commit
// Statuses (StatusContext), which carry only State. Reading Conclusion alone
// classifies every StatusContext as pending forever — a green CircleCI run
// has no Conclusion at all.
type ghStatusCheck struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
	State      string `json:"state"`
}

// ghPRSnapshot is the JSON shape gh returns for the fields we ask for.
type ghPRSnapshot struct {
	Number          int             `json:"number"`
	URL             string          `json:"url"`
	Mergeable       string          `json:"mergeable"`
	ReviewDecision  string          `json:"reviewDecision"`
	StatusCheckRoll []ghStatusCheck `json:"statusCheckRollup"`
	// Hoisted derived field — not part of the raw gh JSON.
	CIConclusion string `json:"-"`
}

// ghPRViewReal queries gh for the open PR on `branch`. Returns (nil, nil)
// when no PR exists. The CI conclusion is rolled up across statusCheckRollup
// entries — both Check Runs and Commit Statuses — by rollupCI.
func ghPRViewReal(dir, branch string) (*ghPRSnapshot, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "view", branch, //nolint:gosec // binary path resolved from trusted sources; not user input
		"--json", "number,url,mergeable,reviewDecision,statusCheckRollup")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// gh exits 1 when no PR exists — indistinguishable here from auth
		// failure without parsing stderr. Treat as "no PR"; downstream
		// fields stay blank. A genuine auth issue surfaces on the next
		// shell-out (gh api comments).
		return nil, nil //nolint:nilerr // fail-open: no PR is the common case
	}
	var pr ghPRSnapshot
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, fmt.Errorf("gh pr view: parse: %w", err)
	}
	pr.CIConclusion = rollupCI(pr.StatusCheckRoll)
	return &pr, nil
}

// rollupCI reduces a statusCheckRollup to one verdict: failure > pending >
// success > "" (nothing classifiable). Each arm reads the Check Run fields
// and the Commit Status field, because the rollup interleaves both shapes.
// Mirrors the jq reducer in anvil/skills/completing-issue/scripts/wait-for-pr.sh;
// the two must classify every state identically (asserted by TestWaitForPRReducer).
//
// The pending arm is the complement of "finished", not a whitelist: a Check
// Run status is any of QUEUED/IN_PROGRESS/WAITING/PENDING/REQUESTED/COMPLETED,
// so anything non-empty that is not COMPLETED is still running. Enumerating
// the running states instead leaks WAITING and REQUESTED into "" (no checks),
// which `anvil fleet status` renders identically to "no CI configured".
// NEUTRAL and SKIPPED are green — `gh pr checks` passes both, so a
// path-filtered workflow that skipped must not read as no-CI.
func rollupCI(rolls []ghStatusCheck) string {
	if len(rolls) == 0 {
		return ""
	}
	hasFailure, hasPending, hasSuccess := false, false, false
	for _, r := range rolls {
		switch {
		case r.Conclusion == "FAILURE" || r.Conclusion == "CANCELLED" ||
			r.Conclusion == "TIMED_OUT" || r.Conclusion == "STARTUP_FAILURE" ||
			r.Conclusion == "ACTION_REQUIRED" || r.Conclusion == "STALE" ||
			r.State == "FAILURE" || r.State == "ERROR":
			hasFailure = true
		case (r.Status != "" && r.Status != "COMPLETED") ||
			r.State == "PENDING" || r.State == "EXPECTED":
			hasPending = true
		case r.Conclusion == "SUCCESS" || r.Conclusion == "NEUTRAL" ||
			r.Conclusion == "SKIPPED" || r.State == "SUCCESS":
			hasSuccess = true
		}
	}
	switch {
	case hasFailure:
		return "failure"
	case hasPending:
		return "pending"
	case hasSuccess:
		return "success"
	default:
		return ""
	}
}

// ghPRCommentsReal counts open inline review comments (`gh api repos/.../pulls/<n>/comments`).
// "Open" = not resolved at the API level: the REST endpoint only returns
// unresolved review comments by default on the v3 list, so a raw len() is
// the count. Returns 0 on any gh failure — partial visibility, not abort.
func ghPRCommentsReal(dir string, number int) (int, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return 0, errGhUnavailable
	}
	// Use the per-PR comments endpoint via `gh pr view --json comments` is
	// the *issue-comments* feed, not inline review comments. The REST path
	// `/repos/{owner}/{repo}/pulls/{n}/comments` is the inline-review feed.
	cmd := exec.Command("gh", "api", //nolint:gosec // binary path resolved from trusted sources; not user input
		fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/comments", number),
		"--jq", "length",
	)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return 0, nil //nolint:nilerr // partial visibility: a gh failure shouldn't fail the whole verb
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n); err != nil {
		return 0, nil //nolint:nilerr // unparseable count is a rendering issue, not a fatal
	}
	return n, nil
}

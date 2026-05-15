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

	worktrees, _ := gitWorktreeListFn() // best-effort; empty map is fine

	var rows []fleetRow
	for _, p := range paths {
		a, err := core.LoadArtifact(p)
		if err != nil {
			continue
		}
		if s, _ := a.FrontMatter["status"].(string); s != "in-progress" {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(p), ".md")
		owner, _ := a.FrontMatter["owner"].(string)

		row := fleetRow{ID: id, Owner: owner}
		branches := fleetCandidateBranches(v, id)
		matched := false
		for _, b := range branches {
			if wt, ok := worktrees[b]; ok {
				row.Worktree = wt.path
				row.Branch = b
				row.HeadSHA = wt.headSHA
				matched = true
				break
			}
		}
		// Fallback: orchestrators frequently choose a divergent short slug
		// (e.g. issue `<proj>.long-descriptive-id` → branch
		// `anvil/short-slug`). Match when exactly one worktree's branch
		// slug is a substring of the issue slug. >1 match = ambiguous,
		// leave unmatched rather than guess.
		if !matched {
			if b, wt, ok := uniqueSubstringWorktree(worktrees, id); ok {
				row.Worktree = wt.path
				row.Branch = b
				row.HeadSHA = wt.headSHA
				matched = true
			}
		}
		if matched {
			fillRowFromWorktree(&row)
		} else {
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

// fleetCandidateBranches returns the `anvil/<slug>` branches plausibly
// hosting an issue's worktree. Unlike candidateBranchesForIssue (used by
// `transition resolved`), we deliberately exclude the current-branch
// fallback — fleet enumerates many issues at once, so reusing the caller's
// branch as a wildcard would cross-pollute every row with the same match.
// id-derived slug + linked-plan slugs is enough; if neither matches, the
// row reports "no matching worktree" and an orchestrator can investigate.
func fleetCandidateBranches(v *core.Vault, id string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(slug string) {
		if slug == "" {
			return
		}
		b := "anvil/" + slug
		if seen[b] {
			return
		}
		seen[b] = true
		out = append(out, b)
	}
	if dot := strings.IndexByte(id, '.'); dot >= 0 && dot+1 < len(id) {
		add(id[dot+1:])
	}
	slugs, _ := linkedPlanSlugs(v, id)
	for _, s := range slugs {
		add(s)
	}
	return out
}

// uniqueSubstringWorktree returns the lone worktree whose branch slug is a
// substring of the issue id's slug, or ok=false if zero or two-plus match.
// The single-match rule keeps the fallback honest: ambiguity yields an
// unmatched row, not a wrong row.
func uniqueSubstringWorktree(worktrees map[string]worktreeInfo, id string) (string, worktreeInfo, bool) {
	dot := strings.IndexByte(id, '.')
	if dot < 0 || dot+1 >= len(id) {
		return "", worktreeInfo{}, false
	}
	issueSlug := id[dot+1:]
	const prefix = "anvil/"
	var hitBranch string
	var hitInfo worktreeInfo
	count := 0
	for b, wt := range worktrees {
		if !strings.HasPrefix(b, prefix) {
			continue
		}
		branchSlug := strings.TrimPrefix(b, prefix)
		if branchSlug == "" || branchSlug == "master" {
			continue
		}
		if strings.Contains(issueSlug, branchSlug) {
			hitBranch, hitInfo = b, wt
			count++
		}
	}
	if count == 1 {
		return hitBranch, hitInfo, true
	}
	return "", worktreeInfo{}, false
}

// worktreeInfo is the subset of `git worktree list --porcelain` we use.
type worktreeInfo struct {
	path    string
	headSHA string
}

// gitWorktreeListReal parses `git worktree list --porcelain` and returns a
// map keyed by branch name ("anvil/<slug>"). Worktrees without a branch (e.g.
// detached HEAD) are skipped because the fleet workflow assumes each task
// runs on its own branch. Bare worktrees are similarly skipped.
func gitWorktreeListReal() (map[string]worktreeInfo, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, err
	}
	out, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, err
	}
	return parseWorktreePorcelain(string(out)), nil
}

// parseWorktreePorcelain is split out so tests can feed canned fixtures.
// The porcelain format is record-per-blank-line, each record a series of
// `<key> <value>` lines starting with `worktree <path>`.
func parseWorktreePorcelain(in string) map[string]worktreeInfo {
	out := map[string]worktreeInfo{}
	var cur worktreeInfo
	var curBranch string
	flush := func() {
		if curBranch != "" && cur.path != "" {
			out[curBranch] = cur
		}
		cur = worktreeInfo{}
		curBranch = ""
	}
	for _, line := range strings.Split(in, "\n") {
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.headSHA = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			curBranch = strings.TrimPrefix(ref, "refs/heads/")
		}
	}
	flush()
	return out
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
type ghStatusCheck struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
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
// entries: "failure" if any failed, "pending" if any pending, "success" if
// all passed, "" if no checks. Mirrors `gh pr checks` semantics.
func ghPRViewReal(dir, branch string) (*ghPRSnapshot, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, errGhUnavailable
	}
	cmd := exec.Command("gh", "pr", "view", branch,
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

func rollupCI(rolls []ghStatusCheck) string {
	if len(rolls) == 0 {
		return ""
	}
	hasPending, hasFailure := false, false
	for _, r := range rolls {
		switch {
		case r.Conclusion == "FAILURE" || r.Conclusion == "CANCELLED" || r.Conclusion == "TIMED_OUT":
			hasFailure = true
		case r.Status == "IN_PROGRESS" || r.Status == "QUEUED" || r.Conclusion == "":
			hasPending = true
		}
	}
	switch {
	case hasFailure:
		return "failure"
	case hasPending:
		return "pending"
	default:
		return "success"
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
	cmd := exec.Command("gh", "api",
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

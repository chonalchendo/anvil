package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// ghPRListFn looks up open PRs whose head branch matches `branch`. Returns the
// first matching PR url, or empty string if none. Package-level for tests to
// swap; default implementation shells out to `gh pr list`.
var ghPRListFn = ghPRListReal

type ghPR struct {
	URL         string `json:"url"`
	Number      int    `json:"number"`
	HeadRefName string `json:"headRefName"`
}

// ghPRListReal invokes `gh pr list --head <branch> --state open --json url,number,headRefName`.
// Returns (url, nil) when an open PR exists; ("", nil) when none; ("", err)
// when gh failed or is not installed.
func ghPRListReal(branch string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errGhMissing
	}
	out, err := exec.Command("gh", "pr", "list",
		"--head", branch,
		"--state", "open",
		"--json", "url,number,headRefName",
	).Output()
	if err != nil {
		return "", fmt.Errorf("gh pr list: %w", err)
	}
	var prs []ghPR
	if err := json.Unmarshal(out, &prs); err != nil {
		return "", fmt.Errorf("gh pr list: parsing json: %w", err)
	}
	for _, pr := range prs {
		if pr.HeadRefName == branch && pr.URL != "" {
			return pr.URL, nil
		}
	}
	return "", nil
}

// errGhMissing signals the gh binary is unavailable. Callers downgrade the
// refusal to a warning rather than fail-closed — agents working without gh
// (CI containers, fresh laptops) shouldn't be blocked, but they shouldn't be
// silently bypassed either.
var errGhMissing = errors.New("gh: command not found")

// openPRForIssueResolve enumerates candidate `anvil/<slug>` branches for an
// issue and returns (branch, prURL) of the first open PR found. Returns
// ("", "", nil) when no candidate branch has an open PR. Returns
// ("", "", errGhMissing) when gh is unavailable.
//
// Candidate branches:
//  1. anvil/<slug-from-issue-id> — covers the common single-issue workflow
//     where the agent cuts a worktree mirroring the issue slug.
//  2. anvil/<plan-slug> for any incoming plan link — covers the case where a
//     fleet-orchestrator or manual planner chose a divergent shorter slug.
func openPRForIssueResolve(v *core.Vault, id string) (branch, prURL string, err error) {
	for _, b := range candidateBranchesForIssue(v, id) {
		url, qerr := ghPRListFn(b)
		if qerr != nil {
			return "", "", qerr
		}
		if url != "" {
			return b, url, nil
		}
	}
	return "", "", nil
}

// candidateBranchesForIssue returns the `anvil/<slug>` branches to query for
// an issue. Order: id-derived first, then linked-plan slugs, then the
// current git branch when it bears the `anvil/` prefix. Duplicates removed.
//
// The current-branch fallback catches the fleet-orchestrator case: a
// dispatcher may choose a divergent short slug that is recorded in neither
// the issue id nor any plan frontmatter. The agent runs `anvil transition`
// from inside the worktree, so the branch name is right there.
func candidateBranchesForIssue(v *core.Vault, id string) []string {
	seen := map[string]bool{}
	var out []string
	addBranch := func(b string) {
		if b == "" || seen[b] {
			return
		}
		seen[b] = true
		out = append(out, b)
	}
	addSlug := func(slug string) {
		if slug == "" {
			return
		}
		addBranch("anvil/" + slug)
	}

	// Issue id is "<project>.<slug>"; pull the slug.
	if dot := strings.IndexByte(id, '.'); dot >= 0 && dot+1 < len(id) {
		addSlug(id[dot+1:])
	}

	// Any incoming plan link contributes its frontmatter slug.
	for _, slug := range linkedPlanSlugs(v, id) {
		addSlug(slug)
	}

	// Current branch, when it follows the `anvil/<slug>` convention.
	if b := currentAnvilBranch(); b != "" {
		addBranch(b)
	}
	return out
}

// currentAnvilBranch returns the current git branch only when it begins with
// `anvil/`. Empty string on any error (not in a git repo, detached HEAD,
// non-anvil branch). Best-effort: this is one of several signals into the
// candidate-branch set.
func currentAnvilBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if !strings.HasPrefix(branch, "anvil/") {
		return ""
	}
	return branch
}

// linkedPlanSlugs returns the `slug:` frontmatter field of every plan whose
// outgoing edges target `id`. Best-effort: any index/read error returns an
// empty list rather than failing the whole transition, on the same logic as
// loadIncomingEdges in show.go — incoming-link discovery is additive.
func linkedPlanSlugs(v *core.Vault, id string) []string {
	db, err := indexForRead(v)
	if err != nil {
		return nil
	}
	defer db.Close()

	rows, err := db.LinksTo(id)
	if err != nil {
		return nil
	}
	var slugs []string
	for _, r := range rows {
		row, err := db.GetArtifact(r.Source)
		if err != nil || row.Type != string(core.TypePlan) {
			continue
		}
		a, err := core.LoadArtifact(row.Path)
		if err != nil {
			continue
		}
		if s, _ := a.FrontMatter["slug"].(string); s != "" {
			slugs = append(slugs, s)
		}
	}
	return slugs
}

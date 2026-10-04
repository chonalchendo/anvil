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
// when gh is unusable in this environment (binary missing, unauthenticated,
// no network, no remote). Callers downgrade `errGhUnavailable` to a stderr
// warning so agents on fresh laptops or CI containers aren't blocked.
func ghPRListReal(branch string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errGhUnavailable
	}
	out, err := exec.Command("gh", "pr", "list", //nolint:gosec // binary path resolved from trusted sources; not user input
		"--head", branch,
		"--state", "open",
		"--json", "url,number,headRefName",
	).Output()
	if err != nil {
		// Any exec failure here is "we can't ask GitHub right now" — auth
		// missing, no network, no upstream remote, repo not on GitHub. All
		// of these warrant a warning, not a hard refusal: the user opted
		// into running anvil without a gh-ready environment.
		return "", errGhUnavailable
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

// errGhUnavailable signals gh is unusable here — binary missing, no auth,
// no network, no GitHub remote. Callers downgrade the refusal to a stderr
// warning. Fail-open is deliberate: a user whose environment lacks gh
// shouldn't be permanently blocked from resolving issues. (The methodology
// already requires a smoke gate before resolve, so the human still owns the
// post-merge call.)
var errGhUnavailable = errors.New("gh: unavailable (missing, unauthenticated, or no remote)")

// openPRForIssueResolve enumerates candidate `anvil/<slug>` branches for an
// issue and returns (branch, prURL, err). On any matching open PR: (branch,
// url, nil). On no match: ("", "", nil). err is non-nil only when gh itself
// is unavailable.
//
// Candidate branches:
//  1. anvil/<slug-from-issue-id> — covers the common single-issue workflow
//     where the agent cuts a worktree mirroring the issue slug.
//  2. The current git branch when it bears the `anvil/` prefix — covers a
//     dispatcher that chose a divergent slug.
func openPRForIssueResolve(id string) (branch, prURL string, err error) {
	for _, b := range candidateBranchesForIssue(id) {
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
// an issue: the id-derived slug first, then the current git branch when it
// bears the `anvil/` prefix. Duplicates removed.
//
// The agent runs `anvil transition` from inside the worktree, so the branch
// name is right there even when the dispatcher chose a slug the id does not
// record.
func candidateBranchesForIssue(id string) []string {
	seen := map[string]bool{}
	var out []string
	addBranch := func(b string) {
		if b == "" || seen[b] {
			return
		}
		seen[b] = true
		out = append(out, b)
	}

	bare := core.BareID(core.TypeIssue, core.CanonicalID(core.TypeIssue, id))
	if dot := strings.IndexByte(bare, '.'); dot >= 0 && dot+1 < len(bare) {
		addBranch("anvil/" + bare[dot+1:])
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

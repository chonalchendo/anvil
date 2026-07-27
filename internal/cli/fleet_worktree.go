package cli

import (
	"os/exec"
	"strings"

	"github.com/chonalchendo/anvil/internal/core"
)

// Worktree discovery + issue-to-branch matching for `anvil fleet status`,
// split out of fleet.go to keep it under the repo's 500-line file cap.

// fleetCandidateSlugs returns the branch slugs plausibly hosting an issue's
// worktree, most-specific first: the id-derived slug, then any linked plan's
// `slug:`. Slugs, not branches — the prefix is whatever project brands the
// worktree (`anvil/`, `burgh/`, `mentat/`), so matching happens on the
// segment after the last "/". Unlike candidateBranchesForIssue (used by
// `transition resolved`), we deliberately exclude the current-branch
// fallback — fleet enumerates many issues at once, so reusing the caller's
// branch as a wildcard would cross-pollute every row with the same match.
func fleetCandidateSlugs(v *core.Vault, id string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(slug string) {
		if slug == "" || seen[slug] {
			return
		}
		seen[slug] = true
		out = append(out, slug)
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

// genericSlugWorktree returns the lone worktree whose branch slug (the
// segment after its last "/") equals a candidate slug exactly, regardless of
// prefix. Candidates are tried in order, so the id-derived slug wins over a
// linked plan's slug. >1 worktree on the same slug = ambiguous, leave
// unmatched rather than guess which project owns it.
func genericSlugWorktree(worktrees map[string]worktreeInfo, slugs []string) (string, worktreeInfo, bool) {
	for _, slug := range slugs {
		var hitBranch string
		var hitInfo worktreeInfo
		count := 0
		for b, wt := range worktrees {
			i := strings.LastIndexByte(b, '/')
			if i < 0 {
				continue
			}
			if b[i+1:] == slug {
				hitBranch, hitInfo = b, wt
				count++
			}
		}
		if count == 1 {
			return hitBranch, hitInfo, true
		}
	}
	return "", worktreeInfo{}, false
}

// uniqueSubstringWorktree returns the lone worktree whose branch slug (the
// segment after its last "/", any prefix) is a substring of the issue id's
// slug, or ok=false if zero or two-plus match. The single-match rule keeps
// the fallback honest: ambiguity yields an unmatched row, not a wrong row.
func uniqueSubstringWorktree(worktrees map[string]worktreeInfo, id string) (string, worktreeInfo, bool) {
	dot := strings.IndexByte(id, '.')
	if dot < 0 || dot+1 >= len(id) {
		return "", worktreeInfo{}, false
	}
	issueSlug := id[dot+1:]
	var hitBranch string
	var hitInfo worktreeInfo
	count := 0
	for b, wt := range worktrees {
		i := strings.LastIndexByte(b, '/')
		if i < 0 {
			continue
		}
		branchSlug := b[i+1:]
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
// map keyed by local branch name (`<prefix>/<slug>`, e.g. `anvil/…`,
// `mentat/…`). Worktrees without a branch (e.g.
// detached HEAD) are skipped because the fleet workflow assumes each task
// runs on its own branch. Bare worktrees are similarly skipped.
// repoDir scopes the listing to a specific repo; "" keeps the caller's
// ambient cwd, which is what the fleet/doctor read-only views want.
func gitWorktreeListReal(repoDir string) (map[string]worktreeInfo, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "worktree", "list", "--porcelain") //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	out, err := cmd.Output()
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

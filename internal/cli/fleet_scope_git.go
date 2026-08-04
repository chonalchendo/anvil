package cli

// Self-derivation of the scope-audit's changed-file set, so the audit no
// longer trusts the caller's --changed computation blindly (issue.anvil.0236:
// a stale-base three-dot diff made a reversion of landed work invisible, so
// the caller's --changed under-reported and the audit blessed it).

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Indirection points for tests; real implementations shell out to git.
var (
	deriveChangedFn     = deriveChangedReal
	resolveBaseRefFn    = resolveBaseRefReal
	gitRevParseVerifyFn = gitRevParseVerifyReal
	gitMergeBaseFn      = gitMergeBaseReal
	gitDiffNamesFn      = gitDiffNamesReal
)

// deriveChangedReal computes the branch's true changed-file set: the diff
// between HEAD and the merge-base with the branch's actual upstream, derived
// from git itself rather than accepted from the caller. Best-effort — any
// step failing (no repo, no origin, no local main/master fallback) returns
// an error, and the caller degrades to the --changed list alone rather than
// aborting the audit.
func deriveChangedReal() ([]string, error) {
	repoDir, err := gitToplevelFn()
	if err != nil {
		return nil, err
	}
	base, err := resolveBaseRefFn(repoDir)
	if err != nil {
		return nil, err
	}
	mergeBase, err := gitMergeBaseFn(repoDir, base)
	if err != nil {
		return nil, err
	}
	return gitDiffNamesFn(repoDir, mergeBase)
}

// resolveBaseRefReal finds the ref the audit diffs HEAD against: a freshly
// fetched origin/HEAD when a remote exists (the normal case, and the one the
// stale-base incident needed to catch), else a local main/master fallback
// for a repo with no remote (the reproduction anchor's synthetic fixture).
func resolveBaseRefReal(repoDir string) (string, error) {
	if ferr := gitFetchOriginFn(repoDir); ferr == nil {
		if ref, rerr := gitResolveOriginHEADFn(repoDir); rerr == nil {
			return ref, nil
		}
	}
	for _, b := range []string{"main", "master"} {
		if gitRevParseVerifyFn(repoDir, b) == nil {
			return b, nil
		}
	}
	return "", errors.New("no origin/HEAD and no local main/master branch to diff against")
}

// gitRevParseVerifyReal reports whether ref resolves in repoDir.
func gitRevParseVerifyReal(repoDir, ref string) error {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref) //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	return cmd.Run()
}

// gitMergeBaseReal returns the merge-base commit of HEAD and ref, run from
// repoDir. This is what the stale caller-supplied diff skipped: recomputing
// the fork point against the ref's *current* tip, not whatever base commit
// the caller happened to diff against.
func gitMergeBaseReal(repoDir, ref string) (string, error) {
	cmd := exec.Command("git", "merge-base", "HEAD", ref) //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base HEAD %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitDiffNamesReal lists the files that differ between base and HEAD, run
// from repoDir.
func gitDiffNamesReal(repoDir, base string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", base, "HEAD") //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s HEAD: %w", base, err)
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

package cli

// Self-derivation of the scope-audit's changed-file set (issue.anvil.0236):
// a stale-base three-dot diff made a reversion of landed work invisible, so
// the caller's --changed under-reported and the audit blessed it. The audit
// recomputes the set itself — merge-base against a freshly fetched
// remote-tracking base. A local main/master is never a base: it can sit
// behind the ref the branch was reparented onto, hiding the same reversion.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Indirection point for tests; the real implementation shells out to git.
var deriveChangedFn = deriveChangedReal

// deriveChangedReal returns the branch's changed-file set — HEAD against its
// merge-base with a freshly fetched remote-tracking base — plus the base ref
// used. On error the caller degrades to the --changed list alone.
func deriveChangedReal() ([]string, string, error) {
	repoDir, err := gitToplevelReal()
	if err != nil {
		return nil, "", err
	}
	base, err := resolveRemoteBase(repoDir)
	if err != nil {
		return nil, "", err
	}
	mergeBase, err := gitMergeBase(repoDir, base)
	if err != nil {
		return nil, "", err
	}
	files, err := gitDiffNames(repoDir, mergeBase)
	return files, base, err
}

// resolveRemoteBase fetches origin and returns the remote-tracking ref to
// diff against: origin/HEAD, else origin/main or origin/master.
func resolveRemoteBase(repoDir string) (string, error) {
	if err := gitFetchOriginNoPrompt(repoDir); err != nil {
		return "", err
	}
	if ref, err := gitResolveOriginHEADReal(repoDir); err == nil {
		return ref, nil
	}
	for _, b := range []string{"origin/main", "origin/master"} {
		if gitRevParseVerify(repoDir, b) == nil {
			return b, nil
		}
	}
	return "", errors.New("no origin/HEAD, origin/main, or origin/master to diff against")
}

// gitFetchOriginNoPrompt fetches origin with a deadline and credential
// prompts disabled — this runs inside an unattended pre-PR gate, where a
// prompt would hang the worker. (gitFetchOriginReal's call sites are
// interactive and carry neither guard.)
func gitFetchOriginNoPrompt(repoDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin") //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch origin: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// gitRevParseVerify reports whether ref resolves in repoDir.
func gitRevParseVerify(repoDir, ref string) error {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", ref) //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	return cmd.Run()
}

// gitMergeBase returns the merge-base commit of HEAD and ref.
func gitMergeBase(repoDir, ref string) (string, error) {
	cmd := exec.Command("git", "merge-base", "HEAD", ref) //nolint:gosec // binary path resolved from trusted sources; not user input
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git merge-base HEAD %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitDiffNames lists the files that differ between base and HEAD.
func gitDiffNames(repoDir, base string) ([]string, error) {
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

package cli

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// unfinishedIssues lists the issues linked to milestone ms that are not yet
// terminal (open or in-progress). A scan or load failure is an error: a
// partial list could let a milestone close with work still open.
func unfinishedIssues(v *core.Vault, ms string) ([]string, error) {
	paths, err := collectArtifactPaths(v.Root, core.TypeIssue)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, p := range paths {
		other, err := core.LoadArtifact(p)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", p, err)
		}
		if milestoneSlug(other.FrontMatter["milestone"]) != ms {
			continue
		}
		if status, _ := other.FrontMatter["status"].(string); status == "open" || status == "in-progress" {
			ids = append(ids, listIDFor(core.TypeIssue, p))
		}
	}
	return ids, nil
}

// gitCurrentBranchFn is a seam so tests can stand in for a checkout.
var gitCurrentBranchFn = gitCurrentBranchReal

func gitCurrentBranchReal(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD") //nolint:gosec // fixed args
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// offBaseBranch returns the project repo's current branch and its default
// branch when they differ. Both are "" when they match or cannot be resolved:
// the check is best-effort, and a branch-only setup has no origin/HEAD.
func offBaseBranch(project string) (current, base string) {
	if project == "" {
		return "", ""
	}
	repoDir, err := resolveProjectRepoFn(project)
	if err != nil {
		return "", ""
	}
	ref, err := gitResolveOriginHEADFn(repoDir)
	if err != nil {
		return "", ""
	}
	cur, err := gitCurrentBranchFn(repoDir)
	if err != nil {
		return "", ""
	}
	base = strings.TrimPrefix(ref, "origin/")
	if cur == base {
		return "", ""
	}
	return cur, base
}

// gateMilestoneDone refuses `transition milestone done` while a linked issue is
// unfinished, the checkout is off the default branch, or an acceptance
// predicate is red. On success it rewrites the milestone body's `## Status`
// block with the measured ledger; the caller saves.
func gateMilestoneDone(cmd *cobra.Command, v *core.Vault, m *core.Artifact, id string) error {
	open, err := unfinishedIssues(v, strings.TrimPrefix(id, "milestone."))
	if err != nil {
		return errfmt.NewStructured("milestone_scan_failed").
			Set("milestone", id).
			Set("error", err.Error()).
			Set("fix_hint", "fix the unreadable issue artifact, then retry")
	}
	if len(open) > 0 {
		return errfmt.NewStructured("milestone_open_issues").
			Set("milestone", id).
			Set("issues", open).
			Set("fix_hint", "resolve or abandon each linked issue, then retry")
	}
	if cur, base := offBaseBranch(projectFromArtifact(m, id)); cur != "" {
		return errfmt.NewStructured("finish_line_not_on_base").
			Set("milestone", id).
			Set("branch", cur).
			Set("base", base).
			Set("fix_hint", "check out "+base+" with the merged work, then retry")
	}
	results := runAcceptance(cmd, m)
	if unmet := unmetCriteria(results); len(unmet) > 0 {
		return errfmt.NewStructured("acceptance_unmet").
			Set("milestone", id).
			Set("unmet", unmet).
			Set("fix_hint", "run: anvil milestone status "+id+"; fix the red criteria, then retry")
	}
	m.Body = replaceStatusBlock(m.Body, statusBlock(results, time.Now().UTC().Format("2006-01-02")))
	return nil
}

func statusBlock(results []acceptanceResult, date string) string {
	var b strings.Builder
	b.WriteString("## Status\n\nMeasured: " + date + ". Every acceptance predicate passes.\n\n| # | AC | Met | Measured |\n|---|---|---|---|\n")
	for i, r := range results {
		fmt.Fprintf(&b, "| %d | `%s` | met | exit %d |\n", i+1, tableCell(r.Criterion), r.Exit)
	}
	return b.String()
}

// tableCell flattens a predicate to a short single-line cell.
func tableCell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 60 {
		s = string(r[:57]) + "..."
	}
	return strings.NewReplacer("|", `\|`, "`", "'").Replace(s)
}

var (
	statusHeadingRe = regexp.MustCompile(`^##[ \t]+Status[ \t\r]*$`)
	h2Re            = regexp.MustCompile(`^##[ \t]`)
)

// replaceStatusBlock swaps the `## Status` section (to the next H2 heading or
// end of body) for block, appending it when the body has none. The scan is
// line-anchored and skips fenced code, as core.Section does, so a `### Status`
// or a fenced `## Status` is never mistaken for the section. Prose after the
// old ledger is not preserved: the block is the whole section.
func replaceStatusBlock(body, block string) string {
	lines := strings.Split(body, "\n")
	start, end := -1, len(lines)
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if start < 0 {
			if statusHeadingRe.MatchString(line) {
				start = i
			}
			continue
		}
		if h2Re.MatchString(line) {
			end = i
			break
		}
	}
	if start < 0 {
		return strings.TrimRight(body, "\n") + "\n\n" + block
	}
	pre := ""
	if start > 0 {
		pre = strings.Join(lines[:start], "\n") + "\n"
	}
	if end == len(lines) {
		return pre + block
	}
	return pre + block + "\n" + strings.Join(lines[end:], "\n")
}

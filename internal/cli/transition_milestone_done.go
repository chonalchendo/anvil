package cli

import (
	"errors"
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
// terminal: any status other than resolved or abandoned (open, in-progress,
// escalated). A scan or load failure is an error: a
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
		if status, _ := other.FrontMatter["status"].(string); status != "resolved" && status != "abandoned" {
			ids = append(ids, listIDFor(core.TypeIssue, p))
		}
	}
	return ids, nil
}

// gitRevParseFn is a seam so tests can stand in for a checkout.
var gitRevParseFn = gitRevParseReal

func gitRevParseReal(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"rev-parse"}, args...)...) //nolint:gosec // fixed verb; refs come from git itself
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// finishLine is where a milestone's acceptance predicates are measured: the
// project repo, and whether its HEAD is the default branch's tip.
type finishLine struct {
	Dir  string // project repo; "" when unresolved
	Head string
	Base string
	Err  error // set when the base check could not run
}

func (f finishLine) off() bool { return f.Err == nil && f.Head != f.Base }

// checkFinishLine compares the project repo's HEAD with origin/HEAD by sha, so
// a branch name that merely matches cannot pass. The check is best-effort: a
// branch-only setup has no origin/HEAD, and callers warn on Err.
func checkFinishLine(project string) finishLine {
	if project == "" {
		return finishLine{Err: errors.New("milestone has no project")}
	}
	dir, err := resolveProjectRepoFn(project)
	if err != nil {
		return finishLine{Err: err}
	}
	fl := finishLine{Dir: dir}
	ref, err := gitResolveOriginHEADFn(dir)
	if err != nil {
		fl.Err = err
		return fl
	}
	if fl.Base, fl.Err = gitRevParseFn(dir, ref); fl.Err != nil {
		return fl
	}
	fl.Head, fl.Err = gitRevParseFn(dir, "HEAD")
	return fl
}

// warnBaseUnchecked reports a finish line whose base could not be checked.
func warnBaseUnchecked(cmd *cobra.Command, fl finishLine) {
	if fl.Err != nil {
		cmd.PrintErrln("warning: base branch not checked (" + fl.Err.Error() + "); predicates measure the current checkout")
	}
}

// gateMilestoneDone refuses `transition milestone done` while a linked issue is
// unfinished, the checkout is off the default branch, or an acceptance
// predicate is red. On success it stamps the `done` date and rewrites the
// milestone body's `## Status` block with the measured ledger; the caller saves.
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
	fl := checkFinishLine(projectFromArtifact(m, id))
	warnBaseUnchecked(cmd, fl)
	if fl.off() {
		return errfmt.NewStructured("finish_line_not_on_base").
			Set("milestone", id).
			Set("head", fl.Head).
			Set("base", fl.Base).
			Set("fix_hint", "check out the default branch with the merged work, then retry")
	}
	results := runAcceptance(cmd, m, fl.Dir)
	if unmet := unmetCriteria(results); len(unmet) > 0 {
		return errfmt.NewStructured("acceptance_unmet").
			Set("milestone", id).
			Set("unmet", unmet).
			Set("fix_hint", "run: anvil milestone status "+id+"; fix the red criteria, then retry")
	}
	commit, _ := gitRevParseFn(fl.Dir, "--short", "HEAD")
	date := time.Now().UTC().Format("2006-01-02")
	m.FrontMatter["done"] = date
	m.Body = replaceStatusBlock(m.Body, statusBlock(results, date, commit))
	return nil
}

func statusBlock(results []acceptanceResult, date, commit string) string {
	var b strings.Builder
	b.WriteString("## Status\n\nMeasured: " + date)
	if commit != "" {
		b.WriteString(", at `" + commit + "`")
	}
	b.WriteString(". Every acceptance predicate passes.\n\n| # | AC | Met | Measured |\n|---|---|---|---|\n")
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

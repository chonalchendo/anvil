package cli

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// acceptanceResult is one milestone `acceptance:` predicate run on the current
// checkout. A timeout or a run failure counts as not met.
type acceptanceResult struct {
	Criterion string `json:"criterion"`
	Met       bool   `json:"met"`
	Exit      int    `json:"exit"`
	TimedOut  bool   `json:"timed_out,omitempty"`
}

// runAcceptance runs each acceptance predicate through the issue create gate's
// block runner (create_feasibility.go), so both gates share one execution shape.
func runAcceptance(m *core.Artifact) []acceptanceResult {
	preds, _ := m.FrontMatter["acceptance"].([]any)
	results := make([]acceptanceResult, 0, len(preds))
	for _, p := range preds {
		s, _ := p.(string)
		r := runFeasibilityBlock(s)
		results = append(results, acceptanceResult{
			Criterion: s,
			Met:       r.runErr == nil && !r.timedOut && r.exit == 0,
			Exit:      r.exit,
			TimedOut:  r.timedOut,
		})
	}
	return results
}

func unmetCriteria(rs []acceptanceResult) []string {
	var unmet []string
	for _, r := range rs {
		if !r.Met {
			unmet = append(unmet, r.Criterion)
		}
	}
	return unmet
}

// unfinishedIssues lists the issues linked to milestone ms that are not yet
// terminal (open or in-progress). Scan errors read as none: callers are
// advisory or run the acceptance gate next.
func unfinishedIssues(v *core.Vault, ms string) []string {
	paths, err := collectArtifactPaths(v.Root, core.TypeIssue)
	if err != nil {
		return nil
	}
	var ids []string
	for _, p := range paths {
		other, err := core.LoadArtifact(p)
		if err != nil || milestoneSlug(other.FrontMatter["milestone"]) != ms {
			continue
		}
		if status, _ := other.FrontMatter["status"].(string); status == "open" || status == "in-progress" {
			ids = append(ids, listIDFor(core.TypeIssue, p))
		}
	}
	return ids
}

// gateMilestoneDone refuses `transition milestone done` while a linked issue is
// unfinished or an acceptance predicate is red. On success it rewrites the
// milestone body's `## Status` block with the measured ledger; the caller saves.
func gateMilestoneDone(v *core.Vault, m *core.Artifact, id string) error {
	if open := unfinishedIssues(v, strings.TrimPrefix(id, "milestone.")); len(open) > 0 {
		return errfmt.NewStructured("milestone_open_issues").
			Set("milestone", id).
			Set("issues", open).
			Set("fix_hint", "resolve or abandon each linked issue, then retry")
	}
	results := runAcceptance(m)
	if unmet := unmetCriteria(results); len(unmet) > 0 {
		return errfmt.NewStructured("acceptance_unmet").
			Set("milestone", id).
			Set("unmet", unmet).
			Set("fix_hint", "run: anvil milestone status "+id+"; fix the red criteria, then retry")
	}
	m.Body = replaceStatusBlock(m.Body, statusBlock(results, time.Now().UTC().Format("2006-01-02"), headCommit()))
	return nil
}

func headCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func statusBlock(results []acceptanceResult, date, commit string) string {
	var b strings.Builder
	b.WriteString("## Status\n\nMeasured: " + date)
	if commit != "" {
		b.WriteString(", at `" + commit + "`")
	}
	b.WriteString(". Every acceptance predicate passes.\n\n| AC | Met | Measured |\n|---|---|---|\n")
	for _, r := range results {
		fmt.Fprintf(&b, "| `%s` | met | exit %d |\n", tableCell(r.Criterion), r.Exit)
	}
	return b.String()
}

// tableCell flattens a predicate to a short single-line cell.
func tableCell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return strings.NewReplacer("|", `\|`, "`", "'").Replace(s)
}

// replaceStatusBlock swaps the `## Status` section (to the next `## ` heading
// or end of body) for block, appending it when the body has none. Prose after
// the old ledger table is not preserved: the block is the whole section.
func replaceStatusBlock(body, block string) string {
	const heading = "## Status\n"
	start := strings.Index(body, heading)
	if start < 0 {
		return strings.TrimRight(body, "\n") + "\n\n" + block
	}
	rest := body[start+len(heading):]
	end := len(body)
	if i := strings.Index(rest, "\n## "); i >= 0 {
		end = start + len(heading) + i + 1
		block += "\n"
	}
	return body[:start] + block + body[end:]
}

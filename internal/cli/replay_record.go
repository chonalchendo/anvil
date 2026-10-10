package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

var hexSha7 = regexp.MustCompile(`[0-9a-f]{7}`)

// replayDiff counts the changed lines and files in the current directory
// against base. Binary files count as a file and no lines.
func replayDiff() (lines, files int, err error) {
	base, err := exec.Command("git", "merge-base", "HEAD", "origin/HEAD").Output()
	if err != nil {
		return 0, 0, fmt.Errorf("finding the replay base: %w", err)
	}
	out, err := exec.Command("git", "diff", "--numstat", strings.TrimSpace(string(base))).Output() //nolint:gosec // base is a sha from git
	if err != nil {
		return 0, 0, fmt.Errorf("git diff: %w", err)
	}
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(l)
		if len(f) < 3 {
			continue
		}
		files++
		add, _ := strconv.Atoi(f[0])
		del, _ := strconv.Atoi(f[1])
		lines += add + del
	}
	return lines, files, nil
}

// replaySection renders the Replay section. The landed cost sits beside the replay's.
func replaySection(rec verifyRecord, lines, files, tokens int, landed *costFields) string {
	sha := hexSha7.FindString(resolveVersion())
	if sha == "" {
		sha = "unknown"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Replay — %s @ %s\n\n", time.Now().UTC().Format("2006-01-02"), sha)
	fmt.Fprintf(&b, "- verdict: %s (%d checks)\n", rec.Verdict, rec.Checks)
	for _, f := range rec.Failed {
		fmt.Fprintf(&b, "- failed: %s — %s\n", f.Check, f.Preview)
	}
	fmt.Fprintf(&b, "- replay: diff %d, files %d, tokens %d\n", lines, files, tokens)
	if landed != nil {
		fmt.Fprintf(&b, "- landed: diff %d, files %d, tokens %d\n", landed.Diff, landed.Files, landed.Tokens)
	} else {
		b.WriteString("- landed: no cost record\n")
	}
	return b.String()
}

// appendReplay reloads the issue and appends the section; no frontmatter changes.
func appendReplay(v *core.Vault, path, id, arg string, rec verifyRecord, tokens int) error {
	lines, files, err := replayDiff()
	if err != nil {
		return errfmt.NewStructured("replay_diff_failed").Set("message", err.Error()).
			Set("fix_hint", "run anvil verify "+id+" --replay from inside the worktree anvil replay cut")
	}
	a, err := loadIssueForVerify(path, id, arg)
	if err != nil {
		return err
	}
	a.Body = strings.TrimRight(a.Body, "\n") + "\n" + replaySection(rec, lines, files, tokens, costFromFrontMatter(a.FrontMatter))
	if err := a.Save(); err != nil {
		return fmt.Errorf("saving artifact: %w", err)
	}
	if err := indexAfterSave(v, a); err != nil {
		return fmt.Errorf("indexing %s: %w", id, err)
	}
	return nil
}

// verifyReplay runs the blocks here and records the result as a Replay section
// only: the landed verified_* and outcome_* fields stay as they were.
func verifyReplay(cmd *cobra.Command, v *core.Vault, path, id, arg, body string, tokens int, asJSON bool) error {
	rec, err := runVerification(cmd.ErrOrStderr(), body, "")
	if err != nil {
		return err
	}
	if err := appendReplay(v, path, id, arg, rec, tokens); err != nil {
		return err
	}
	if asJSON {
		b, _ := json.Marshal(rec)
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
	}
	if rec.Verdict != "pass" {
		return fmt.Errorf("replay verification %s: %d of %d check(s) failed", rec.Verdict, len(rec.Failed), rec.Checks)
	}
	return nil
}

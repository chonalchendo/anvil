package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// verifyFailure is one red block. Line is the block line a set -e abort
// stopped on, empty when the block ended another way. Exit is nil (JSON null)
// when the block never produced an exit status.
type verifyFailure struct {
	Check   string `json:"check"`
	Exit    *int   `json:"exit"`
	Line    string `json:"line"`
	Preview string `json:"preview"`
}

// verifyRecord mirrors run-verification.sh's verdict line so the skill bundle
// can swap the script for this verb. One known difference for wave 5: a block
// that leaves a background process holding stdout (`sleep 8 &`) fails here once
// the 2 s WaitDelay passes (exit null), where the script waits and passes it.
type verifyRecord struct {
	Verdict  string          `json:"verdict"`
	Checks   int             `json:"checks"`
	Failed   []verifyFailure `json:"failed"`
	Deferred []verifyFailure `json:"deferred"`
	Commit   string          `json:"commit"`
	RanAt    string          `json:"ran_at"`
}

func newVerifyCmd() *cobra.Command {
	var flagJSON, flagAccept bool
	cmd := &cobra.Command{
		Use:   "verify <issue-id>",
		Short: "Run an issue's Verification blocks here and record the verdict on the issue",
		Long: "Run every Direct and Indirect block of the issue's `## Verification` in the current directory " +
			"and stamp verified_verdict, verified_commit and verified_at on the issue, pass or fail. " +
			"Refuses with verification_changed when the section differs from the claim's verification_lock, unless --accept-change. A red Indirect block marked `# anvil:post-land` is deferred, not failed. Exits non-zero unless the verdict is pass.",
		Example: "  anvil verify issue.anvil.0314.anvil-verify-records-the-verdict --json | jq -r .verdict",
		Args:    namedArgs("anvil verify <issue-id>", []string{"<issue-id>"}, 1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			id, path, err := core.ResolveArtifact(v, core.TypeIssue, args[0])
			if err != nil {
				return err
			}
			a, err := loadIssueForVerify(path, id, args[0])
			if err != nil {
				return err
			}
			if err := checkVerificationLock(a, id, flagAccept); err != nil {
				return printAndReturn(cmd, err)
			}
			ranBody := a.Body
			rec, err := runVerification(cmd, ranBody)
			if err != nil {
				return err
			}
			// Reload: the run can take minutes, and a write to the issue in that
			// window must not be overwritten by the pre-run copy.
			if a, err = loadIssueForVerify(path, id, args[0]); err != nil {
				return err
			}
			if flagAccept {
				a.FrontMatter["verification_lock"] = core.VerificationLock(ranBody)
			}
			a.FrontMatter["verified_verdict"] = rec.Verdict
			a.FrontMatter["verified_commit"] = rec.Commit
			a.FrontMatter["verified_at"] = rec.RanAt
			if err := a.Save(); err != nil {
				return fmt.Errorf("saving artifact: %w", err)
			}
			if err := indexAfterSave(v, a); err != nil {
				return fmt.Errorf("indexing %s: %w", id, err)
			}
			if flagJSON {
				b, _ := json.Marshal(rec)
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
			}
			if rec.Verdict != "pass" {
				return fmt.Errorf("verification %s: %d of %d check(s) failed", rec.Verdict, len(rec.Failed), rec.Checks)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagAccept, "accept-change", false, "re-lock a Verification section edited after the claim, then run (the human's flag)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "print the verdict record as one JSON line on stdout")
	return cmd
}

// checkVerificationLock refuses before any block runs when the section differs
// from the claim's lock. No lock means the issue predates the rule.
func checkVerificationLock(a *core.Artifact, id string, accept bool) error {
	lock, _ := a.FrontMatter["verification_lock"].(string)
	if accept || lock == "" || lock == core.VerificationLock(a.Body) {
		return nil
	}
	return errfmt.NewStructured("verification_changed").
		Set("issue", id).
		Set("message", id+": the ## Verification section changed after the claim").
		Set("fix_hint", "review the change, then run anvil verify "+id+" --accept-change")
}

func loadIssueForVerify(path, id, arg string) (*core.Artifact, error) {
	a, err := core.LoadArtifact(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, notFoundErr(id, arg)
		}
		return nil, fmt.Errorf("loading artifact: %w", err)
	}
	return a, nil
}

// runVerification runs every Direct then Indirect block in the cwd. Unlike the
// create gate, green and red mean pass and fail for both subsections.
func runVerification(cmd *cobra.Command, body string) (verifyRecord, error) {
	rec := verifyRecord{Failed: []verifyFailure{}, Deferred: []verifyFailure{}, RanAt: time.Now().UTC().Format(time.RFC3339)}
	rec.Commit = cwdCommit()
	for _, label := range []string{"Direct", "Indirect"} {
		blocks, err := core.VerificationBlocks(body, label)
		if err != nil {
			return rec, fmt.Errorf("verification %s: %w", label, err)
		}
		if len(blocks) == 0 {
			// Invariant 6: a section with nothing to run is a failed check, not a pass.
			rec.Checks++
			cmd.PrintErrln(fmt.Sprintf("FAIL ### %s has no executable ```bash block", label))
			rec.Failed = append(rec.Failed, verifyFailure{Check: label, Preview: "no executable bash block"})
			continue
		}
		for i, block := range blocks {
			rec.Checks++
			check := fmt.Sprintf("%s#%d", label, i+1)
			f := verifyFailure{Check: check, Preview: blockPreview(block)}
			if f.Preview == "" {
				f.Preview = "block has no executable command"
				cmd.PrintErrln(fmt.Sprintf("FAIL [%s] %s (empty or all comments)", check, f.Preview))
				rec.Failed = append(rec.Failed, f)
				continue
			}
			if vacuous := core.NonGatingNegation(block); vacuous != "" {
				cmd.PrintErrln(fmt.Sprintf("FAIL [%s] %s: carries `%s`; %s", check, f.Preview, vacuous, nonGatingNegationWhy))
				f.Preview = "non-gating negation: " + vacuous
				rec.Failed = append(rec.Failed, f)
				continue
			}
			cmd.PrintErrln("anvil: running verification " + check + " in this environment (your privileges, cwd and environment; not sandboxed)")
			// No timeout: Direct is typically the repo's whole suite, which the
			// create gate's cap would fail as red.
			r := runFeasibilityBlock(block, "", 0)
			why, failed := r.failure()
			if !failed {
				cmd.PrintErrln(fmt.Sprintf("PASS [%s] %s", check, f.Preview))
				continue
			}
			if r.runErr == nil && !r.timedOut {
				exit := r.exit
				f.Exit = &exit
			}
			_, f.Line, _ = blockLines(block, r.redLine)
			if label == "Indirect" && core.IsPostLand(block) {
				cmd.PrintErrln(fmt.Sprintf("DEFERRED [%s] %s (%s; post-land)\n%s", check, f.Preview, why, firstLines(r.output, 10)))
				rec.Deferred = append(rec.Deferred, f)
				continue
			}
			cmd.PrintErrln(fmt.Sprintf("FAIL [%s] %s (%s)\n%s", check, f.Preview, why, firstLines(r.output, 10)))
			rec.Failed = append(rec.Failed, f)
		}
	}
	rec.Verdict = "pass"
	if len(rec.Failed) > 0 {
		rec.Verdict = "fail"
	}
	return rec, nil
}

// blockPreview is the block's first command line.
func blockPreview(block string) string {
	for _, l := range strings.Split(block, "\n") {
		if !isBlankOrComment(l) {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// cwdCommit is the cwd's HEAD, suffixed -dirty when the tree has uncommitted
// changes, and empty outside a git repo. A verdict must name the tree it ran on.
func cwdCommit() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output() //nolint:gosec // fixed argv
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))
	if st, err := exec.Command("git", "status", "--porcelain").Output(); err == nil && len(strings.TrimSpace(string(st))) > 0 { //nolint:gosec // fixed argv
		sha += "-dirty"
	}
	return sha
}

// firstLines is the first n lines of out, indented, as run-verification.sh prints them.
func firstLines(out string, n int) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return "    " + strings.Join(lines, "\n    ")
}

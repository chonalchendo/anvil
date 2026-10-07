package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// verifyFailure is one red block. Line is the block line a set -e abort
// stopped on, empty when the block ended another way.
type verifyFailure struct {
	Check   string `json:"check"`
	Exit    int    `json:"exit"`
	Line    string `json:"line"`
	Preview string `json:"preview"`
}

// verifyRecord mirrors run-verification.sh's verdict line so the skill bundle
// can swap the script for this verb.
type verifyRecord struct {
	Verdict  string          `json:"verdict"`
	Checks   int             `json:"checks"`
	Failed   []verifyFailure `json:"failed"`
	Deferred []verifyFailure `json:"deferred"`
	Commit   string          `json:"commit"`
	RanAt    string          `json:"ran_at"`
}

func newVerifyCmd() *cobra.Command {
	var flagJSON bool
	cmd := &cobra.Command{
		Use:   "verify <issue-id>",
		Short: "Run an issue's Verification blocks here and record the verdict on the issue",
		Long: "Run every Direct and Indirect block of the issue's `## Verification` in the current directory " +
			"and stamp verified_verdict, verified_commit and verified_at on the issue, pass or fail. " +
			"A red Indirect block marked `# anvil:post-land` is deferred, not failed. Exits non-zero unless the verdict is pass.",
		Args: namedArgs("anvil verify <issue-id>", []string{"<issue-id>"}, 1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := core.ResolveVault()
			if err != nil {
				return fmt.Errorf("resolving vault: %w", err)
			}
			id, path, err := core.ResolveArtifact(v, core.TypeIssue, args[0])
			if err != nil {
				return err
			}
			a, err := core.LoadArtifact(path)
			if err != nil {
				if os.IsNotExist(err) {
					return notFoundErr(id, args[0])
				}
				return fmt.Errorf("loading artifact: %w", err)
			}
			rec, err := runVerification(cmd, a.Body)
			if err != nil {
				return err
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
	cmd.Flags().BoolVar(&flagJSON, "json", false, "print the verdict record as one JSON line on stdout")
	return cmd
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
		for i, block := range blocks {
			rec.Checks++
			check := fmt.Sprintf("%s#%d", label, i+1)
			f := verifyFailure{Check: check, Preview: blockPreview(block)}
			if vacuous := core.NonGatingNegation(block); vacuous != "" {
				f.Exit = 1
				f.Line = vacuous
				cmd.PrintErrln(fmt.Sprintf("FAIL [%s] %s: carries `%s`; %s", check, f.Preview, vacuous, nonGatingNegationWhy))
				rec.Failed = append(rec.Failed, f)
				continue
			}
			cmd.PrintErrln("anvil: running verification " + check + " in this environment (your privileges, cwd and environment; not sandboxed)")
			r := runFeasibilityBlock(block, "")
			var why string
			switch {
			case r.runErr != nil:
				f.Exit, why = -1, r.runErr.Error()
			case r.timedOut:
				f.Exit, why = -1, "timed out after "+feasibilityTimeout.String()
			case r.exit != 0:
				f.Exit, why = r.exit, fmt.Sprintf("exit %d", r.exit)
			default:
				cmd.PrintErrln(fmt.Sprintf("PASS [%s] %s", check, f.Preview))
				continue
			}
			_, f.Line, _ = blockLines(block, r.redLine)
			if label == "Indirect" && core.IsPostLand(block) {
				cmd.PrintErrln(fmt.Sprintf("DEFERRED [%s] %s (%s; post-land)", check, f.Preview, why))
				rec.Deferred = append(rec.Deferred, f)
				continue
			}
			cmd.PrintErrln(fmt.Sprintf("FAIL [%s] %s (%s)\n%s", check, f.Preview, why, r.output))
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

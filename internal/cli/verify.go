package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

// verifyRecord is the verdict line the skill bundle gates on. A block that
// leaves a background process holding stdout (`sleep 8 &`) fails once the 2 s
// WaitDelay passes (exit null).
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
	var flagAt string
	cmd := &cobra.Command{
		Use:   "verify <issue-id>",
		Short: "Run an issue's Verification blocks (here, or at a commit with --at) and record the verdict",
		Long: "Run every Direct and Indirect block of the issue's `## Verification` in the current directory " +
			"and stamp verified_verdict, verified_commit and verified_at on the issue, pass or fail. " +
			"--at <sha> runs the blocks on a fresh detached checkout of that commit instead and stamps the record at it, so untracked files and local builds cannot turn a red block green. " +
			"Refuses with verification_changed when the section differs from the claim's verification_lock, unless --accept-change. A red Indirect block marked `# anvil:post-land` is deferred, not failed. Exits non-zero unless the verdict is pass.",
		Example: "  anvil verify issue.anvil.0314.anvil-verify-records-the-verdict --json | jq -r .verdict\n" +
			"  anvil verify <issue> --at $(gh pr view <n> --json headRefOid -q .headRefOid) --json",
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
			a, err := loadIssueForVerify(path, id, args[0])
			if err != nil {
				return err
			}
			if err := checkVerificationLock(a, id, flagAccept); err != nil {
				return printAndReturn(cmd, err)
			}
			var rec verifyRecord
			if flagAt != "" {
				root, terr := gitToplevelFn()
				if terr != nil {
					return printAndReturn(cmd, errfmt.NewStructured("verify_at_no_repo").
						Set("message", "--at needs a git repo; the current directory is not in one").
						Set("fix_hint", "cd into the repo that holds "+flagAt+", then run anvil verify "+id+" --at "+flagAt))
				}
				rec, err = verifyAt(cmd.ErrOrStderr(), id, root, flagAt, a.Body)
			} else {
				rec, err = runVerification(cmd.ErrOrStderr(), a.Body, "")
			}
			if err != nil {
				var se *errfmt.Structured
				if errors.As(err, &se) {
					return printAndReturn(cmd, err)
				}
				return err
			}
			lock := ""
			if flagAccept {
				lock = core.VerificationLock(a.Body)
			}
			if err := stampVerification(v, path, id, args[0], rec, lock); err != nil {
				return err
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
	cmd.Flags().StringVar(&flagAt, "at", "", "run the blocks on a fresh detached checkout of this commit and stamp it")
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

// stampVerification reloads the issue, stamps the record on it and saves. The
// run can take minutes, and a write to the issue in that window must not be
// overwritten by a pre-run copy. A non-empty lock also re-locks the section.
func stampVerification(v *core.Vault, path, id, arg string, rec verifyRecord, lock string) error {
	a, err := loadIssueForVerify(path, id, arg)
	if err != nil {
		return err
	}
	if lock != "" {
		if old, _ := a.FrontMatter["verification_lock"].(string); old != lock {
			bumpOutcome(a, "outcome_rescopes")
		}
		a.FrontMatter["verification_lock"] = lock
	}
	// The first verdict after the first claim is the outcome record; later runs
	// overwrite verified_verdict but not this.
	if _, claimed := a.FrontMatter["claimed_at"]; claimed && a.FrontMatter["outcome_first_verdict"] == nil {
		a.FrontMatter["outcome_first_verdict"] = rec.Verdict
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
	return nil
}

// verifyAt runs body on a fresh detached checkout of sha in root's repo and
// returns the record stamped at the resolved commit.
func verifyAt(errW io.Writer, id, root, sha, body string) (verifyRecord, error) {
	dir, commit, cleanup, err := checkoutAt(errW, id, root, sha)
	if err != nil {
		return verifyRecord{}, err
	}
	defer cleanup()
	rec, err := runVerification(errW, body, dir)
	if err != nil {
		return rec, err
	}
	// The checkout is the resolved commit by construction; git status can still
	// report changes on it (case-folding filesystems).
	rec.Commit = commit
	return rec, nil
}

// checkoutAt adds a detached worktree of sha under a temp dir, provisions it
// like a cut worktree (carry files, then the worktree hook), and returns it
// with the resolved commit and its cleanup. A removal failure is a notice: it
// must not change the verdict.
func checkoutAt(errW io.Writer, id, root, sha string) (string, string, func(), error) {
	full, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "--end-of-options", sha+"^{commit}").Output() //nolint:gosec // sha is a single argv element after --end-of-options
	if err != nil {
		return "", "", nil, errfmt.NewStructured("verify_at_unresolved").
			Set("message", sha+" does not resolve to a commit in "+root).
			Set("fix_hint", "fetch it with git fetch, or check it with git rev-parse "+sha+"^{commit}")
	}
	commit := strings.TrimSpace(string(full))
	failed := func(msg, hint string) error {
		return errfmt.NewStructured("verify_at_checkout_failed").
			Set("message", msg).
			Set("fix_hint", hint+", then re-run anvil verify "+id+" --at "+sha)
	}
	tmp, err := os.MkdirTemp("", "anvil-verify-at-*")
	if err != nil {
		return "", "", nil, failed("creating checkout dir: "+err.Error(), "check free disk space and that TMPDIR is writable")
	}
	dir := filepath.Join(tmp, "wt")
	if out, err := exec.Command("git", "-C", root, "worktree", "add", "--detach", dir, commit).CombinedOutput(); err != nil { //nolint:gosec // commit is a resolved sha
		_ = os.RemoveAll(tmp)
		return "", "", nil, failed("git worktree add: "+err.Error()+": "+strings.TrimSpace(string(out)), "fix the git error in message")
	}
	cleanup := func() {
		if err := gitWorktreeRemoveForceFn(root, dir); err != nil {
			fmt.Fprintln(errW, "anvil: could not remove verify checkout "+dir+": "+err.Error())
		}
		_ = os.RemoveAll(tmp)
	}
	if err := provisionCheckout(root, dir); err != nil {
		cleanup()
		return "", "", nil, failed("provisioning the checkout: "+err.Error(), "fix the carry list or worktree hook named in message")
	}
	return dir, commit, cleanup, nil
}

// provisionCheckout gives a clean checkout what a cut worktree gets: the
// declared carry files, then the repo's worktree hook.
func provisionCheckout(root, dir string) error {
	paths, err := checkCarryDeclarations(root)
	if err != nil {
		return err
	}
	if err := copyCarryFiles(root, dir, paths); err != nil {
		return err
	}
	return runWorktreeHookFn(root, dir)
}

// runVerification runs every Direct then Indirect block in dir ("" is the cwd).
// Unlike the create gate, green and red mean pass and fail for both subsections.
func runVerification(errW io.Writer, body, dir string) (verifyRecord, error) {
	rec := verifyRecord{Failed: []verifyFailure{}, Deferred: []verifyFailure{}, RanAt: time.Now().UTC().Format(time.RFC3339)}
	rec.Commit = commitOf(dir)
	for _, label := range []string{"Direct", "Indirect"} {
		blocks, err := core.VerificationBlocks(body, label)
		if err != nil {
			return rec, fmt.Errorf("verification %s: %w", label, err)
		}
		if len(blocks) == 0 {
			// Invariant 6: a section with nothing to run is a failed check, not a pass.
			rec.Checks++
			fmt.Fprintf(errW, "FAIL ### %s has no executable ```bash block\n", label)
			rec.Failed = append(rec.Failed, verifyFailure{Check: label, Preview: "no executable bash block"})
			continue
		}
		for i, block := range blocks {
			rec.Checks++
			check := fmt.Sprintf("%s#%d", label, i+1)
			f := verifyFailure{Check: check, Preview: blockPreview(block)}
			if f.Preview == "" {
				f.Preview = "block has no executable command"
				fmt.Fprintf(errW, "FAIL [%s] %s (empty or all comments)\n", check, f.Preview)
				rec.Failed = append(rec.Failed, f)
				continue
			}
			if vacuous := core.NonGatingNegation(block); vacuous != "" {
				fmt.Fprintf(errW, "FAIL [%s] %s: carries `%s`; %s\n", check, f.Preview, vacuous, nonGatingNegationWhy)
				f.Preview = "non-gating negation: " + vacuous
				rec.Failed = append(rec.Failed, f)
				continue
			}
			fmt.Fprintln(errW, "anvil: running verification "+check+" in this environment (your privileges, cwd and environment; not sandboxed)")
			// No timeout: Direct is typically the repo's whole suite, which the
			// create gate's cap would fail as red.
			r := runFeasibilityBlock(block, dir, 0)
			why, failed := r.failure()
			if !failed {
				fmt.Fprintf(errW, "PASS [%s] %s\n", check, f.Preview)
				continue
			}
			if r.runErr == nil && !r.timedOut {
				exit := r.exit
				f.Exit = &exit
			}
			_, f.Line, _ = blockLines(block, r.redLine)
			if label == "Indirect" && core.IsPostLand(block) {
				fmt.Fprintf(errW, "DEFERRED [%s] %s (%s; post-land)\n%s\n", check, f.Preview, why, firstLines(r.output, 10))
				rec.Deferred = append(rec.Deferred, f)
				continue
			}
			fmt.Fprintf(errW, "FAIL [%s] %s (%s)\n%s\n", check, f.Preview, why, firstLines(r.output, 10))
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

// commitOf is dir's HEAD ("" is the cwd), suffixed -dirty when the tree has
// uncommitted changes, and empty outside a git repo. A verdict must name the tree it ran on.
func commitOf(dir string) string {
	rp := exec.Command("git", "rev-parse", "HEAD") //nolint:gosec // fixed argv
	rp.Dir = dir
	out, err := rp.Output()
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))
	stc := exec.Command("git", "status", "--porcelain") //nolint:gosec // fixed argv
	stc.Dir = dir
	if st, err := stc.Output(); err == nil && len(strings.TrimSpace(string(st))) > 0 {
		sha += "-dirty"
	}
	return sha
}

// firstLines is the first n lines of out, indented, as the summary prints them.
func firstLines(out string, n int) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return "    " + strings.Join(lines, "\n    ")
}

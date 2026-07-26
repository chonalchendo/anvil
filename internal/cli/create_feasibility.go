package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// feasibilityTimeout caps a single Verification block. The gate fires
// synchronously at create time, so a block that legitimately needs longer
// should be slimmed to the one command that proves the predicate — the same
// guidance writing-issue already gives authors.
const feasibilityTimeout = 60 * time.Second

// feasibilityMaxOutputBytes caps captured combined stdout+stderr, mirroring
// anchorMaxStdoutBytes's rationale: a runaway block (an accidental `yes`)
// must not grow memory unbounded.
const feasibilityMaxOutputBytes = 64 * 1024

// feasibilityWaitDelay bounds how long Run waits for the output pipe to close
// after the process group is killed. Without it a leaked grandchild holding
// the pipe open would hang create indefinitely.
const feasibilityWaitDelay = 2 * time.Second

// flagSkipVerifyPredicates is the documented escape hatch for the rare
// justified case (authoring on a machine that cannot run the predicate at
// all). It is package-level because both `create` and `promote` bind it and
// the gate itself lives here; only one of the two ever runs per process, and
// each command rebuild re-applies the false default.
var flagSkipVerifyPredicates bool

// skipVerifyPredicatesFlagUsage is the shared --skip-verify-predicates help
// text; both create and promote register the flag.
const skipVerifyPredicatesFlagUsage = "do NOT execute the issue body's `## Verification` bash blocks (escape hatch; the created issue then ships an unproven predicate)"

// blockRun is the observed outcome of executing one Verification block.
// runErr is set only when the block produced no usable exit status at all
// (bash could not start, or a leaked child held the output pipe past
// feasibilityWaitDelay); exit is meaningful otherwise.
type blockRun struct {
	exit     int
	output   string
	timedOut bool
	runErr   error
}

// runFeasibilityGate executes every ```bash block under the issue body's
// Verification → Direct and Indirect subsections in the authoring environment
// and judges each by its exit status, so a predicate that has never actually
// run cannot ship as a green Iron Law gate (anvil.0196).
//
// The verdict is asymmetric, because the two subsections are expected to be in
// opposite states at authoring time:
//
//   - Indirect asserts POST-fix behaviour (docs/issue-spec.md), so a healthy
//     Indirect block is RED until the fix lands. Exit 0 is therefore the
//     failure: a predicate that already passes against the unmodified tree
//     cannot discriminate fixed from broken, and would report green the moment
//     a worker claims the issue. That is the false-green class this gate
//     exists to kill — anvil.0170's `anvil list issue` without `--ready` and
//     anvil.0162's `go test -tags integration` against a repo with no
//     integration-tagged file both exited 0 pre-fix.
//   - Direct is typically the repo's existing suite (`just check`), which is
//     green at authoring time, so exit 0 must not be refused there.
//
// Both subsections refuse on 126/127 — command not found or not executable
// means the predicate never ran, so its status says nothing about the code
// (anvil.0193's `./anvil install agents` when the binary is `bin/anvil`).
//
// Each block runs as one script under `set -e`, matching completing-issue's
// run-verification.sh semantics. A subsection with no fenced block is silently
// skipped — presence enforcement is ValidateIssue's job, not this gate's.
func runFeasibilityGate(cmd *cobra.Command, path, body string) []*errfmt.ValidationError {
	var errs []*errfmt.ValidationError
	for _, label := range []string{"Direct", "Indirect"} {
		blocks, err := core.VerificationBlocks(body, label)
		if err != nil {
			errs = append(errs, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", err.Error()).
				WithFix("close the ``` fence so every block can be extracted and run"))
			continue
		}
		for i, block := range blocks {
			name := fmt.Sprintf("verification %s block %d", label, i+1)
			cmd.PrintErrln("anvil: running " + name + " in this environment (your privileges, cwd and environment; not sandboxed)")
			r := runFeasibilityBlock(block)
			if r.timedOut && label == "Direct" {
				cmd.PrintErrln("anvil: " + name + " did not finish within " + feasibilityTimeout.String() + "; accepted unjudged (Direct is only checked for runnability)")
			}
			msg, fix := classifyFeasibility(label, name, r)
			if msg == "" {
				continue
			}
			if r.output != "" {
				msg += "\n" + r.output
			}
			errs = append(errs, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", msg).WithFix(fix))
		}
	}
	return errs
}

// classifyFeasibility maps one block's observed outcome to a refusal message
// and its fix, or ("", "") to accept. See runFeasibilityGate for why the two
// labels are judged differently.
func classifyFeasibility(label, name string, r blockRun) (msg, fix string) {
	switch {
	case r.runErr != nil:
		return fmt.Sprintf("%s produced no usable exit status: %v", name, r.runErr),
			"the gate could not judge the predicate — check that the block does not background work outliving it, then retry"
	case r.timedOut && label != "Indirect":
		// Direct is only checked for runnability, and a timeout is not an
		// unrunnable command; accept rather than make a slow suite unfileable.
		return "", ""
	case r.timedOut:
		return fmt.Sprintf("%s did not finish within %s, so the gate cannot classify it", name, feasibilityTimeout),
			"slim the block to the one command that proves the predicate; a full suite belongs in Direct"
	case r.exit == 127:
		return fmt.Sprintf("%s is unrunnable here (exit 127: command not found)", name),
			"fix the command name or path — the predicate never ran, so its exit status says nothing about the code"
	case r.exit == 126:
		return fmt.Sprintf("%s is unrunnable here (exit 126: found but not executable)", name),
			"point the block at an executable path — the predicate never ran, so its exit status says nothing about the code"
	case label != "Indirect":
		return "", ""
	case r.exit == 0:
		return fmt.Sprintf("%s already passes (exit 0) against the current, unfixed tree, so it cannot discriminate fixed from broken", name),
			"write an Indirect predicate that is red until the fix lands: assert the observed post-fix behaviour, not the presence of a mechanism. " +
				"A block ending in `! <cmd>` also exits 0 when <cmd> is missing — check the captured output before assuming the predicate ran"
	}
	return "", ""
}

// runFeasibilityBlock runs a single Verification block's lines as one bash
// script and reports what it observed. It never decides pass/fail — that is
// classifyFeasibility's job.
func runFeasibilityBlock(block string) blockRun {
	ctx, cancel := context.WithTimeout(context.Background(), feasibilityTimeout)
	defer cancel()

	// /bin/bash, not a $PATH lookup: the same pinned-shell precedent
	// runAnchorCheck sets, so the gate's verdict does not depend on which bash
	// happens to come first on the author's PATH.
	c := exec.CommandContext(ctx, "/bin/bash", "-ec", block) //nolint:gosec // G204: runs the issue's own Verification block verbatim by design — proving it is what the feasibility gate (anvil.0196) exists to do; author-trusted vault content, bounded by feasibilityTimeout
	// Run the block in its own process group so the timeout kill reaches the
	// whole tree. A block that backgrounds work (`nohup … &`) leaves
	// grandchildren that survive a signal aimed at bash alone and keep running
	// (and holding the output pipe) long after create returns.
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGKILL) }
	c.WaitDelay = feasibilityWaitDelay
	out := &capWriter{cap: feasibilityMaxOutputBytes}
	c.Stdout = out
	c.Stderr = out
	runErr := c.Run()

	tail := out.buf.String()
	if out.truncated {
		tail += "\n(output truncated)"
	}
	r := blockRun{output: tail}
	var exitErr *exec.ExitError
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		r.timedOut = true
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		r.exit = exitErr.ExitCode()
	default:
		r.runErr = runErr
	}
	return r
}

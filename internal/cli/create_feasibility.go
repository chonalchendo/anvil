package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
// each command rebuild re-applies the false default. Those two verbs are the
// gate's only reachable callers — `append` validates addenda through
// staticBodyFailures and never executes Verification blocks, so the flag
// does not exist there.
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
	// redLine is the 1-based block line a set -e abort stopped on, or 0 when
	// the block ended another way (explicit exit, success, kill).
	redLine int
	// lastLine is the 1-based line of the block's final command.
	lastLine int
	// earlyRed is true when the set -e abort hit a complete command before
	// lastLine, i.e. setup failed rather than the assertion.
	earlyRed bool
	// redText is the trimmed text of redLine.
	redText string
}

// redLineEnv names the side file the ERR trap writes $LINENO to. A file, not
// the block's stderr: a sentinel in the shared output stream can leak into the
// text shown to the author and is lost when capWriter truncates the tail.
// An explicit `exit`, a `|| true` guard and an `if <cmd>` condition do not
// fire the trap; only a set -e abort does, which is what separates a broken
// setup line from a deliberate red.
const redLineEnv = "ANVIL_RED_LINE_FILE"

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
// run-verification.sh semantics — except a block carrying a non-last-line
// `!` assertion, which is refused unrun (core.NonGatingNegation) because
// `set -e` would exempt it. A subsection with no fenced block is skipped —
// presence enforcement is ValidateIssue's job, not this gate's.
//
// Indirect also refuses a set -e abort before the block's last line: the red
// comes from setup, not the assertion.
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
			if vacuous := core.NonGatingNegation(block); vacuous != "" {
				errs = append(errs, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "",
					fmt.Sprintf("%s carries `%s`: %s", name, vacuous, nonGatingNegationWhy)).
					WithFix(nonGatingNegationFix))
				continue
			}
			cmd.PrintErrln("anvil: running " + name + " in this environment (your privileges, cwd and environment; not sandboxed)")
			r := runFeasibilityBlock(block, "")
			if r.timedOut && label == "Direct" {
				cmd.PrintErrln("anvil: " + name + " did not finish within " + feasibilityTimeout.String() + "; accepted unjudged (Direct is only checked for runnability)")
			}
			if label == "Direct" && r.runErr == nil && !r.timedOut && r.exit == 0 {
				cmd.PrintErrln("anvil: " + name + " exits 0 — " + directGreenNote)
			}
			if label == "Direct" && r.redLine > 0 && !r.timedOut && r.exit != 126 && r.exit != 127 {
				cmd.PrintErrln(fmt.Sprintf("anvil: %s exits non-zero at line %d (`%s`); accepted, but check it is not a missing path or an empty suite", name, r.redLine, r.redText))
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

// The one refusal wording for a non-gating negation, shared by the create-time
// gate and `anvil validate --verification-stdin` so the pre-flight lint and the
// create reject read the same (convention.cli-tooling rule 5).
const (
	nonGatingNegationWhy = "a `!` in command position on a line that is not the block's last. " +
		"set -e exempts a non-final `!` command, so its failure cannot fail the block; " +
		"and where a loop or if tail does gate, only its final iteration's status survives"
	earlyRedFix          = "move the setup fix so the block reaches its assertion, or make the assertion the block's last line; split independent assertions into separate blocks"
	nonGatingNegationFix = "rewrite the negative assertion as `if <cmd>; then exit 1; fi`, which gates on any line — or make it the block's last line"
)

// directGreenNote is a warning, not a refusal: a green Direct is the healthy
// state for a suite invocation, but it is indistinguishable from an
// issue-specific check that already passes before the change.
const directGreenNote = "it passes against the unfixed tree, so it proves nothing about this change; " +
	"put any check of the new behaviour under Indirect, where exit 0 is refused"

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
	case r.earlyRed:
		return fmt.Sprintf("%s aborts at line %d (`%s`) before its last line (%d), so it is red for a setup reason, not the assertion", name, r.redLine, r.redText, r.lastLine),
			earlyRedFix
	case r.exit == 0:
		return fmt.Sprintf("%s already passes (exit 0) against the current, unfixed tree, so it cannot discriminate fixed from broken", name),
			"write an Indirect predicate that is red until the fix lands: assert the observed post-fix behaviour, not the presence of a mechanism. " +
				"A block ending in `! <cmd>` also exits 0 when <cmd> is missing — check the captured output before assuming the predicate ran"
	}
	return "", ""
}

// runFeasibilityBlock runs a single Verification block's lines as one bash
// script in dir ("" = the process cwd) and reports what it observed. It never decides pass/fail — that is
// classifyFeasibility's job.
func runFeasibilityBlock(block, dir string) blockRun {
	ctx, cancel := context.WithTimeout(context.Background(), feasibilityTimeout)
	defer cancel()

	// A script file, not -c: bash 3.2 numbers -c lines from 0, so $LINENO would
	// disagree across the bash versions the gate may meet.
	script, err := os.CreateTemp("", "anvil-verify-*.sh")
	if err != nil {
		return blockRun{runErr: err}
	}
	defer func() { _ = os.Remove(script.Name()) }()
	redFile, err := os.CreateTemp("", "anvil-redline-*")
	if err != nil {
		_ = script.Close()
		return blockRun{runErr: err}
	}
	_ = redFile.Close()
	defer func() { _ = os.Remove(redFile.Name()) }()
	// The $- guard keeps a block's own `set +e` from reporting every failing command.
	_, werr := script.WriteString("trap 'case $- in *e*) printf %s \"$LINENO\" > \"$" + redLineEnv + "\";; esac' ERR; " + block)
	if cerr := script.Close(); werr != nil || cerr != nil {
		return blockRun{runErr: errors.Join(werr, cerr)}
	}
	// /bin/bash, not a $PATH lookup: the same pinned-shell precedent
	// runAnchorCheck sets, so the gate's verdict does not depend on which bash
	// happens to come first on the author's PATH.
	c := exec.CommandContext(ctx, "/bin/bash", "-e", script.Name()) //nolint:gosec // G204: runs the issue's own Verification block verbatim by design — proving it is what the feasibility gate (anvil.0196) exists to do; author-trusted vault content, bounded by feasibilityTimeout
	// Run the block in its own process group so the timeout kill reaches the
	// whole tree. A block that backgrounds work (`nohup … &`) leaves
	// grandchildren that survive a signal aimed at bash alone and keep running
	// (and holding the output pipe) long after create returns.
	c.Dir = dir // "" keeps the process cwd
	c.Env = append(os.Environ(), redLineEnv+"="+redFile.Name())
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
	if b, rerr := os.ReadFile(redFile.Name()); rerr == nil {
		r.redLine, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	}
	r.lastLine, r.redText, r.earlyRed = blockLines(block, r.redLine)
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

// blockLines returns the 1-based line where the block's final command starts
// (trailing blanks and comments dropped, backslash continuations folded in),
// the trimmed text of line red, and whether the abort at red is early. It is
// early only when lines[:red] parse cleanly as a script and a later command
// exists: bash 5 reports LINENO 1 inside a heredoc or multi-line quote, and a
// compound command reports its opening line, so those prefixes do not parse.
func blockLines(block string, red int) (last int, redText string, early bool) {
	lines := strings.Split(strings.TrimRight(block, " \t\r\n"), "\n")
	last = len(lines)
	for last > 1 && isBlankOrComment(lines[last-1]) {
		last--
	}
	for last > 1 && strings.HasSuffix(strings.TrimRight(lines[last-2], " \t"), "\\") {
		last--
	}
	if red < 1 || red > len(lines) {
		return last, "", false
	}
	redText = strings.TrimSpace(lines[red-1])
	if red >= last {
		return last, redText, false
	}
	return last, redText, parsesCleanly(strings.Join(lines[:red], "\n"))
}

func isBlankOrComment(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "#")
}

// parsesCleanly reports whether /bin/bash -n accepts src with no diagnostics.
func parsesCleanly(src string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), feasibilityWaitDelay)
	defer cancel()
	c := exec.CommandContext(ctx, "/bin/bash", "-n") //nolint:gosec // G204: fixed argv, the script arrives on stdin and is only parsed
	c.Stdin = strings.NewReader(src)
	out, err := c.CombinedOutput()
	return err == nil && len(out) == 0
}

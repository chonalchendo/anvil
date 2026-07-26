package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/chonalchendo/anvil/internal/core"
)

// feasibilityTimeout caps a single Verification block. The gate fires
// synchronously at create time, so a block that legitimately needs longer
// (a full `just check`) should be slimmed to the one command that proves
// feasibility — the same guidance writing-issue already gives authors.
const feasibilityTimeout = 60 * time.Second

// feasibilityMaxOutputBytes caps captured combined stdout+stderr, mirroring
// anchorMaxStdoutBytes's rationale: a runaway block (an accidental `yes`)
// must not grow memory unbounded.
const feasibilityMaxOutputBytes = 64 * 1024

// runFeasibilityGate executes every ```bash block under the issue body's
// Verification → Direct and Indirect subsections in the authoring
// environment, so a predicate that has never actually run cannot ship as a
// green Iron Law gate (anvil.0196). Each block runs as one script under `set
// -e`, matching completing-issue's run-verification.sh semantics. Returns one
// error per block that fails to execute (non-zero exit, or timeout); nil when
// every present block passes. A subsection with no fenced block is silently
// skipped — presence enforcement is ValidateIssue's job, not this gate's.
func runFeasibilityGate(body string) []error {
	var errs []error
	for _, label := range []string{"Direct", "Indirect"} {
		for i, block := range core.VerificationBlocks(body, label) {
			if err := runFeasibilityBlock(block); err != nil {
				errs = append(errs, fmt.Errorf("verification %s block %d did not pass in this environment: %w", label, i+1, err))
			}
		}
	}
	return errs
}

// runFeasibilityBlock runs a single Verification block's lines as one bash
// script and reports the first failure (or a timeout) it hits.
func runFeasibilityBlock(block string) error {
	ctx, cancel := context.WithTimeout(context.Background(), feasibilityTimeout)
	defer cancel()

	c := exec.CommandContext(ctx, "bash", "-ec", block) //nolint:gosec // G204: runs the issue's own Verification block verbatim by design — proving it is what the feasibility gate (anvil.0196) exists to do; author-trusted vault content, bounded by feasibilityTimeout
	out := &capWriter{cap: feasibilityMaxOutputBytes}
	c.Stdout = out
	c.Stderr = out
	runErr := c.Run()
	if runErr == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timed out after %s", feasibilityTimeout)
	}
	tail := out.buf.String()
	if out.truncated {
		tail += "\n(output truncated)"
	}
	return fmt.Errorf("%w\n%s", runErr, tail)
}

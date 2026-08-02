package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// runVerificationStdinLint reads a Verification predicate's raw bash text from
// stdin and reports any hardcoded checkout path it contains (see
// core.CheckoutPathMatches). Distinct from the vault-wide validate path: this
// mode takes no vault, no artifact — just the predicate text a fleet worker or
// author wants checked before running it. Emits the same envelope as the
// vault-wide scan on both paths — an empty ValidationError array on success,
// emitValidationErrors' shape on failure — so `anvil validate --json` has one
// schema regardless of mode, per convention.cli-tooling's "--json mandatory on
// read-shape verbs" rule.
func runVerificationStdinLint(cmd *cobra.Command, asJSON bool) error {
	data, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	var failures []*errfmt.ValidationError
	for _, h := range core.CheckoutPathMatches(string(data)) {
		failures = append(failures, errfmt.NewValidationError(errfmt.CodeConstraintViolation, "-", "",
			fmt.Sprintf("checkout path: verification block hardcodes a checkout path (%q)", h)).
			WithFix("anchor with $(git rev-parse --show-toplevel) instead so a dispatched worktree verifies itself, not the main checkout"))
	}
	if len(failures) == 0 {
		if asJSON {
			b, _ := json.Marshal([]*errfmt.ValidationError{})
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
		} else {
			cmd.PrintErrln("checkout path lint: no hardcoded checkout path found")
		}
		return nil
	}
	if !asJSON {
		cmd.PrintErrf("checkout path lint: %d hardcoded checkout path(s) found\n", len(failures))
	}
	return emitValidationErrors(cmd, asJSON, failures)
}

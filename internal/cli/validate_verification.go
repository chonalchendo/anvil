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
// core.CheckoutPathMatches), any hardcoded lakehouse schema (see
// core.LakehouseSchemaMatches), plus any non-gating `!` assertion (see
// core.NonGatingNegation) — the same rules `anvil create issue` enforces,
// so this pre-flight lint never says clean on text create would reject.
// Distinct from the vault-wide validate path: this
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
	for _, h := range core.LakehouseSchemaMatches(string(data)) {
		failures = append(failures, errfmt.NewValidationError(errfmt.CodeConstraintViolation, "-", "",
			fmt.Sprintf("lakehouse schema: verification block hardcodes a lakehouse schema (%q)", h)).
			WithFix("parameterise with ${SCHEMA:-modelled} so a worker without prod-apply rights can satisfy it"))
	}
	if vacuous := core.NonGatingNegation(string(data)); vacuous != "" {
		failures = append(failures, errfmt.NewValidationError(errfmt.CodeConstraintViolation, "-", "",
			fmt.Sprintf("non-gating negation: verification block carries `%s`: %s", vacuous, nonGatingNegationWhy)).
			WithFix(nonGatingNegationFix))
	}
	if len(failures) == 0 {
		if asJSON {
			b, _ := json.Marshal([]*errfmt.ValidationError{})
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
		} else {
			cmd.PrintErrln("verification lint: no hardcoded checkout path, no hardcoded lakehouse schema, no non-gating negation found")
		}
		return nil
	}
	if !asJSON {
		cmd.PrintErrf("verification lint: %d finding(s)\n", len(failures))
	}
	return emitValidationErrors(cmd, asJSON, failures)
}

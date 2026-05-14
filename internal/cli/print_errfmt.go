package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
)

// augmentFacetErrors enriches missing_required_facet errors with vault-aware
// context: the known values for the relevant facet (so the agent can pick an
// existing one), or — when the facet bucket is empty — a hint that
// `--allow-new-facet=<facet>` is required because the value will be novel.
// This is the coalescing step that collapses the fresh-vault round-trip from
// missing-facet → unknown-facet-value into a single error block.
func augmentFacetErrors(errs []*errfmt.ValidationError, vaultValues map[string]map[string]struct{}) {
	for _, e := range errs {
		if e.Code != errfmt.CodeMissingRequiredFacet || e.Field != "tags" {
			continue
		}
		facet := facetFromExpected(e.Expected)
		if facet == "" {
			continue
		}
		bucket := vaultValues[facet]
		example := facet + "/<x>"
		if len(bucket) == 0 {
			e.WithNote(fmt.Sprintf("no %s/* tags in vault yet — first value will be novel", facet)).
				WithFix(fmt.Sprintf(
					"add --tags %s and pass --allow-new-facet=%s (this is the first %s/* in the vault)",
					example, facet, facet,
				))
			continue
		}
		known := make([]string, 0, len(bucket))
		for v := range bucket {
			known = append(known, facet+"/"+v)
		}
		sort.Strings(known)
		e.WithFix(fmt.Sprintf(
			"add a tag from --tags %v, or pass --allow-new-facet=%s to introduce a new one",
			known, facet,
		))
	}
}

// facetFromExpected reads the pattern out of a missing_required_facet error's
// Expected field (a []string of one pattern) and returns the facet prefix.
func facetFromExpected(expected any) string {
	patterns, ok := expected.([]string)
	if !ok || len(patterns) == 0 {
		return ""
	}
	return facetNameFromPattern(patterns[0])
}

func printValidationErrors(cmd *cobra.Command, errs []*errfmt.ValidationError) {
	for i, f := range errs {
		if i > 0 {
			cmd.PrintErrln("")
		}
		cmd.PrintErrln(fmt.Sprintf("[%s] %s", f.Code, f.Path))
		if f.Field != "" {
			cmd.PrintErrln(fmt.Sprintf("  field: %s", f.Field))
		}
		if f.Got != "" {
			cmd.PrintErrln(fmt.Sprintf("  got: %s", f.Got))
		}
		if f.Suggest != "" {
			cmd.PrintErrln(fmt.Sprintf("  suggest: %s", f.Suggest))
		}
		if f.Expected != nil {
			cmd.PrintErrln(fmt.Sprintf("  expected: %v", f.Expected))
		}
		if f.Note != "" {
			cmd.PrintErrln(fmt.Sprintf("  note: %s", f.Note))
		}
		if f.Fix != "" {
			cmd.PrintErrln(fmt.Sprintf("  fix: %s", f.Fix))
		}
	}
}

// printValidationErrorsJSON emits the stable schema-invalid envelope to stdout
// for `--json` callers of create/set/promote. Shape:
//
//	{"error":"schema_invalid","violations":[<ValidationError>...]}
//
// The envelope is an object (not a bare array) so it is distinguishable from
// success envelopes — agents dispatch on the top-level `error` key, then walk
// `violations[]` to correct fields without a non-JSON debug round-trip.
func printValidationErrorsJSON(cmd *cobra.Command, errs []*errfmt.ValidationError) {
	if errs == nil {
		errs = []*errfmt.ValidationError{}
	}
	payload := map[string]any{
		"error":      "schema_invalid",
		"violations": errs,
	}
	b, _ := json.Marshal(payload)
	// cobra's cmd.Println falls back to stderr; route through OutOrStdout so
	// the envelope lands on stdout next to the success envelopes — agents
	// parse one stream for both outcomes.
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
}

// emitValidationErrors is the dispatch helper: JSON envelope when asJSON, the
// human-readable block otherwise. Centralised so create/set/promote agree on
// the contract for both axes.
func emitValidationErrors(cmd *cobra.Command, asJSON bool, errs []*errfmt.ValidationError) {
	if asJSON {
		printValidationErrorsJSON(cmd, errs)
		return
	}
	printValidationErrors(cmd, errs)
}

// renderSchemaErr prints structured validation errors and returns
// ErrSchemaInvalid. When v is non-nil it walks the vault to enrich
// missing_required_facet errors with known facet values or the
// `--allow-new-facet` hint, so a single error block tells the agent every
// constraint they need to satisfy. Pass nil for v when no vault is available
// (the error block remains correct, just less specific). When asJSON, the
// envelope goes to stdout in the shape documented on printValidationErrorsJSON
// — agents parse it instead of re-running without --json to read the human
// block.
func renderSchemaErr(cmd *cobra.Command, v *core.Vault, path string, err error, asJSON bool) error {
	errs := schemaErrToValidationErrors(path, err)
	if v != nil {
		if vals, skipped, gErr := facets.CollectValues(v.Root); gErr == nil {
			augmentFacetErrors(errs, vals)
			for _, p := range skipped {
				cmd.PrintErrln("warn: skipped corrupt artifact during facet walk: " + p)
			}
		}
	}
	emitValidationErrors(cmd, asJSON, errs)
	return ErrSchemaInvalid
}

package cli

import (
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

// renderSchemaErr prints structured validation errors and returns
// ErrSchemaInvalid. When v is non-nil it walks the vault to enrich
// missing_required_facet errors with known facet values or the
// `--allow-new-facet` hint, so a single error block tells the agent every
// constraint they need to satisfy. Pass nil for v when no vault is available
// (the error block remains correct, just less specific).
func renderSchemaErr(cmd *cobra.Command, v *core.Vault, path string, err error) error {
	errs := schemaErrToValidationErrors(path, err)
	if v != nil {
		if vals, skipped, gErr := facets.CollectValues(v.Root); gErr == nil {
			augmentFacetErrors(errs, vals)
			for _, p := range skipped {
				cmd.PrintErrln("warn: skipped corrupt artifact during facet walk: " + p)
			}
		}
	}
	printValidationErrors(cmd, errs)
	return ErrSchemaInvalid
}

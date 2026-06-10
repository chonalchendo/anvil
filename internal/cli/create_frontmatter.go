package cli

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
	"github.com/chonalchendo/anvil/internal/templates"
)

// validateBeforeCreate runs every create-time validation layer against the
// in-memory artifact and emits all violations in one block: schema frontmatter
// (including missing-facet patterns and required scalars), facet novelty, and —
// when authoredBody — body required-headings plus wikilink resolution. Walking
// every layer instead of short-circuiting at the first failure lets a single
// create surface every blocking class, so the author pays one round-trip, not
// one per layer.
//
// preErrors holds caller-collected violations (e.g. missing required CLI flags
// that can't be expressed in the JSON Schema) that are prepended before schema
// errors so the full set is emitted in a single block.
//
// Returns ErrSchemaInvalid (after emitting the violations block) when any layer
// fails, a usage error for an unknown --allow-new-facet name, or nil when the
// artifact is clean and safe to write.
func validateBeforeCreate(cmd *cobra.Command, v *core.Vault, t core.Type, path string, fm map[string]any, body string, authoredBody bool, allowNewFacet []string, asJSON bool, preErrors ...*errfmt.ValidationError) error {
	for _, f := range allowNewFacet {
		if !facets.Has(f) {
			return formatEnumError("--allow-new-facet", f, facets.Names(), "")
		}
	}
	values, skipped, gErr := facets.CollectValues(v.Root)
	if gErr != nil {
		return fmt.Errorf("walking vault for facet values: %w", gErr)
	}
	for _, p := range skipped {
		cmd.PrintErrln("warn: skipped corrupt artifact during facet walk: " + p)
	}

	failures := append([]*errfmt.ValidationError(nil), preErrors...)

	if err := schema.Validate(string(t), fm); err != nil {
		errs := schemaErrToValidationErrors(path, err)
		augmentFacetErrors(errs, values)
		failures = append(failures, errs...)
	}

	allowed := make(map[string]bool, len(allowNewFacet))
	for _, f := range allowNewFacet {
		allowed[f] = true
	}
	tagsRaw, _ := fm["tags"].([]any)
	tagsStr := make([]string, 0, len(tagsRaw))
	for _, raw := range tagsRaw {
		if s, ok := raw.(string); ok {
			tagsStr = append(tagsStr, s)
		}
	}
	for _, e := range facets.Check(values, tagsStr, allowed) {
		e.Path = path
		failures = append(failures, e)
	}

	// A contract's `kind` is a registered label, not a free scalar: it must
	// already exist in the glossary `kind/` vocabulary (register via `anvil
	// contract kinds add`). Mirrors the tag-facet gate — an unregistered kind
	// is rejected, not silently accepted — keeping the kind set typo-safe and
	// discoverable for the writing-contract skill.
	if t == core.TypeContract {
		if e := checkContractKind(v.Root, path, fm); e != nil {
			failures = append(failures, e)
		}
	}

	if authoredBody {
		a := &core.Artifact{Path: path, FrontMatter: fm, Body: body}
		switch t {
		case core.TypeIssue:
			for _, vErr := range core.ValidateIssue(a) {
				failures = append(failures, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
			}
		case core.TypeLearning:
			for _, vErr := range core.ValidateLearning(a, nil) {
				failures = append(failures, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
			}
		}
		for _, link := range core.ResolveBodyLinks(v, body) {
			failures = append(failures, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "body", fmt.Sprintf("unresolved wikilink [[%s]]", link.Target)).
				WithFix("create the target artifact or remove the wikilink"))
		}
	}

	if len(failures) == 0 {
		return nil
	}
	emitValidationErrors(cmd, asJSON, failures)
	return ErrSchemaInvalid
}

// renderFrontMatter executes the template for t against data and parses the
// result into a map suitable for schema validation and artifact storage.
func renderFrontMatter(t core.Type, data templateData) (map[string]any, error) {
	src, err := templates.FS.ReadFile(string(t) + ".tmpl")
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", t, err)
	}
	tmpl, err := template.New(string(t)).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parsing template %s: %w", t, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %s: %w", t, err)
	}
	var fm map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &fm); err != nil {
		return nil, fmt.Errorf("parsing rendered YAML: %w", err)
	}
	// yaml.v3 parses YYYY-MM-DD scalars as time.Time; convert them back to
	// date strings so the JSON Schema validator sees a plain string.
	normaliseDates(fm)
	return fm, nil
}

// normaliseDates walks fm and replaces any time.Time value with its
// YYYY-MM-DD string representation. yaml.v3 auto-converts date scalars.
func normaliseDates(fm map[string]any) {
	for k, v := range fm {
		if t, ok := v.(time.Time); ok {
			fm[k] = t.UTC().Format("2006-01-02")
		}
	}
}

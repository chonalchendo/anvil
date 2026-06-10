package cli

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// maxDescriptionChars mirrors the `maxLength: 120` cap in every spine-type
// schema (issue, plan, milestone, decision, sweep, product-design,
// system-design). Pre-flighted here so the CLI rejects oversize descriptions
// before any template rendering or facet walk, with a single focused error.
const maxDescriptionChars = 120

// maxGoalChars bounds an issue's `goal:` — the one-sentence terminal predicate.
// Same cap as description: it is a spine field, not a place for prose.
const maxGoalChars = 120

// checkFieldCaps runs the capped-field checks before vault/project resolution
// so cap feedback fast-fails for every type, in or out of a vault. The
// description cap applies to all spine types; the issue path also collects its
// goal-length overage so the author sees every violation in one rejection
// rather than one per resubmit.
func checkFieldCaps(t core.Type, description, goal string) error {
	if t == core.TypeSession {
		return nil
	}
	var capErrs []error
	if n := utf8.RuneCountInString(description); n > maxDescriptionChars {
		capErrs = append(capErrs, fmt.Errorf(
			"--description too long: %d chars (max %d); description is spine index/preview text, not docs — re-summarise to fit the cap rather than raise it",
			n, maxDescriptionChars,
		))
	}
	if (t == core.TypeIssue || t == core.TypeMilestone) && strings.TrimSpace(goal) != "" {
		if n := utf8.RuneCountInString(goal); n > maxGoalChars {
			capErrs = append(capErrs, fmt.Errorf(
				"--goal too long: %d chars (max %d); goal is a one-sentence predicate, not docs — tighten it",
				n, maxGoalChars,
			))
		}
	}
	return errors.Join(capErrs...)
}

// collectPreValidationErrors applies the per-type required-flag checks, three
// tiers:
//
//   - Schema-owned: flags that fill a schema-required scalar
//     (issue/milestone --goal, sweep --scope, contract --kind) get
//     no CLI-level check. Their empty render is stripped in the create
//     path so schema.Validate reports them as missing_required in the
//     same aggregated block as facet and body violations;
//     requiredFlagFix re-attaches the flag hint.
//   - Deferred: requirements the schema cannot express — decision
//     --topic (an ID/path input, not a frontmatter field) and
//     sweep's explicit --breaking (false is schema-valid) — are
//     collected here and prepended to that same block by
//     validateBeforeCreate.
//   - Short-circuit: plan --issue is the named remaining
//     exception — the plan's default slug, and so its id and path,
//     derive from the issue link, so nothing downstream is
//     meaningful without it.
func collectPreValidationErrors(cmd *cobra.Command, t core.Type, issue, topic string) ([]*errfmt.ValidationError, error) {
	var preValidationErrors []*errfmt.ValidationError
	switch t {
	case core.TypePlan:
		if issue == "" {
			return nil, fmt.Errorf("--issue is required for plan")
		}
	case core.TypeSweep:
		if !cmd.Flags().Changed("breaking") {
			preValidationErrors = append(preValidationErrors,
				errfmt.NewValidationError(errfmt.CodeMissingRequired, "", "breaking", "").
					WithExpected("--breaking must be set explicitly for sweep (true or false)"))
		}
	case core.TypeDecision:
		if topic == "" {
			preValidationErrors = append(preValidationErrors,
				errfmt.NewValidationError(errfmt.CodeMissingRequired, "", "topic", "").
					WithExpected("--topic is required for decision"))
		}
	}
	return preValidationErrors, nil
}

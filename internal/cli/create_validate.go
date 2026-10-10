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
// schema (issue, milestone, decision, sweep, product-design,
// system-design). Pre-flighted here so the CLI rejects oversize descriptions
// before any template rendering or facet walk, with a single focused error.
const maxDescriptionChars = 120

// maxGoalChars bounds an issue's `goal:` — the one-sentence terminal predicate.
// Same cap as description: it is a spine field, not a place for prose.
const maxGoalChars = 120

// capViolation is one over-cap field. Typed so the create call site renders
// text errors or the JSON envelope; the check itself knows no output mode.
type capViolation struct {
	Code  string `json:"code"`
	Field string `json:"field"`
	Got   int    `json:"got"`
	Max   int    `json:"max"`
	Fix   string `json:"fix"`
	text  string
}

// checkFieldCaps runs the capped-field checks before vault/project resolution
// so cap feedback fast-fails for every type, in or out of a vault. The
// description cap applies to all spine types; the issue path also collects its
// goal-length overage so the author sees every violation in one rejection
// rather than one per resubmit.
func checkFieldCaps(t core.Type, description, goal string) []capViolation {
	if t == core.TypeSession {
		return nil
	}
	var vs []capViolation
	if n := utf8.RuneCountInString(description); n > maxDescriptionChars {
		vs = append(vs, capViolation{
			Code: "description_too_long", Field: "description", Got: n, Max: maxDescriptionChars,
			Fix: "re-summarise --description to fit the cap; it is spine index/preview text, not docs",
			text: fmt.Sprintf(
				"--description too long: %d chars (max %d); description is spine index/preview text, not docs — re-summarise to fit the cap rather than raise it",
				n, maxDescriptionChars),
		})
	}
	if (t == core.TypeIssue || t == core.TypeMilestone) && strings.TrimSpace(goal) != "" {
		if n := utf8.RuneCountInString(goal); n > maxGoalChars {
			vs = append(vs, capViolation{
				Code: "goal_too_long", Field: "goal", Got: n, Max: maxGoalChars,
				Fix: "tighten --goal to one sentence within the cap",
				text: fmt.Sprintf(
					"--goal too long: %d chars (max %d); goal is a one-sentence predicate, not docs — tighten it",
					n, maxGoalChars),
			})
		}
	}
	return vs
}

// collectPreValidationErrors applies the per-type required-flag checks, two
// tiers:
//
//   - Schema-owned: flags that fill a schema-required scalar
//     (issue/milestone --goal, sweep --scope, component design --kind) get
//     no CLI-level check. Their empty render is stripped in the create
//     path so schema.Validate reports them as missing_required in the
//     same aggregated block as facet and body violations;
//     requiredFlagFix re-attaches the flag hint.
//   - Deferred: requirements the schema cannot express — decision
//     --topic (an ID/path input, not a frontmatter field) and
//     sweep's explicit --breaking (false is schema-valid) — are
//     collected here and prepended to that same block by
//     validateBeforeCreate.
func collectPreValidationErrors(cmd *cobra.Command, t core.Type, topic string) []*errfmt.ValidationError {
	var preValidationErrors []*errfmt.ValidationError
	switch t {
	case core.TypeSweep:
		if !cmd.Flags().Changed("breaking") {
			preValidationErrors = append(preValidationErrors,
				errfmt.NewValidationError(errfmt.CodeMissingRequired, "", "breaking", "").
					WithExpected("--breaking must be set explicitly for sweep (true or false)"))
		}
	case core.TypeDecision, core.TypeThread:
		if topic == "" {
			preValidationErrors = append(preValidationErrors,
				errfmt.NewValidationError(errfmt.CodeMissingRequired, "", "topic", "").
					WithExpected(fmt.Sprintf("--topic is required for %s", t)))
		}
	}
	return preValidationErrors
}

// preResolutionRefusal reports every pre-resolution violation at once: joined
// text errors, or under --json the one schema_invalid envelope.
func preResolutionRefusal(cmd *cobra.Command, asJSON bool, t core.Type, missingTitle bool, caps []capViolation) error {
	if asJSON {
		var vs []any
		if missingTitle {
			vs = append(vs, errfmt.NewValidationError(errfmt.CodeMissingRequired, "", "title", "").
				WithFix(fmt.Sprintf("pass --title; it is required for %s", t)))
		}
		for _, c := range caps {
			vs = append(vs, c)
		}
		return emitValidationErrorsJSON(cmd, vs)
	}
	var errs []error
	if missingTitle {
		errs = append(errs, fmt.Errorf("--title is required for %s", t))
	}
	for _, c := range caps {
		errs = append(errs, errors.New(c.text))
	}
	return errors.Join(errs...)
}

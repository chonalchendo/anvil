package cli

import (
	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// appendIssueTypeErrors runs the issue-type-specific validators — body-shape,
// stale-verb, and hardcoded-lakehouse-schema — and appends their findings to
// out.
func appendIssueTypeErrors(out []*errfmt.ValidationError, a *core.Artifact, path string, verbs core.VerbPathValidator, sweep bool) []*errfmt.ValidationError {
	for _, vErr := range core.ValidateIssue(a) {
		out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
	}
	goal, _ := a.FrontMatter["goal"].(string)
	title, _ := a.FrontMatter["title"].(string)
	for _, vErr := range core.ValidateIssueVerbs(a.Body, goal, title, verbs) {
		out = append(out, errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error()))
	}
	// ValidateIssueCheckoutPaths is deliberately NOT wired here: the
	// checkout-path lint gates create/promote only. The ~219 issues
	// authored before the rule would fail vault-hygiene CI retroactively
	// if the vault-wide scan enforced it. Lint a single predicate with
	// `anvil validate --verification-stdin` instead.
	return appendLakehouseSchemaErrors(out, a, path, sweep)
}

// leadSentenceHeading maps a type to the H2 heading its lead-sentence rule
// governs. Types outside this set (including learning, handled earlier and
// returned before this runs) carry no lead-sentence check.
var leadSentenceHeading = map[core.Type]string{
	core.TypeIssue:     "Problem",
	core.TypeMilestone: "Objective",
}

// appendLeadSentenceErrors runs core.ValidateLeadSentence against the
// heading t governs and appends its finding, always at SeverityWarning — the
// rule must never fail create or validate (writing-issue/writing-milestone
// prescribe it in prose only; this is the deterministic backstop).
func appendLeadSentenceErrors(out []*errfmt.ValidationError, t core.Type, a *core.Artifact, path string) []*errfmt.ValidationError {
	return append(out, leadSentenceFailures(t, a.Body, path)...)
}

// leadSentenceFailures runs core.ValidateLeadSentence for t's governing
// heading and wraps the result as SeverityWarning findings — the create-time
// counterpart to appendLeadSentenceErrors, called on the in-memory body
// before an Artifact exists.
func leadSentenceFailures(t core.Type, body, path string) []*errfmt.ValidationError {
	heading, ok := leadSentenceHeading[t]
	if !ok {
		return nil
	}
	var out []*errfmt.ValidationError
	for _, vErr := range core.ValidateLeadSentence(body, heading) {
		out = append(out, errfmt.NewValidationError(errfmt.CodeLeadSentence, path, "", vErr.Error()).
			WithSeverity(errfmt.SeverityWarning))
	}
	return out
}

// hasBlockingFailure reports whether failures contains any finding above
// SeverityWarning. A SeverityWarning finding is still emitted (both JSON and
// human output) but must not fail the run — the sweep's grandfather tier for
// a rule (e.g. the lakehouse-schema check below) that refuses outright at
// create/promote and single-file validate.
func hasBlockingFailure(failures []*errfmt.ValidationError) bool {
	for _, f := range failures {
		if f.Severity != errfmt.SeverityWarning {
			return true
		}
	}
	return false
}

// appendLakehouseSchemaErrors runs core.ValidateIssueLakehouseSchema on the
// artifact's body and appends the resulting errors to out.
//
// ValidateIssueLakehouseSchema IS wired here so a hand-edited Verification
// block never passes through create again unchecked; on the multi-file
// sweep it's a warning (retro-fixing the back catalogue is a non-goal),
// full refusal elsewhere.
func appendLakehouseSchemaErrors(out []*errfmt.ValidationError, a *core.Artifact, path string, sweep bool) []*errfmt.ValidationError {
	for _, vErr := range core.ValidateIssueLakehouseSchema(a.Body) {
		e := errfmt.NewValidationError(errfmt.CodeConstraintViolation, path, "", vErr.Error())
		if sweep {
			e = e.WithSeverity(errfmt.SeverityWarning)
		}
		out = append(out, e)
	}
	return out
}

package cli

import (
	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// appendIssueTypeErrors runs the issue-type-specific validators — body-shape
// and stale-verb — and appends their findings to out.
func appendIssueTypeErrors(out []*errfmt.ValidationError, a *core.Artifact, path string, verbs core.VerbPathValidator) []*errfmt.ValidationError {
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
	return out
}

// hasBlockingFailure reports whether failures contains any finding above
// SeverityWarning. A SeverityWarning finding is still emitted (both JSON and
// human output) but must not fail the run — the sweep's grandfather tier for
// a type-specific check that refuses outright at create/promote and
// single-file validate (e.g. lead_sentence, below).
func hasBlockingFailure(failures []*errfmt.ValidationError) bool {
	for _, f := range failures {
		if f.Severity != errfmt.SeverityWarning {
			return true
		}
	}
	return false
}

package cli

import (
	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

// leadSentenceHeading maps a type to the H2 heading its lead-sentence rule
// governs. Types outside this set (including learning, handled earlier and
// returned before this runs) carry no lead-sentence check.
var leadSentenceHeading = map[core.Type]string{
	core.TypeIssue:     "Problem",
	core.TypeMilestone: "Objective",
}

// leadSentenceFailures runs core.ValidateLeadSentence for t's governing
// heading and wraps the result as SeverityWarning findings — always at
// warning severity because the rule must never fail create, promote, or
// validate (writing-issue/writing-milestone prescribe it in prose only;
// this is the deterministic backstop). Shared by validateOne (an already
// loaded Artifact) and staticBodyFailures (the in-memory body before an
// Artifact exists).
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

package core

import (
	"fmt"
	"strings"
)

// RequiredIssueSections is the ordered set of headings validate enforces on
// issue body content. H3 entries (### Direct, ### Indirect) are sub-headings
// of ## Verification and must appear after it. Exported so create can scaffold
// the skeleton without duplicating the list.
// `## Acceptance criteria` is deliberately absent: the issue's terminal
// predicate now lives in the `goal:` frontmatter field and the test-list in
// `## Verification`, so AC is an optional prose checklist, not a required
// heading. See docs/issue-spec.md.
var RequiredIssueSections = []string{
	"## Problem",
	"## Non-goals",
	"## Verification",
	"### Direct",
	"### Indirect",
	"## Links",
}

// ScaffoldSections renders an ordered heading list into a body skeleton: each
// heading on its own line, blank line between, so the result passes the
// matching ordered-scan validator (ValidateIssue, ValidateLearning) without a
// follow-up edit. Shared by create and promote so the two scaffolds can't drift.
func ScaffoldSections(headings []string) string {
	var sb strings.Builder
	for _, h := range headings {
		sb.WriteString("\n")
		sb.WriteString(h)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ValidateIssue checks that the issue body contains the required headings in
// order and that code fences in the Verification section are balanced.
// Same ordered-scan algorithm as ValidateLearning.
func ValidateIssue(a *Artifact) []error {
	var errs []error
	pos := 0
	body := a.Body
	for _, h := range RequiredIssueSections {
		idx := strings.Index(body[pos:], "\n"+h)
		if idx < 0 && !strings.HasPrefix(body[pos:], h) {
			errs = append(errs, fmt.Errorf("issue body missing required heading %q", h))
			continue
		}
		if idx >= 0 {
			pos = pos + idx + len(h) + 1
		} else {
			pos += len(h)
		}
	}

	// Fence-balance check, scoped to the Verification section per the issue's
	// goal ("only fence balance in the Verification section"). An odd number of
	// triple-backtick fence lines means at least one fence is unclosed — the
	// canonical failure mode is a heredoc delimiter eating the closing ```.
	//
	// Accepted limitation: this is line-level parity, not the depth-aware scan
	// the verification runner performs (docs/issue-spec.md). A heredoc holding a
	// mini issue doc with a single illustrative ```bash opener is real markdown
	// text to us but a nested fence to the runner, so such a body counts odd and
	// false-rejects. Distinguishing the two requires executing the bash (the
	// runner's job); per this issue's "not full markdown linting" non-goal we
	// accept the false-positive rather than reimplement the runner at write time.
	fenceCount := 0
	for _, line := range strings.Split(verificationSpan(body), "\n") {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "```") {
			fenceCount++
		}
	}
	if fenceCount%2 != 0 {
		errs = append(errs, fmt.Errorf("issue body has unbalanced code fences (unterminated ``` block) in Verification"))
	}

	return errs
}

// verificationSpan returns the body slice from the "## Verification" heading to
// the next "## " heading (or end of body). Empty if Verification is absent —
// the missing-heading check above already reports that.
func verificationSpan(body string) string {
	start := strings.Index(body, "## Verification")
	if start < 0 {
		return ""
	}
	rest := body[start:]
	if next := strings.Index(rest[len("## Verification"):], "\n## "); next >= 0 {
		return rest[:len("## Verification")+next]
	}
	return rest
}

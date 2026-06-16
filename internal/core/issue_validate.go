package core

import (
	"fmt"
	"regexp"
	"strings"
)

// anvilInvocationRE matches an `anvil <args...>` invocation inside a code fence
// and captures the remainder of the line after `anvil`. The boundary is `\b`
// rather than line-start, so command-chained forms (`x && anvil bogus`) and
// substitutions (`$(anvil bogus)`) are caught too. The trailing capture is the
// raw argument string, tokenised and walked against the command tree below.
var anvilInvocationRE = regexp.MustCompile(`\banvil\s+([^\n]*)`)

// VerbPathValidator reports whether the anvil command described by tokens — the
// whitespace-split words following `anvil` on a fence line — names a real path
// through the command tree. It returns the offending subcommand token and false
// when a token sits in command position but matches no registered subcommand;
// "" and true otherwise. The CLI layer builds this from cobra (which owns the
// command tree); core stays cobra-free, mirroring the existing core/CLI split.
type VerbPathValidator func(tokens []string) (bad string, ok bool)

// lintVerificationVerbs scans code-fence lines inside the Verification span and
// reports any `anvil <verb> <subverb>...` invocation whose deepest subcommand
// token names no registered command. Only lines inside a code fence (between
// opening ``` and closing ```) are scanned. Returns nil without scanning when
// validate is nil (caller has no command tree to check against).
func lintVerificationVerbs(body string, validate VerbPathValidator) []error {
	if validate == nil {
		return nil
	}
	span := verificationSpan(body)
	if span == "" {
		return nil
	}

	// Collect only lines inside a code fence.
	var codeLines strings.Builder
	inFence := false
	for _, line := range strings.Split(span, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			codeLines.WriteString(line)
			codeLines.WriteByte('\n')
		}
	}

	var errs []error
	seen := make(map[string]struct{})
	for _, m := range anvilInvocationRE.FindAllStringSubmatch(codeLines.String(), -1) {
		tokens := strings.Fields(m[1])
		bad, ok := validate(tokens)
		if ok {
			continue
		}
		if _, already := seen[bad]; already {
			continue
		}
		seen[bad] = struct{}{}
		errs = append(errs, fmt.Errorf("Verification block cites unknown anvil subcommand %q — fix the command or update the issue", bad))
	}
	return errs
}

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

// ValidateIssueVerbs checks that every `anvil <verb> <subverb>...` invocation
// inside a code fence in the Verification block names a real path through the
// command tree — the deepest subcommand token must match a registered command,
// so a stale nested subcommand (`anvil project init`) is caught, not just a
// bogus top-level verb. Call this from CLI layers that own the cobra tree; pass
// the result through the same errfmt pipeline as ValidateIssue.
func ValidateIssueVerbs(body string, validate VerbPathValidator) []error {
	return lintVerificationVerbs(body, validate)
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

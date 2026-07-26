package core

import (
	"fmt"
	"regexp"
	"strings"
)

// anvilInvocationRE matches an `anvil <args...>` invocation inside a code fence
// and captures the remainder of the line after `anvil`. `anvil` must sit in
// *command position* — line start, or after whitespace or a shell separator —
// so chained forms (`x && anvil bogus`) and substitutions (`$(anvil bogus)`)
// are still caught while a path fragment (`cd ~/Development/anvil && just
// install`) is not; reading that `&&` as a subcommand produced most of the
// false positives. Horizontal whitespace only, so a line-trailing `anvil` does
// not swallow the next line as its arguments.
var anvilInvocationRE = regexp.MustCompile(`(?m)(?:^|[ \t;&|(])anvil[ \t]+([^\n]*)`)

// plausibleVerbRE is the shape a real anvil subcommand token can take. A
// reported token that fails it (`$tmp`, `<id>`, `2`, `""`) is shell debris the
// tokeniser mis-read, not a stale verb — dropping it keeps a genuine typo
// (`anvil craete issue`) reportable while silencing the noise.
var plausibleVerbRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

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
//
// introducedIn is checked before an unresolved token is reported as an error:
// a verb the issue's own goal/title names is the one it is introducing, not a
// stale one, so it passes even though validate rejects it (the command tree
// doesn't know about it yet — that's the point of the issue).
func lintVerificationVerbs(body string, validate VerbPathValidator, introducedIn string) []error {
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
			if strings.HasPrefix(trimmed, "#") {
				continue // shell comment: prose, not an invocation
			}
			codeLines.WriteString(line)
			codeLines.WriteByte('\n')
		}
	}

	var errs []error
	seen := make(map[string]struct{})
	for _, m := range anvilInvocationRE.FindAllStringSubmatch(codeLines.String(), -1) {
		args := simpleCommandArgs(m[1])
		if len(args) == 0 {
			continue
		}
		bad, ok := validate(args)
		if ok {
			continue
		}
		if !plausibleVerbRE.MatchString(bad) {
			continue
		}
		if _, already := seen[bad]; already {
			continue
		}
		// Accepted v0.1 limitation: a common-English anvil verb (link/show/set/
		// run/build/install) that appears unrelated in goal/title silences a
		// genuinely stale token here — a false-green we take over tightening the
		// match (phrase-adjacency breaks nested verbs like `anvil session gc`).
		if verbIntroduced(bad, introducedIn) {
			continue
		}
		seen[bad] = struct{}{}
		errs = append(errs, fmt.Errorf("verification block cites unknown anvil subcommand %q — fix the command or update the issue", bad))
	}
	return errs
}

// simpleCommandArgs returns the leading subcommand-path words after `anvil`:
// tokens up to the first shell operator or the first flag. Words past an
// operator belong to a *different* command (this is what flagged `grep`, `sed`,
// `just`); words past a flag are flag values (`--project burgh`). A subcommand
// path is always positional and precedes its flags, so stopping there loses
// nothing — and an empty result means `anvil` was itself a flag value.
func simpleCommandArgs(rest string) []string {
	var tokens []string
	for _, tok := range strings.Fields(rest) {
		if strings.ContainsAny(tok, "|;&<>#`") || strings.HasPrefix(tok, "-") {
			break
		}
		tokens = append(tokens, tok)
	}
	return tokens
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
//
// goal and title are the issue's own frontmatter fields — the escape hatch for
// a feature issue introducing a new subcommand: an unresolved verb named there
// is what the issue is *for*, not drift, so it's accepted rather than rejected.
func ValidateIssueVerbs(body, goal, title string, validate VerbPathValidator) []error {
	return lintVerificationVerbs(body, validate, goal+" "+title)
}

// verbIntroduced reports whether bad appears as a whole word in text (the
// issue's goal+title) — the signal that the issue is introducing that verb
// rather than citing a stale one.
func verbIntroduced(bad, text string) bool {
	if bad == "" {
		return false
	}
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(bad) + `\b`)
	return re.MatchString(text)
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

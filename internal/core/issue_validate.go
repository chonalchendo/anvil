package core

import (
	"fmt"
	"regexp"
	"strings"
)

// anvilInvocationRE matches an `anvil <args...>` invocation inside a code fence
// and captures the remainder of the line after `anvil`. `anvil` must sit in
// *command position* — line start, or after whitespace, a shell separator, or a
// path separator — so chained forms (`x && anvil bogus`), substitutions
// (`$(anvil bogus)`) and path-prefixed forms (`./bin/anvil bogus`, the mandated
// smoke-gate shape) are all caught. `/` is in the class deliberately: a path
// *fragment* like `cd ~/Development/anvil && just install` matches too, but
// simpleCommandArgs then breaks at the `&&` and yields no tokens, so the false
// positive that motivated this change stays dead without blinding the check to
// `./bin/anvil <verb>`.
//
// The capture stops at a shell separator rather than running to end of line.
// Matches are non-overlapping, so a run-to-EOL capture would let the path
// fragment in `cd ~/Development/anvil && anvil project init` swallow the genuine
// invocation behind it and report nothing. Stopping early re-anchors the scan on
// the next command. Nothing is lost: simpleCommandArgs discards post-separator
// words anyway. Horizontal whitespace only, so a line-trailing `anvil` does not
// swallow the next line as its arguments.
var anvilInvocationRE = regexp.MustCompile(`(?m)(?:^|[ \t;&|(/])anvil[ \t]+([^\n;&|]*)`)

// plausibleVerbRE is the shape a real anvil subcommand token can take. A
// reported token that fails it (`$tmp`, `<id>`, `2`, `""`) is shell debris the
// tokeniser mis-read, not a stale verb — dropping it keeps a genuine typo
// (`anvil craete issue`) reportable while silencing the noise.
var plausibleVerbRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// VerbPathValidator reports whether the anvil command described by tokens — the
// leading positional run following `anvil` on a fence line, flags and anything
// past a shell operator already stripped by simpleCommandArgs — names a real
// path through the command tree. It returns the offending subcommand token and false
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
	codeLines := verificationCodeLines(body)
	if codeLines == "" {
		return nil
	}

	var errs []error
	seen := make(map[string]struct{})
	for _, m := range anvilInvocationRE.FindAllStringSubmatch(codeLines, -1) {
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

// verificationCodeLines returns the lines inside a code fence within the
// Verification span, one per line, comments dropped. Shared by every lint that
// only cares about the shell commands a Verification block actually runs, not
// surrounding prose.
func verificationCodeLines(body string) string {
	span := verificationSpan(body)
	if span == "" {
		return ""
	}
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
				continue // shell comment: prose, not a command
			}
			codeLines.WriteString(line)
			codeLines.WriteByte('\n')
		}
	}
	return codeLines.String()
}

// checkoutPathRE matches a hardcoded developer checkout root
// (~/Development/<repo>, $HOME/Development/<repo>, or /Users|/home/<user>/
// Development/<repo>) anywhere on a line, instead of deriving the tree from
// where the predicate runs ($(git rev-parse --show-toplevel), per
// docs/issue-spec.md's Parsing rules). The offence is the root, not the verb:
// `cd`, `git -C`, `ls`, or a variable assignment (`BIN=/Users/…/bin/anvil`)
// all pin the predicate to the main checkout, so any worktree anchoring the
// runner performs is overridden and the predicate silently verifies the wrong
// tree when dispatched to a fleet worktree (the runner's own anchoring was
// fixed by resolved sibling issue anvil.completing-issue-verification-runner-
// tests-the-main-checkout).
//
// The literal /Development/ segment is deliberate, not an oversight: a
// checkout root under another layout (/Users/<u>/code/<repo>) is textually
// indistinguishable from the legitimate absolute paths predicates must be
// allowed to reference (the external vault, /Users/<u>/anvil-vault). Matching
// any home-anchored absolute path would flag those, so the lint trades
// coverage of unconventional layouts for zero false positives on vault paths.
var checkoutPathRE = regexp.MustCompile(`(?:~|\$HOME|/Users/[^/\s"']+|/home/[^/\s"']+)/Development/[A-Za-z0-9_.-]+`)

// CheckoutPathMatches returns every hardcoded checkout-path reference found
// in text, one per matching line, in encounter order. Shared by
// ValidateIssueCheckoutPaths (fenced Verification lines only, at authoring
// time) and `anvil validate --verification-stdin` (raw predicate text piped
// in directly, no markdown wrapper required).
func CheckoutPathMatches(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if m := checkoutPathRE.FindString(line); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// nonGatingNegationRE matches a `!` in command position — every position where
// bash exempts the negated command from `set -e`, so a failure there cannot
// abort the block. Verified against bash 5.3: `! c`, `x; ! c`, `a && ! c`,
// `a || ! c`, `for …; do ! c; done`, `if a; then ! c; fi` and `{ ! c; }` all
// survive a failing `c`. `(` is deliberately absent — a subshell `( ! c )`
// does abort, so flagging it would refuse a predicate that gates.
var nonGatingNegationRE = regexp.MustCompile(`(^|[;&|{]|(^|[[:space:]])(do|then|else))[[:space:]]*!([[:space:]]|$)`)

// NonGatingNegation returns the first line of a verification block that carries
// a `!` in command position and is not the block's last executable line, or "".
// Such a line cannot fail the block: `set -e` exempts it, and only the last
// line's status becomes the block's verdict. The detector is textual, so a
// quoted or heredoc `; ! ` trips it too — refusing loudly beats shipping an
// assertion that silently never gates (mentat.0291).
//
// completing-issue's run-verification.sh carries the same rule in awk so both
// executors judge a block identically; internal/installer's lockstep test
// drives one case corpus through this function and the shipped script.
func NonGatingNegation(block string) string {
	var lines []string
	for _, l := range strings.Split(block, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		lines = append(lines, t)
	}
	for i := 0; i+1 < len(lines); i++ {
		if nonGatingNegationRE.MatchString(lines[i]) {
			return lines[i]
		}
	}
	return ""
}

// ValidateIssueCheckoutPaths reports one error per hardcoded checkout-path
// reference found in the issue's Verification blocks — the authoring-time half
// of this fix; the runner's own anchoring was already resolved by the sibling
// issue named in checkoutPathRE's doc comment.
func ValidateIssueCheckoutPaths(body string) []error {
	var errs []error
	for _, m := range CheckoutPathMatches(verificationCodeLines(body)) {
		errs = append(errs, fmt.Errorf("verification block hardcodes a checkout path (%q) — anchor with $(git rev-parse --show-toplevel) instead so a dispatched worktree verifies itself, not the main checkout", m))
	}
	return errs
}

// lakehouseSchemaRE matches an unparameterised `lakehouse.<schema>.` table
// reference: `lakehouse.` followed directly by a bare identifier and a dot.
// The established fix is `${SCHEMA:-modelled}` parameterisation (mentat.0291
// precedent, now on eight issues) — `${` after `lakehouse.` starts with `$`,
// not an identifier character, so a parameterised reference never matches.
var lakehouseSchemaRE = regexp.MustCompile(`\blakehouse\.[A-Za-z_][A-Za-z0-9_]*\.`)

// LakehouseSchemaMatches returns every hardcoded lakehouse.<schema>. reference
// found in text, one per matching line, in encounter order. A prod-hardcoded
// schema (mentat.0250/0252/0253's `lakehouse.prod.`) is legitimately red at
// authoring time and only bites at completion — after dispatch, review and
// responder rounds have already run — because the worker is forbidden the prod
// apply the predicate demands.
func LakehouseSchemaMatches(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if m := lakehouseSchemaRE.FindString(line); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// ValidateIssueLakehouseSchema reports one error per hardcoded lakehouse
// schema reference found in the issue's Indirect Verification block(s). Scoped
// to Indirect only: a hardcoded schema in Direct (an in-tree unit/integration
// test) does not carry the same forbidden-prod-apply failure mode this rule
// targets. An unterminated fence is already reported by ValidateIssue's
// fence-balance check, so this returns no errors for that case rather than
// duplicating it.
func ValidateIssueLakehouseSchema(body string) []error {
	blocks, err := VerificationBlocks(body, "Indirect")
	if err != nil {
		return nil
	}
	var errs []error
	for _, block := range blocks {
		for _, m := range LakehouseSchemaMatches(block) {
			errs = append(errs, fmt.Errorf("Indirect verification block hardcodes a lakehouse schema (%q) — parameterise with ${SCHEMA:-modelled} so a worker without prod-apply rights can satisfy it", m))
		}
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

// bashFenceOpenRE matches an exact ```bash fence opener (optional trailing
// whitespace only) — the same shape run-verification.sh requires.
var bashFenceOpenRE = regexp.MustCompile(`^` + "```" + `bash[ \t]*$`)

// VerificationBlocks returns the ```bash fenced blocks under the "### <label>"
// heading of the body's "## Verification" section (label: "Direct" or
// "Indirect"), preserving each block's raw lines.
//
// It walks the whole body in one pass rather than slicing with
// verificationSpan, because that helper's `strings.Index(rest, "\n## ")` scan
// is fence-blind: a heredoc payload carrying its own "## " line (a mini issue
// doc nested inside a check) truncates the span mid-block, and the blocks past
// the cut are then silently dropped — a gate that skips predicates is the
// defect class the create-time feasibility gate exists to end (anvil.0196).
// Here every structural marker — the opening "## Verification", the "### "
// subsection headings, the closing "## " — is only honoured at fence depth 0,
// so nested markers are read as the block content they are.
//
// A subsection with no fenced block returns nil — absence is not this
// function's concern, only extraction of what's present. An unterminated fence
// leaves the extraction unknowable, so it returns an error rather than a
// truncated block list: the caller must fail closed.
func VerificationBlocks(body, label string) ([]string, error) {
	headingRE := regexp.MustCompile(`^### ` + regexp.QuoteMeta(label) + `([^A-Za-z]|$)`)

	var blocks []string
	var buf strings.Builder
	started, inSection, inBlock := false, false, false
	depth := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "```") {
			switch {
			case len(line) > 3 && isAlpha(line[3]):
				if inSection && depth == 0 && bashFenceOpenRE.MatchString(line) {
					inBlock = true
					depth = 1
					buf.Reset()
					continue
				}
				depth++
			case inBlock && depth == 1:
				blocks = append(blocks, buf.String())
				inBlock = false
				depth = 0
				continue
			default:
				if depth > 0 {
					depth--
				}
			}
			if inBlock {
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
			continue
		}
		if depth == 0 {
			switch {
			case !started:
				if strings.HasPrefix(line, "## Verification") {
					started = true
				}
				continue
			case strings.HasPrefix(line, "## "):
				return blocks, nil
			case headingRE.MatchString(line):
				inSection = true
				continue
			case strings.HasPrefix(line, "### "):
				inSection = false
			}
		}
		if inBlock {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	if inBlock {
		return nil, fmt.Errorf("issue body has an unterminated ``` fence inside Verification %s; the remaining blocks cannot be extracted", label)
	}
	return blocks, nil
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// verificationSpan returns the body slice from the "## Verification" heading to
// the next "## " heading (or end of body). Empty if Verification is absent —
// the missing-heading check above already reports that.
//
// Fence depth is tracked so a "## " line inside a still-open fence — a heredoc
// payload carrying a mini issue doc — is read as the block content it is,
// rather than cutting the section short. A truncated span made the fence-
// balance check below refuse valid bodies and made the verb lint read less
// than VerificationBlocks executes; the two must agree on where the section
// ends.
func verificationSpan(body string) string {
	var span strings.Builder
	started := false
	depth := 0
	for _, line := range strings.Split(body, "\n") {
		switch {
		case strings.HasPrefix(line, "```"):
			if len(line) > 3 && isAlpha(line[3]) {
				depth++
			} else if depth > 0 {
				depth--
			}
		case depth > 0:
		case !started:
			started = strings.HasPrefix(line, "## Verification")
		case strings.HasPrefix(line, "## "):
			return span.String()
		}
		if started {
			span.WriteString(line)
			span.WriteByte('\n')
		}
	}
	return span.String()
}

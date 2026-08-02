package core

import (
	"strings"
	"testing"
)

// fullIssueBody is a body containing all required issue sections in order.
const fullIssueBody = "\n## Problem\np\n\n## Acceptance criteria\nac\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\nd\n\n### Indirect\ni\n\n## Links\n"

func TestValidateIssue_MissingSection(t *testing.T) {
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\n\n## Acceptance criteria\n\n## Non-goals\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for missing ## Verification and ## Links")
	}
}

func TestValidateIssue_MissingVerification(t *testing.T) {
	// Has all pre-existing sections but no ## Verification block.
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\np\n\n## Acceptance criteria\nac\n\n## Non-goals\nng\n\n## Links\nlinks\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for missing ## Verification")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "## Verification") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ## Verification named in errors, got: %v", errs)
	}
}

func TestValidateIssue_MissingDirect(t *testing.T) {
	// ## Verification present but ### Direct missing (### Indirect comes first).
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\np\n\n## Acceptance criteria\nac\n\n## Non-goals\nng\n\n## Verification\n\n### Indirect\ni\n\n## Links\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for missing ### Direct")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "### Direct") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ### Direct named in errors, got: %v", errs)
	}
}

func TestValidateIssue_AllSectionsPresent(t *testing.T) {
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        fullIssueBody,
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateIssue_OutOfOrder(t *testing.T) {
	// sections present but order wrong — validator enforces ordered scan
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Links\n\n## Problem\n\n## Acceptance criteria\n\n## Non-goals\n\n## Verification\n\n### Direct\n\n### Indirect\n",
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for out-of-order sections")
	}
}

func TestValidateIssue_NoLeadingNewline_AllSectionsPresent(t *testing.T) {
	// body with no leading newline triggers the HasPrefix branch on the first
	// heading; subsequent headings also butt up against each other, exercising
	// the pos-advance path.
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "## Problem## Non-goals## Verification### Direct### Indirect## Links\n",
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("all headings present — expected no errors, got: %v", errs)
	}
}

func TestValidateIssue_NoAcceptanceCriteria_Valid(t *testing.T) {
	// `## Acceptance criteria` is demoted to an optional prose checklist: a
	// body omitting it must still validate.
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\nd\n\n### Indirect\ni\n\n## Links\n",
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("AC is optional — expected no errors, got: %v", errs)
	}
}

func TestValidateIssue_UnterminatedFence_Rejected(t *testing.T) {
	// Body with an unterminated code fence in Verification must be rejected.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntrue\n\n## Links\n"
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        body,
	}
	errs := ValidateIssue(a)
	if len(errs) == 0 {
		t.Fatal("expected error for unterminated code fence")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "unbalanced") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'unbalanced' in error, got: %v", errs)
	}
}

func TestValidateIssue_BalancedFences_Valid(t *testing.T) {
	// Body with balanced fences must pass.
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        fullIssueBody,
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("balanced fences — expected no errors, got: %v", errs)
	}
}

func TestValidateIssue_BalancedFencesInVerification_Valid(t *testing.T) {
	// Body with fenced bash blocks in both Direct and Indirect must pass.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        body,
	}
	if errs := ValidateIssue(a); len(errs) != 0 {
		t.Errorf("balanced fenced blocks — expected no errors, got: %v", errs)
	}
}

func TestValidateIssue_UnbalancedFenceOutsideVerification_Ignored(t *testing.T) {
	// The check is scoped to the Verification section per the issue goal: an odd
	// fence outside Verification (here in ## Problem) must NOT be flagged.
	body := "\n## Problem\n```bash\noops unterminated\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        body,
	}
	for _, e := range ValidateIssue(a) {
		if strings.Contains(e.Error(), "unbalanced") {
			t.Errorf("fence outside Verification must be ignored, got: %v", e)
		}
	}
}

// fixtureVerbValidator mimics the cobra-backed VerbPathValidator the CLI builds
// at runtime, but without importing cobra (core stays cobra-free). It models a
// tiny command tree: a path is a non-leaf (has subcommands) until a leaf token
// is reached, after which trailing tokens are args/flags. Mirrors the
// Find-based rule in cli.verbPathValidator — a token in subcommand position
// that names no child is the bogus one.
//
// tree maps a command path (joined by space) to its set of child names; a path
// absent from tree is a leaf (consumes the rest as args).
func fixtureVerbValidator(tree map[string]map[string]struct{}) VerbPathValidator {
	return func(tokens []string) (string, bool) {
		path := ""
		for _, tok := range tokens {
			children, hasSub := tree[path]
			if !hasSub {
				return "", true // reached a leaf; rest are args/flags
			}
			if strings.HasPrefix(tok, "-") {
				return "", true // flag before any deeper subcommand
			}
			if _, ok := children[tok]; !ok {
				return strings.Trim(tok, "()\"';|&"), false
			}
			if path == "" {
				path = tok
			} else {
				path += " " + tok
			}
		}
		return "", true
	}
}

// fixtureTree models `anvil create issue`, `anvil list`, `anvil show`,
// `anvil validate`, `anvil project adopt`, `anvil transition`. `project` is a
// non-leaf whose only child is `adopt` (so `project init` is bogus); `create`
// is a non-leaf whose child is `issue`; the rest are leaves.
var fixtureTree = map[string]map[string]struct{}{
	"": {
		"create": {}, "list": {}, "show": {}, "validate": {},
		"project": {}, "transition": {},
	},
	"create":  {"issue": {}},
	"project": {"adopt": {}},
}

func validatorFixture() VerbPathValidator { return fixtureVerbValidator(fixtureTree) }

func TestValidateIssueVerbs_UnknownVerb_Rejected(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil frobnicate widget\n```\n\n### Indirect\n```bash\nanvil frobnicate widget\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) == 0 {
		t.Fatal("expected error for unknown anvil verb 'frobnicate'")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "frobnicate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'frobnicate' named in error, got: %v", errs)
	}
}

func TestValidateIssueVerbs_NestedUnknownSubcommand_Rejected(t *testing.T) {
	// The issue's motivating reproduction: `project` is a real verb but `init` is
	// not a registered subcommand. The deepest token must be validated, not just
	// the top-level verb.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil project init scratch\n```\n\n### Indirect\n```bash\nanvil project init scratch\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) == 0 {
		t.Fatal("expected error for nested unknown subcommand 'anvil project init'")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "init") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'init' named in error, got: %v", errs)
	}
}

func TestValidateIssueVerbs_NestedKnownSubcommand_Accepted(t *testing.T) {
	// `anvil project adopt` is a real nested path; its trailing positional arg
	// (`scratch`) must not be mistaken for a bogus subcommand.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil project adopt scratch\n```\n\n### Indirect\n```bash\nanvil create issue --title t\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) != 0 {
		t.Errorf("known nested path must be accepted, got: %v", errs)
	}
}

func TestValidateIssueVerbs_UnknownVerb_DeduplicatedAcrossFences(t *testing.T) {
	// The same bogus verb in both Direct and Indirect should only be reported once.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil bogus\n```\n\n### Indirect\n```bash\nanvil bogus\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) != 1 {
		t.Errorf("expected exactly 1 error for duplicate unknown verb, got %d: %v", len(errs), errs)
	}
}

func TestValidateIssueVerbs_KnownVerb_Accepted(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil create issue --title t\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) != 0 {
		t.Errorf("known verbs must be accepted, got: %v", errs)
	}
}

func TestValidateIssueVerbs_ChainedInvocation_Rejected(t *testing.T) {
	// A non-line-start invocation (`x && anvil bogus`) must still be caught.
	// Verification fences routinely chain setup onto the call under test
	// (`cd $tmp && anvil ...`) or capture it (`echo $(anvil ...)`), so anchoring
	// on line start alone would blind the lint to most real invocations. The
	// anchor is an explicit separator class instead: the char before `anvil` must
	// be a shell separator, whitespace, or a path separator — never a word char,
	// which is what keeps `anvil` inside a longer word or path segment out.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue && anvil bogus\n```\n\n### Indirect\n```bash\necho $(anvil bogus)\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) == 0 {
		t.Fatal("expected error for chained/substituted unknown verb")
	}
	for _, e := range errs {
		if strings.Contains(e.Error(), "bogus)") {
			t.Errorf("shell punctuation must be trimmed from the reported token, got: %v", e)
		}
	}
}

func TestValidateIssueVerbs_NonAnvilShellLines_Ignored(t *testing.T) {
	// The anvil.0168 reproduction set: shell content around a legitimate anvil
	// call. `anvil` inside a path is not an invocation; words after a pipe or
	// separator belong to another command; a shell comment is prose.
	lines := []string{
		"cd ~/Development/anvil && just install",
		"cd ~/Development/anvil && go test ./internal/...",
		"anvil list issue --json | grep -c ready",
		"anvil validate 2>&1 | sed -n '1p'",
		"tmp=$(mktemp -d); anvil validate --vault $tmp",
		"# anvil frobnicate is what this issue introduces",
		"ls ~/anvil-worktrees/",
		// `anvil` as a flag value, and as the last word on a line: neither may
		// consume the following token / line as its arguments.
		"go run ./cmd/anvil create issue --project anvil --title t --tags domain/x",
		"anvil create issue --project anvil",
		"test -f \"$d/70-issues/anvil.t.md\"",
	}
	fence := strings.Join(lines, "\n")
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\n" + fence + "\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) != 0 {
		t.Errorf("non-anvil shell lines must raise no unknown-subcommand findings, got: %v", errs)
	}
}

func TestValidateIssueVerbs_GenuineTypo_StillRejectedAmongShellNoise(t *testing.T) {
	// The 0077 guarantee must survive the narrowing: a real typo'd subcommand in
	// a line that also carries a pipe is still reported. The second line pins the
	// path-prefixed form: `./bin/anvil <verb>` is the mandated smoke-gate shape
	// (docs/worktree-workflow.md), so dropping `/` from the command-position class
	// would silently unlint every smoke fence in the vault.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ncd ~/Development/anvil && anvil project init scratch | tee log\n./bin/anvil frobnicate widget\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) != 2 {
		t.Fatalf("expected exactly two errors ('init' and 'frobnicate'), got %d: %v", len(errs), errs)
	}
	joined := errs[0].Error() + " " + errs[1].Error()
	for _, want := range []string{"init", "frobnicate"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected an error naming %q, got: %v", want, errs)
		}
	}
}

func TestValidateIssueVerbs_AnvilOutsideFence_Ignored(t *testing.T) {
	// `anvil bogus` mentioned in prose (outside a code fence) must not be flagged.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\nRun anvil bogus to test.\n\n### Direct\n```bash\nanvil create issue\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) != 0 {
		t.Errorf("anvil verb outside fence must be ignored, got: %v", errs)
	}
}

func TestValidateIssueVerbs_AnvilOutsideVerificationSpan_Ignored(t *testing.T) {
	// `anvil bogus` in ## Problem (outside the Verification span) must not fire.
	body := "\n## Problem\n```bash\nanvil bogus\n```\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil create issue\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", validatorFixture())
	if len(errs) != 0 {
		t.Errorf("anvil verb outside Verification span must be ignored, got: %v", errs)
	}
}

func TestValidateIssueVerbs_NilValidator_SkipsCheck(t *testing.T) {
	// Passing nil skips the verb-lint so callers without a command tree can safely
	// call ValidateIssueVerbs without panicking.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil totally-fake-verb\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, "", "", nil)
	if len(errs) != 0 {
		t.Errorf("nil validator must skip check, got: %v", errs)
	}
}

func TestValidateIssueVerbs_IntroducedVerb_Accepted(t *testing.T) {
	// The escape hatch: a feature issue citing the subcommand it is introducing
	// (named in goal/title) must not be rejected, even though the command tree
	// doesn't know about it yet.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil hydrate issue\n```\n\n### Indirect\n```bash\nanvil hydrate issue\n```\n\n## Links\n"
	goal := "anvil hydrate <issue> assembles the linked-context closure"
	title := "anvil hydrate issue — assemble the linked context"
	errs := ValidateIssueVerbs(body, goal, title, validatorFixture())
	if len(errs) != 0 {
		t.Errorf("verb named in goal/title must be accepted, got: %v", errs)
	}
}

func TestValidateIssueVerbs_StaleVerb_StillRejectedDespiteGoalTitle(t *testing.T) {
	// The escape hatch must not swallow a genuinely stale/unrelated verb: one
	// absent from goal/title still fails even though the issue has unrelated
	// goal/title text naming a different (introduced) verb.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil project init scratch\n```\n\n### Indirect\n```bash\nanvil project init scratch\n```\n\n## Links\n"
	goal := "anvil hydrate <issue> assembles the linked-context closure"
	title := "anvil hydrate issue — assemble the linked context"
	errs := ValidateIssueVerbs(body, goal, title, validatorFixture())
	if len(errs) == 0 {
		t.Fatal("expected stale nested subcommand 'anvil project init' to still be rejected")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "init") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'init' named in error, got: %v", errs)
	}
}

func TestValidateIssueVerbs_StaleVerb_SuppressedWhenTokenInGoalTitle(t *testing.T) {
	// Pins the accepted v0.1 limitation: when a stale token's whole word also
	// appears (unrelated) in goal/title, the escape hatch suppresses the error —
	// a false-green we take rather than tightening the whole-word match.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil project init scratch\n```\n\n### Indirect\n```bash\nanvil project init scratch\n```\n\n## Links\n"
	goal := "reindex must run init before the first list call"
	title := "anvil init drift — unrelated mention of the stale token"
	errs := ValidateIssueVerbs(body, goal, title, validatorFixture())
	if len(errs) != 0 {
		t.Errorf("accepted limitation: stale 'init' is suppressed when its token appears in goal/title, got: %v", errs)
	}
}

func TestValidateIssue_NestedHeredocFence_Accepted(t *testing.T) {
	// This body was previously false-rejected as "unbalanced code fences": the
	// write-time check was line-level parity over a fence-blind span, so a
	// heredoc holding a mini issue doc (its own "## Verification" line plus an
	// illustrative ```bash opener) both truncated the span and made the fence
	// count odd. verificationSpan now tracks fence depth, which the create-time
	// feasibility gate depends on — the verb lint and VerificationBlocks must
	// agree with each other on where the section ends (anvil.0196).
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ncat <<'EOF' > /tmp/mini.md\n## Verification\n```bash\ntrue\n```\nEOF\nanvil create issue --body-file /tmp/mini.md\n```\n\n## Links\n"
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        body,
	}
	for _, e := range ValidateIssue(a) {
		if strings.Contains(e.Error(), "unbalanced") {
			t.Errorf("valid nested-heredoc body should not be rejected: %v", e)
		}
	}
}

func mustVerificationBlocks(t *testing.T, body, label string) []string {
	t.Helper()
	blocks, err := VerificationBlocks(body, label)
	if err != nil {
		t.Fatalf("VerificationBlocks(%s) error = %v", label, err)
	}
	return blocks
}

func TestVerificationBlocks_ExtractsDirectAndIndirect(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\nexit 3\n```\n\n## Links\n"
	direct := mustVerificationBlocks(t, body, "Direct")
	indirect := mustVerificationBlocks(t, body, "Indirect")
	if len(direct) != 1 || direct[0] != "true\n" {
		t.Fatalf("direct = %q, want [%q]", direct, "true\n")
	}
	if len(indirect) != 1 || indirect[0] != "exit 3\n" {
		t.Fatalf("indirect = %q, want [%q]", indirect, "exit 3\n")
	}
}

func TestVerificationBlocks_NoFencedBlock_ReturnsNil(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\njust test\n\n### Indirect\nsmoke\n\n## Links\n"
	if got := mustVerificationBlocks(t, body, "Direct"); got != nil {
		t.Errorf("Direct = %v, want nil", got)
	}
	if got := mustVerificationBlocks(t, body, "Indirect"); got != nil {
		t.Errorf("Indirect = %v, want nil", got)
	}
}

func TestVerificationBlocks_NestedFenceInBlockDoesNotEndCaptureEarly(t *testing.T) {
	// A nested ```bash fence inside the Indirect block must not truncate the
	// outer block's capture.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ncat <<'EOF2' > /tmp/mini.md\n```bash\ntrue\n```\nEOF2\necho done\n```\n\n## Links\n"
	indirect := mustVerificationBlocks(t, body, "Indirect")
	if len(indirect) != 1 {
		t.Fatalf("indirect = %v, want 1 block", indirect)
	}
	if !strings.Contains(indirect[0], "echo done") {
		t.Errorf("indirect[0] = %q, want it to include the tail of the block", indirect[0])
	}
}

// TestVerificationBlocks_HeredocH2DoesNotTruncateSpan is the regression for the
// gate's own false-green: verificationSpan's fence-blind `\n## ` scan cut the
// Verification section short at a "## " line *inside* a heredoc, so every block
// past the cut was silently never executed while the create still succeeded.
func TestVerificationBlocks_HeredocH2DoesNotTruncateSpan(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ncat <<'EOF' > /tmp/mini.md\n## Links to the mini doc\n- none\nEOF\nexit 6\n```\n\n## Links\n- none\n"
	indirect := mustVerificationBlocks(t, body, "Indirect")
	if len(indirect) != 1 {
		t.Fatalf("indirect = %v, want 1 block", indirect)
	}
	if !strings.Contains(indirect[0], "exit 6") {
		t.Errorf("indirect[0] = %q, want the post-heredoc `exit 6` line to survive extraction", indirect[0])
	}
}

// TestVerificationBlocks_UnterminatedFenceFailsClosed: when the extraction is
// unknowable the gate must refuse, not run a truncated block list.
func TestVerificationBlocks_UnterminatedFenceFailsClosed(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\nexit 3\n"
	blocks, err := VerificationBlocks(body, "Indirect")
	if err == nil {
		t.Fatalf("err = nil, want an unterminated-fence error; blocks = %q", blocks)
	}
	if blocks != nil {
		t.Errorf("blocks = %q, want nil on failure", blocks)
	}
}

// TestVerificationBlocks_EarlierFencedH2IsNotTheSectionStart guards the other
// direction: a "## Verification" line inside an earlier fenced block (a mini
// issue doc quoted in Problem) is block content, not the real section start.
func TestVerificationBlocks_EarlierFencedH2IsNotTheSectionStart(t *testing.T) {
	body := "\n## Problem\n```bash\ncat <<'EOF' > /tmp/mini.md\n## Verification\n### Indirect\nEOF\n```\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\nexit 3\n```\n\n## Links\n"
	indirect := mustVerificationBlocks(t, body, "Indirect")
	if len(indirect) != 1 || indirect[0] != "exit 3\n" {
		t.Fatalf("indirect = %q, want [%q]", indirect, "exit 3\n")
	}
}

func TestValidateIssueCheckoutPaths_HardcodedCdRejected(t *testing.T) {
	cases := []string{
		"cd $HOME/Development/anvil",
		"cd ~/Development/burgh",
		"cd /Users/conal/Development/anvil",
	}
	for _, cdLine := range cases {
		body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\n" + cdLine + "\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
		errs := ValidateIssueCheckoutPaths(body)
		if len(errs) == 0 {
			t.Errorf("cdLine %q: expected a checkout-path error, got none", cdLine)
			continue
		}
		if !strings.Contains(errs[0].Error(), "checkout path") {
			t.Errorf("cdLine %q: error %q does not mention 'checkout path'", cdLine, errs[0].Error())
		}
	}
}

func TestValidateIssueCheckoutPaths_ToplevelDerivedAccepted(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ncd $(git rev-parse --show-toplevel) && just test\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	errs := ValidateIssueCheckoutPaths(body)
	if len(errs) != 0 {
		t.Errorf("git-toplevel-derived cd must be accepted, got: %v", errs)
	}
}

func TestCheckoutPathMatches_NoHeadingRequired(t *testing.T) {
	// anvil validate --verification-stdin feeds raw predicate text with no
	// "## Verification" wrapper — the matcher must not require one.
	if m := CheckoutPathMatches("true"); len(m) != 0 {
		t.Errorf("CheckoutPathMatches(%q) = %v, want none", "true", m)
	}
	if m := CheckoutPathMatches("cd /Users/conal/Development/anvil"); len(m) != 1 {
		t.Errorf("CheckoutPathMatches with hardcoded cd = %v, want 1 match", m)
	}
}

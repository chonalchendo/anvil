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
	errs := ValidateIssueVerbs(body, validatorFixture())
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
	errs := ValidateIssueVerbs(body, validatorFixture())
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
	errs := ValidateIssueVerbs(body, validatorFixture())
	if len(errs) != 0 {
		t.Errorf("known nested path must be accepted, got: %v", errs)
	}
}

func TestValidateIssueVerbs_UnknownVerb_DeduplicatedAcrossFences(t *testing.T) {
	// The same bogus verb in both Direct and Indirect should only be reported once.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil bogus\n```\n\n### Indirect\n```bash\nanvil bogus\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, validatorFixture())
	if len(errs) != 1 {
		t.Errorf("expected exactly 1 error for duplicate unknown verb, got %d: %v", len(errs), errs)
	}
}

func TestValidateIssueVerbs_KnownVerb_Accepted(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil create issue --title t\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, validatorFixture())
	if len(errs) != 0 {
		t.Errorf("known verbs must be accepted, got: %v", errs)
	}
}

func TestValidateIssueVerbs_ChainedInvocation_Rejected(t *testing.T) {
	// A non-line-start invocation (`x && anvil bogus`) must still be caught: the
	// regex anchors on a word boundary, not line start.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue && anvil bogus\n```\n\n### Indirect\n```bash\necho $(anvil bogus)\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, validatorFixture())
	if len(errs) == 0 {
		t.Fatal("expected error for chained/substituted unknown verb")
	}
	for _, e := range errs {
		if strings.Contains(e.Error(), "bogus)") {
			t.Errorf("shell punctuation must be trimmed from the reported token, got: %v", e)
		}
	}
}

func TestValidateIssueVerbs_AnvilOutsideFence_Ignored(t *testing.T) {
	// `anvil bogus` mentioned in prose (outside a code fence) must not be flagged.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\nRun anvil bogus to test.\n\n### Direct\n```bash\nanvil create issue\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, validatorFixture())
	if len(errs) != 0 {
		t.Errorf("anvil verb outside fence must be ignored, got: %v", errs)
	}
}

func TestValidateIssueVerbs_AnvilOutsideVerificationSpan_Ignored(t *testing.T) {
	// `anvil bogus` in ## Problem (outside the Verification span) must not fire.
	body := "\n## Problem\n```bash\nanvil bogus\n```\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil create issue\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, validatorFixture())
	if len(errs) != 0 {
		t.Errorf("anvil verb outside Verification span must be ignored, got: %v", errs)
	}
}

func TestValidateIssueVerbs_NilValidator_SkipsCheck(t *testing.T) {
	// Passing nil skips the verb-lint so callers without a command tree can safely
	// call ValidateIssueVerbs without panicking.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil totally-fake-verb\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, nil)
	if len(errs) != 0 {
		t.Errorf("nil validator must skip check, got: %v", errs)
	}
}

func TestValidateIssue_NestedHeredocFence_AcceptedFalsePositive(t *testing.T) {
	// ACCEPTED LIMITATION (docs/issue-spec.md depth-aware runner contract): the
	// write-time check is line-level parity, not depth-aware. A heredoc holding a
	// mini issue doc with one illustrative ```bash opener makes the fence count
	// odd, so this VALID body is false-rejected. Distinguishing it from a real
	// unterminated fence needs executing the bash (the runner's job); per the
	// issue's "not full markdown linting" non-goal we pin the false-positive
	// rather than reimplement the runner. If this ever stops rejecting, the
	// scoping/algorithm changed — revisit the contract, don't just flip the test.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ncat <<'EOF' > /tmp/mini.md\n## Verification\n```bash\ntrue\n```\nEOF\nanvil create issue --body-file /tmp/mini.md\n```\n\n## Links\n"
	a := &Artifact{
		FrontMatter: map[string]any{"type": "issue"},
		Body:        body,
	}
	found := false
	for _, e := range ValidateIssue(a) {
		if strings.Contains(e.Error(), "unbalanced") {
			found = true
		}
	}
	if !found {
		t.Skip("nested-heredoc false-positive no longer reproduces — depth-awareness may have landed; re-evaluate the accepted limitation")
	}
}

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

// knownVerbs is a minimal set that mirrors real CLI top-level subcommands for
// verb-lint tests. Kept small; the real set is discovered from cobra at runtime.
var knownVerbs = map[string]struct{}{
	"create":     {},
	"list":       {},
	"show":       {},
	"validate":   {},
	"project":    {},
	"transition": {},
}

func TestValidateIssueVerbs_UnknownVerb_Rejected(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil frobnicate widget\n```\n\n### Indirect\n```bash\nanvil frobnicate widget\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, knownVerbs)
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

func TestValidateIssueVerbs_UnknownVerb_DeduplicatedAcrossFences(t *testing.T) {
	// The same bogus verb in both Direct and Indirect should only be reported once.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil bogus\n```\n\n### Indirect\n```bash\nanvil bogus\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, knownVerbs)
	if len(errs) != 1 {
		t.Errorf("expected exactly 1 error for duplicate unknown verb, got %d: %v", len(errs), errs)
	}
}

func TestValidateIssueVerbs_KnownVerb_Accepted(t *testing.T) {
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil create issue --title t\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, knownVerbs)
	if len(errs) != 0 {
		t.Errorf("known verbs must be accepted, got: %v", errs)
	}
}

func TestValidateIssueVerbs_AnvilOutsideFence_Ignored(t *testing.T) {
	// `anvil bogus` mentioned in prose (outside a code fence) must not be flagged.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\nRun anvil bogus to test.\n\n### Direct\n```bash\nanvil create issue\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, knownVerbs)
	if len(errs) != 0 {
		t.Errorf("anvil verb outside fence must be ignored, got: %v", errs)
	}
}

func TestValidateIssueVerbs_AnvilOutsideVerificationSpan_Ignored(t *testing.T) {
	// `anvil bogus` in ## Problem (outside the Verification span) must not fire.
	body := "\n## Problem\n```bash\nanvil bogus\n```\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil create issue\n```\n\n### Indirect\n```bash\nanvil list issue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, knownVerbs)
	if len(errs) != 0 {
		t.Errorf("anvil verb outside Verification span must be ignored, got: %v", errs)
	}
}

func TestValidateIssueVerbs_NilKnownVerbs_SkipsCheck(t *testing.T) {
	// Passing nil skips the verb-lint so callers without a command tree can safely
	// call ValidateIssueVerbs without panicking.
	body := "\n## Problem\np\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\nanvil totally-fake-verb\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n"
	errs := ValidateIssueVerbs(body, nil)
	if len(errs) != 0 {
		t.Errorf("nil knownVerbs must skip check, got: %v", errs)
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

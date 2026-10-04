package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_SlugFlag_OverridesTitleDerivation(t *testing.T) {
	setupVault(t)
	path := createIssueGetPath(t,
		"create", "issue",
		"--project", "demo",
		"--title", "Investigate the very long auto-derived slug that would be cut",
		"--description", "x",
		"--goal", "investigation is done",
		"--slug", "custom-slug",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
	// Numbered format with --slug: issue.demo.NNNN.custom-slug.md
	base := filepath.Base(path)
	if matched, _ := filepath.Match("issue.demo.[0-9][0-9][0-9][0-9].custom-slug.md", base); !matched {
		t.Errorf("unexpected filename %q: want issue.demo.NNNN.custom-slug.md", base)
	}
}

func TestCreate_SlugFlag_RejectsInvalidSlug(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "issue",
		"--project", "demo",
		"--title", "X",
		"--description", "x",
		"--goal", "X is done",
		"--slug", "Bad Slug!",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected validation error for slug %q\n%s", "Bad Slug!", out.String())
	}
	if !strings.Contains(out.String(), "invalid_slug") && !strings.Contains(err.Error(), "invalid_slug") {
		t.Errorf("expected invalid_slug code in error/output:\n%s\n%v", out.String(), err)
	}
}

// TestCreate_SlugFlag_IssueAlwaysNewOrdinal verifies that each create issue call
// allocates a new ordinal even when --slug is constant. Issues use numbered
// filenames; there is no "already_exists" response for issues.
// A numbered issue's slug is its idempotency key (agent-cli-principles §6):
// re-creating the same slug with identical content is a no-op, while a
// genuinely-new slug gets the next ordinal. Distinct issues are disambiguated
// by an explicit distinct --slug, not by minting duplicate same-slug ordinals.
func TestCreate_SlugFlag_IssueIdempotentBySlug(t *testing.T) {
	setupVault(t)
	args := []string{
		"create", "issue",
		"--project", "demo",
		"--title", "Same title",
		"--description", "same",
		"--goal", "goal",
		"--slug", "stable-slug",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--json",
	}

	cmd := newRootCmd()
	out1, _, err := runCmd(t, cmd, args...)
	if err != nil {
		t.Fatalf("first create: %v\n%s", err, out1)
	}
	if !strings.Contains(out1, `"status":"created"`) {
		t.Errorf("first run status not 'created': %s", out1)
	}
	if !strings.Contains(out1, `demo.0001.stable-slug`) {
		t.Errorf("first run id missing expected numbered format: %s", out1)
	}

	// Same slug + identical content → idempotent no-op, same id.
	cmd2 := newRootCmd()
	out2, _, err := runCmd(t, cmd2, args...)
	if err != nil {
		t.Fatalf("second create: %v\n%s", err, out2)
	}
	if !strings.Contains(out2, `"status":"already_exists"`) {
		t.Errorf("second run with identical slug should be a no-op (already_exists): %s", out2)
	}
	if !strings.Contains(out2, `demo.0001.stable-slug`) {
		t.Errorf("second run should resolve to the existing id, not a new ordinal: %s", out2)
	}

	// A distinct slug gets the next ordinal.
	other := append([]string{}, args...)
	for i, a := range other {
		if a == "stable-slug" {
			other[i] = "other-slug"
		}
	}
	cmd3 := newRootCmd()
	out3, _, err := runCmd(t, cmd3, other...)
	if err != nil {
		t.Fatalf("third create: %v\n%s", err, out3)
	}
	if !strings.Contains(out3, `"status":"created"`) || !strings.Contains(out3, `demo.0002.other-slug`) {
		t.Errorf("distinct slug should mint ordinal 0002: %s", out3)
	}
}

func TestCreate_SlugFlag_AppliesToDecisionsToo(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "decision",
		"--topic", "go-rewrite",
		"--title", "Pick a sqlite driver",
		"--description", "decision",
		"--slug", "modernc-driver",
		"--tags", "domain/dev-tools,activity/research",
		"--allow-new-facet=domain",
		"--allow-new-facet=activity",
		"--json",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, out.String())
	}
	// Decision id format is <topic>.NNNN-<slug>, slug must come from --slug.
	if !strings.Contains(out.String(), "go-rewrite.0001-modernc-driver") {
		t.Errorf("expected slug-bearing decision id in output:\n%s", out.String())
	}
}

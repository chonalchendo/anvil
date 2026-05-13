package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreate_SlugFlag_OverridesTitleDerivation(t *testing.T) {
	vault := setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "issue",
		"--project", "demo",
		"--title", "Investigate the very long auto-derived slug that would be cut",
		"--description", "x",
		"--slug", "custom-slug",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, out.String())
	}
	path := filepath.Join(vault, "70-issues", "demo.custom-slug.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
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

func TestCreate_SlugFlag_IdempotentOnReRun(t *testing.T) {
	setupVault(t)
	args := []string{
		"create", "issue",
		"--project", "demo",
		"--title", "Same title",
		"--description", "same",
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

	cmd2 := newRootCmd()
	out2, _, err := runCmd(t, cmd2, args...)
	if err != nil {
		t.Fatalf("second create: %v\n%s", err, out2)
	}
	if !strings.Contains(out2, `"status":"already_exists"`) {
		t.Errorf("second run status not 'already_exists': %s", out2)
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

// TestCreate_Plan_DefaultsSlugFromIssueLink locks in the contract: a plan
// created with --issue and no --slug derives its slug from the issue's slug,
// not from the plan's own title. Prevents the connective-token drift bug
// (issue title "X with Y" + plan title "X + Y" producing
// `foo.x-with-y` vs `foo.x-y` linked artifacts).
func TestCreate_Plan_DefaultsSlugFromIssueLink(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "plan",
		"--title", "totally different plan title",
		"--description", "x",
		"--issue", "[[issue.foo.bootstrap-with-pre-parse]]",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, out.String())
	}
	path := filepath.Join(vault, "80-plans", "foo.bootstrap-with-pre-parse.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected plan slug to derive from issue: missing %s: %v\n%s",
			path, err, out.String())
	}
}

// TestCreate_Plan_SlugFlagOverridesIssueDerivation asserts --slug still wins
// over the issue-derived default — needed for the fan-out case (multiple
// plans per issue).
func TestCreate_Plan_SlugFlagOverridesIssueDerivation(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "plan",
		"--title", "phase 2",
		"--description", "x",
		"--issue", "[[issue.foo.bootstrap-with-pre-parse]]",
		"--slug", "bootstrap-phase-2",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, out.String())
	}
	path := filepath.Join(vault, "80-plans", "foo.bootstrap-phase-2.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected --slug to override issue derivation: missing %s\n%s",
			path, out.String())
	}
}

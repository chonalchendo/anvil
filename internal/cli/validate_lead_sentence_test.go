package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

const overLongLeadSentence = "This opening sentence keeps going well past the twenty five word limit that the writing issue skill now prescribes for a lead sentence so the validator has to notice it."

func TestValidate_SingleFile_OverLongLeadSentence_WarnsNotFails(t *testing.T) {
	vault := setupVault(t)

	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "issue.foo.0001.long-lead.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "long lead", "created": "2026-08-06",
			"status": "open", "project": "foo", "goal": "fixed",
			"description": "test", "severity": "low", "tags": []any{"domain/vault"},
		},
		Body: "\n## Problem\n" + overLongLeadSentence + "\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", a.Path, "--json"})
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("an over-long lead sentence must not fail validate, got: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), errfmt.CodeLeadSentence) {
		t.Errorf("output should carry code %q, got: %s", errfmt.CodeLeadSentence, out.String())
	}
	if !strings.Contains(out.String(), errfmt.SeverityWarning) {
		t.Errorf("output should carry severity %q, got: %s", errfmt.SeverityWarning, out.String())
	}
}

func TestValidate_VaultWideSweep_SkipsLeadSentence(t *testing.T) {
	vault := setupVault(t)

	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "issue.foo.0001.long-lead.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "long lead", "created": "2026-08-06",
			"status": "open", "project": "foo", "goal": "fixed",
			"description": "test", "severity": "low", "tags": []any{"domain/vault"},
		},
		Body: "\n## Problem\n" + overLongLeadSentence + "\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntrue\n```\n\n## Links\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", "--json", vault})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()

	if strings.Contains(out.String(), errfmt.CodeLeadSentence) {
		t.Errorf("vault-wide sweep must not report lead_sentence (back-catalogue noise, anvil.0274 non-goal), got: %s", out.String())
	}
}

func TestCreateIssue_OverLongLeadSentence_WarnsButCreates(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	body := "\n## Problem\n" + overLongLeadSentence + "\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntest -f /nonexistent-red-until-fixed\n```\n\n## Links\n"
	bodyPath := filepath.Join(t.TempDir(), "issue-body.md")
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "issue",
		"--title", "long-lead",
		"--description", "test",
		"--goal", "goal",
		"--body-file", bodyPath,
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("an over-long lead sentence must not fail create, got: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), errfmt.CodeLeadSentence) {
		t.Errorf("stderr should mention %q, got: %s", errfmt.CodeLeadSentence, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.foo.0001.long-lead.md")); err != nil {
		t.Errorf("artifact must still be created despite the warning; stat err = %v", err)
	}
}

func TestCreateIssue_OverLongLeadSentence_JSONStaysCleanOnStdout(t *testing.T) {
	// Under --json, validateBeforeCreate must not print the human-text
	// finding to stderr: a `--json` caller expects a clean success envelope
	// and no side-channel noise (full envelope-threading of warning findings
	// is a separate follow-up; this only guards against the misleading
	// stderr print observed in review).
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	body := "\n## Problem\n" + overLongLeadSentence + "\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntest -f /nonexistent-red-until-fixed\n```\n\n## Links\n"
	bodyPath := filepath.Join(t.TempDir(), "issue-body.md")
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "issue",
		"--title", "long-lead",
		"--description", "test",
		"--goal", "goal",
		"--body-file", bodyPath,
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--json",
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("an over-long lead sentence must not fail create --json, got: %v\nstderr: %s", err, stderr.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse json: %v\n%s", err, stdout.String())
	}
	if payload["status"] != "created" {
		t.Errorf("payload status = %v, want created (a warning-only finding must not surface as an error envelope)", payload["status"])
	}
	if strings.Contains(stderr.String(), errfmt.CodeLeadSentence) {
		t.Errorf("stderr must stay clean under --json, got: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.foo.0001.long-lead.md")); err != nil {
		t.Errorf("artifact must still be created despite the warning; stat err = %v", err)
	}
}

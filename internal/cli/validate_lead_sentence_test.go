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

func TestCreateIssue_OverLongLeadSentence_JSONCarriesWarning(t *testing.T) {
	// Under --json the finding rides the success envelope's warnings array,
	// not stderr.
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
	ws, _ := payload["warnings"].([]any)
	found := false
	for _, w := range ws {
		m, _ := w.(map[string]any)
		if m["kind"] == "validation" && m["code"] == errfmt.CodeLeadSentence {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings must carry a validation %s entry, got: %v", errfmt.CodeLeadSentence, payload["warnings"])
	}
	if strings.Contains(stderr.String(), errfmt.CodeLeadSentence) {
		t.Errorf("stderr must stay clean under --json, got: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.foo.0001.long-lead.md")); err != nil {
		t.Errorf("artifact must still be created despite the warning; stat err = %v", err)
	}
}

func TestPromoteIssue_OverLongLeadSentence_JSONCarriesWarning(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	var buf bytes.Buffer
	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "promote long lead", "--json"})
	add.SetOut(&buf)
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	var inbox struct{ ID string }
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &inbox); err != nil {
		t.Fatalf("parse inbox json: %v", err)
	}

	body := "\n## Problem\n" + overLongLeadSentence + "\n\n## Non-goals\nng\n\n## Verification\n\n### Direct\n```bash\ntrue\n```\n\n### Indirect\n```bash\ntest -f /nonexistent-red-until-fixed\n```\n\n## Links\n"
	bodyPath := filepath.Join(t.TempDir(), "issue-body.md")
	if err := os.WriteFile(bodyPath, []byte(body), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	isolateRootEnv(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", inbox.ID, "--as", "issue", "--json",
		"--description", "test", "--goal", "goal", "--body-file", bodyPath,
		"--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v\nstderr: %s", err, stderr.String())
	}
	var payload struct {
		Warnings []map[string]string `json:"warnings"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &payload); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout.String())
	}
	found := false
	for _, w := range payload.Warnings {
		if w["kind"] == "validation" && w["code"] == errfmt.CodeLeadSentence {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings must carry a validation %s entry, got: %v", errfmt.CodeLeadSentence, payload.Warnings)
	}
	if strings.Contains(stderr.String(), errfmt.CodeLeadSentence) {
		t.Errorf("stderr must stay clean under --json, got: %s", stderr.String())
	}
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestValidate_GoodVault(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	// Add one valid issue.
	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "good"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	val := newRootCmd()
	val.SetArgs([]string{"validate", vault})
	if err := val.Execute(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestValidate_BadFrontmatter(t *testing.T) {
	vault := setupVault(t)

	// Plant an issue with invalid status.
	bad := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "foo.bad.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "created": "2026-04-29",
			"status": "totally-bogus",
		},
		Body: "",
	}
	if err := bad.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", vault})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected validation error")
	}
}

func TestValidate_DefaultsToAnvilVault(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_VAULT", vault)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate empty vault failed: %v", err)
	}
	_ = os.Remove // silence unused if not needed
}

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestPromote_TopLevel_Issue(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "broken thing", "--suggested-project", "foo"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 inbox file, got %d", len(entries))
	}
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "issue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "00-inbox", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "promoted" {
		t.Errorf("status = %v", a.FrontMatter["status"])
	}
}

func TestPromote_TopLevel_Idempotent(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"promote", id, "--as", "thread"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := newRootCmd()
	second.SetArgs([]string{"promote", id, "--as", "thread"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second: %v", err)
	}
	if !strings.HasPrefix(out.String(), "already promoted ") {
		t.Errorf("output = %q", out.String())
	}
}

func TestPromote_TopLevel_InvalidAsSuggestsTopLevelCommand(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "isue"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "anvil promote ") {
		t.Errorf("corrected line should reference top-level promote: %q", err.Error())
	}
}

func TestPromote_TopLevel_JSON(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "thread", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}
	var r struct {
		ID, Status string
		TargetID   *string `json:"target_id"`
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if r.Status != "promoted" || r.TargetID == nil || *r.TargetID == "" {
		t.Errorf("unexpected: %+v", r)
	}
}

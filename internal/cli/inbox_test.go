package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestInbox_Add(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "add", "--title", "streaming feels laggy"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Errorf("expected 1 inbox file, got %d", len(entries))
	}
}

func TestInbox_List(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "list"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestInbox_Promote_Issue(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "broken thing", "--suggested-type", "issue", "--suggested-project", "foo"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Fatal("expected 1 inbox file")
	}
	inboxID := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", inboxID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "00-inbox", inboxID+".md")); !os.IsNotExist(err) {
		t.Errorf("inbox file should be deleted: %v", err)
	}
	issuePath := filepath.Join(vault, "70-issues", "foo.broken-thing.md")
	a, err := core.LoadArtifact(issuePath)
	if err != nil {
		t.Fatalf("expected issue at %s: %v", issuePath, err)
	}
	if a.FrontMatter["promoted_from"] != inboxID {
		t.Errorf("promoted_from = %v, want %s", a.FrontMatter["promoted_from"], inboxID)
	}
}

func TestInbox_Promote_MissingSuggestedType(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: missing suggested_type")
	}
}

func TestInbox_Promote_LearningOutOfScope(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x", "--suggested-type", "learning"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "out of scope") {
		t.Errorf("expected out-of-scope error, got %v", err)
	}
}

func TestInbox_Promote_Discard(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x", "--suggested-type", "discard"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "00-inbox", id+".md")); !os.IsNotExist(err) {
		t.Error("inbox file should be deleted")
	}
	issues, _ := os.ReadDir(filepath.Join(vault, "70-issues"))
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

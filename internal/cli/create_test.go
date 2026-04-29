package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func setupVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ANVIL_VAULT", dir)
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCreate_Issue_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "Fix login bug"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create issue: %v\nstdout: %s", err, out.String())
	}

	path := filepath.Join(vault, "70-issues", "foo.fix-login-bug.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s", path)
	}
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["title"] != "Fix login bug" {
		t.Errorf("title = %v", a.FrontMatter["title"])
	}
	if err := schema.Validate("issue", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestCreate_Milestone_RequiresOrdinalAndTargetDate(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "milestone", "--title", "first"})
	var stderr bytes.Buffer
	cmd.SetOut(&stderr)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: missing --ordinal / --target-date")
	}

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"create", "milestone", "--title", "first", "--ordinal", "1", "--target-date", "2026-05-15"})
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestCreate_Inbox_NoProjectNeeded(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir()) // not a git repo

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "inbox", "--title", "Streaming feels laggy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create inbox: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if err != nil || len(entries) != 1 {
		t.Errorf("expected 1 file in 00-inbox, got %d (%v)", len(entries), err)
	}
	if !strings.HasSuffix(entries[0].Name(), "-streaming-feels-laggy.md") {
		t.Errorf("got %s", entries[0].Name())
	}
}

func TestCreate_JSON_ReturnsIDAndPath(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "x", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got["id"] != "foo.x" {
		t.Errorf("id = %q", got["id"])
	}
	if !strings.HasPrefix(got["path"], vault) {
		t.Errorf("path = %q, expected under %q", got["path"], vault)
	}
}

func TestCreate_Decision_TopicScoped(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "decision", "--title", "use jwt", "--topic", "auth"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "30-decisions", "auth.0001-use-jwt.md")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("missing %s", path)
	}
}

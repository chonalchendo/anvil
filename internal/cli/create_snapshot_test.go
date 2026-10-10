package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

type snapshotEnvelope struct {
	Status   string              `json:"status"`
	Snapshot string              `json:"snapshot"`
	Warnings []map[string]string `json:"warnings"`
}

func snapshotBody(intro string) string {
	return "## Problem\n" + intro + "\n## Acceptance criteria\n- ok\n## Non-goals\n- none\n## Verification\n\n### Direct\njust test\n\n### Indirect\nsmoke\n\n## Links\n- none"
}

func snapshotIssueArgs(body string, extra ...string) []string {
	return append([]string{
		"create", "issue", "--title", "Snap", "--description", "d",
		"--goal", "g", "--tags", "domain/dev-tools", "--allow-new-facet=domain",
		"--body", body, "--json",
	}, extra...)
}

func runSnapshotUpdate(t *testing.T, body string) snapshotEnvelope {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(snapshotIssueArgs(body, "--update"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var env snapshotEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return env
}

func vaultGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...) //nolint:gosec // G204: test-only fixed git args
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// initVaultRepo makes root a git repo with its own identity, so the commit the
// code under test makes does not depend on the machine's global git config.
func initVaultRepo(t *testing.T, root string) {
	t.Helper()
	vaultGit(t, root, "init", "-q")
	vaultGit(t, root, "config", "user.email", "t@t")
	vaultGit(t, root, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "seed.md"), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	vaultGit(t, root, "add", "-A")
	vaultGit(t, root, "commit", "-qm", "init")
}

func TestCreate_UpdateSnapshot_CommitsOnlyTheArtifact(t *testing.T) {
	root := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)
	initVaultRepo(t, root)

	path := createIssueGetPath(t, snapshotIssueArgs(snapshotBody("old-marker"))...)
	other := filepath.Join(root, "other.md")
	if err := os.WriteFile(other, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	vaultGit(t, root, "add", "other.md")

	env := runSnapshotUpdate(t, snapshotBody("new-marker"))
	if env.Status != "updated" || env.Snapshot == "" {
		t.Fatalf("envelope = %+v, want updated with a snapshot", env)
	}
	rel, _ := filepath.Rel(root, path)
	if subj := strings.TrimSpace(vaultGit(t, root, "log", "-1", "--format=%s")); !strings.HasPrefix(subj, "anvil snapshot: issue.") {
		t.Errorf("subject = %q, want the type-qualified id", subj)
	}
	if files := strings.Fields(vaultGit(t, root, "show", "--name-only", "--format=", "HEAD")); len(files) != 1 || files[0] != rel {
		t.Errorf("HEAD files = %v, want only %s", files, rel)
	}
	if staged := strings.TrimSpace(vaultGit(t, root, "diff", "--cached", "--name-only")); staged != "other.md" {
		t.Errorf("staged = %q, want other.md still staged", staged)
	}
	if log := vaultGit(t, root, "log", "-p", "--", rel); !strings.Contains(log, "old-marker") {
		t.Errorf("vault git log lacks the prior body:\n%s", log)
	}
}

func TestCreate_UpdateSnapshot_NestedVaultWarns(t *testing.T) {
	root := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)
	parent := filepath.Dir(root) // parent repo; root itself has no .git
	vaultGit(t, parent, "init", "-q")
	vaultGit(t, parent, "config", "user.email", "t@t")
	vaultGit(t, parent, "config", "user.name", "t")
	vaultGit(t, parent, "commit", "-qm", "init", "--allow-empty")
	createIssueGetPath(t, snapshotIssueArgs(snapshotBody("old"))...)
	headBefore := vaultGit(t, parent, "rev-parse", "HEAD")

	env := runSnapshotUpdate(t, snapshotBody("new"))
	if env.Snapshot != "" || len(env.Warnings) != 1 || env.Warnings[0]["kind"] != "snapshot" {
		t.Fatalf("envelope = %+v, want one snapshot warning and no sha", env)
	}
	if after := vaultGit(t, parent, "rev-parse", "HEAD"); after != headBefore {
		t.Errorf("parent repo gained a commit: %s -> %s", headBefore, after)
	}
}

func TestCreate_UpdateSnapshot_FailureWarnsAndLeavesIndexClean(t *testing.T) {
	root := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)
	initVaultRepo(t, root)
	path := createIssueGetPath(t, snapshotIssueArgs(snapshotBody("old"))...)
	rel, _ := filepath.Rel(root, path)

	// No identity anywhere: the snapshot commit fails after `git add`.
	vaultGit(t, root, "config", "--unset", "user.email")
	vaultGit(t, root, "config", "--unset", "user.name")
	vaultGit(t, root, "config", "user.useConfigOnly", "true")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("EMAIL", "")

	env := runSnapshotUpdate(t, snapshotBody("new"))
	if env.Status != "updated" || env.Snapshot != "" {
		t.Fatalf("envelope = %+v, want updated with no snapshot", env)
	}
	if len(env.Warnings) != 1 || env.Warnings[0]["kind"] != "snapshot" {
		t.Fatalf("warnings = %v, want one snapshot warning", env.Warnings)
	}
	if staged := vaultGit(t, root, "diff", "--cached", "--name-only", "--", rel); staged != "" {
		t.Errorf("failed snapshot left %q staged", staged)
	}
	if a, err := core.LoadArtifact(path); err != nil || !strings.Contains(a.Body, "new") {
		t.Errorf("--update did not proceed after the snapshot failure")
	}
}

func TestCreate_UpdateSnapshot_NonGitVaultWarns(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)
	createIssueGetPath(t, snapshotIssueArgs(snapshotBody("old"))...)

	env := runSnapshotUpdate(t, snapshotBody("new"))
	if env.Status != "updated" || env.Snapshot != "" {
		t.Fatalf("envelope = %+v, want updated with no snapshot", env)
	}
	if len(env.Warnings) != 1 || env.Warnings[0]["kind"] != "snapshot" {
		t.Errorf("warnings = %v, want one snapshot warning", env.Warnings)
	}
}

func TestCreate_UpdateSnapshot_NoOpTakesNone(t *testing.T) {
	root := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)
	initVaultRepo(t, root)
	createIssueGetPath(t, snapshotIssueArgs(snapshotBody("same"))...)
	before := vaultGit(t, root, "status", "--porcelain")

	env := runSnapshotUpdate(t, snapshotBody("same"))
	if env.Status != "already_exists" || env.Snapshot != "" {
		t.Fatalf("envelope = %+v, want already_exists with no snapshot", env)
	}
	if after := vaultGit(t, root, "status", "--porcelain"); after != before {
		t.Errorf("no-op changed vault git state:\n%s\nvs\n%s", before, after)
	}
}

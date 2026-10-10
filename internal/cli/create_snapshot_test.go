package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestCreate_UpdateSnapshot_CommitsOnlyTheArtifact(t *testing.T) {
	root := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)
	vaultGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "seed.md"), []byte("seed"), 0o600); err != nil {
		t.Fatal(err)
	}
	vaultGit(t, root, "add", "-A")
	vaultGit(t, root, "commit", "-qm", "init")

	path := createIssueGetPath(t, snapshotIssueArgs(snapshotBody("old-marker"))...)
	other := filepath.Join(root, "other.md")
	if err := os.WriteFile(other, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}

	env := runSnapshotUpdate(t, snapshotBody("new-marker"))
	if env.Status != "updated" || env.Snapshot == "" {
		t.Fatalf("envelope = %+v, want updated with a snapshot", env)
	}
	rel, _ := filepath.Rel(root, path)
	if log := vaultGit(t, root, "log", "-p", "--", rel); !strings.Contains(log, "old-marker") {
		t.Errorf("vault git log lacks the prior body:\n%s", log)
	}
	if st := vaultGit(t, root, "status", "--porcelain"); !strings.Contains(st, "other.md") {
		t.Errorf("unrelated file was swept into the snapshot; status:\n%s", st)
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
	vaultGit(t, root, "init", "-q")
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

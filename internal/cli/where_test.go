package cli

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitRepo(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := runIn(dir, "git", "init", "-q"); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if out, err := runIn(dir, "git", "remote", "add", "origin", remote); err != nil {
		t.Fatalf("git remote add: %v %s", err, out)
	}
	return dir
}

func runIn(dir string, name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	return c.CombinedOutput()
}

func TestWhere_PrintsVaultAndProject(t *testing.T) {
	dir := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANVIL_VAULT", filepath.Join(t.TempDir(), "vault"))
	t.Chdir(dir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"where"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "vault:") || !strings.Contains(got, "project: foo") {
		t.Errorf("unexpected output:\n%s", got)
	}
}

func TestWhere_JSON(t *testing.T) {
	dir := setupGitRepo(t, "git@github.com:acme/bar.git")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ANVIL_VAULT", filepath.Join(t.TempDir(), "vault"))
	t.Chdir(dir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"where", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got["project"] != "bar" {
		t.Errorf("project = %q, want bar", got["project"])
	}
	if got["vault"] == "" || got["project_root"] == "" {
		t.Errorf("missing keys: %v", got)
	}
}

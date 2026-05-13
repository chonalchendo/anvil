package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReindexEmptyVault(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"reindex"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "0 artifacts") {
		t.Fatalf("stdout: %q", stdout.String())
	}
}

func TestReindexJSONShape(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	if err := os.MkdirAll(filepath.Join(vault, "70-issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntype: issue\nid: a\nstatus: open\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "a.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"reindex", "--json"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got struct {
		Artifacts  int   `json:"artifacts"`
		Links      int   `json:"links"`
		DurationMS int64 `json:"duration_ms"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json: %v\noutput: %s", err, stdout.String())
	}
	if got.Artifacts != 1 {
		t.Fatalf("artifacts: got %d want 1", got.Artifacts)
	}
}

func TestReindexWarnsOnStub(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	stubPath := filepath.Join(vault, "issue.burgh.fake.md")
	if err := os.WriteFile(stubPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"reindex"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(stderr.String(), "issue.burgh.fake.md") {
		t.Fatalf("expected stub warning on stderr, got: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--prune-stubs") {
		t.Fatalf("expected prune hint, got: %q", stderr.String())
	}
	if _, err := os.Stat(stubPath); err != nil {
		t.Fatalf("stub should still exist without --prune-stubs: %v", err)
	}
}

func TestReindexPruneStubs(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)

	emptyStub := filepath.Join(vault, "issue.burgh.empty.md")
	if err := os.WriteFile(emptyStub, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	// Non-zero stub: name matches but file has content. Must NOT be deleted.
	contentStub := filepath.Join(vault, "plan.anvil.content.md")
	if err := os.WriteFile(contentStub, []byte("user typed something here"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Real artifact in canonical dir. Must NOT be touched.
	if err := os.MkdirAll(filepath.Join(vault, "70-issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	realArtifact := filepath.Join(vault, "70-issues", "anvil.real.md")
	if err := os.WriteFile(realArtifact, []byte("---\ntype: issue\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"reindex", "--prune-stubs"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if _, err := os.Stat(emptyStub); !os.IsNotExist(err) {
		t.Fatalf("empty stub should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(contentStub); err != nil {
		t.Fatalf("non-zero stub must be preserved: %v", err)
	}
	if _, err := os.Stat(realArtifact); err != nil {
		t.Fatalf("canonical artifact must be preserved: %v", err)
	}
	if !strings.Contains(stderr.String(), "pruned") {
		t.Fatalf("expected prune log, got: %q", stderr.String())
	}
}

func TestReindexPruneStubsJSON(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	if err := os.WriteFile(filepath.Join(vault, "issue.x.md"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"reindex", "--prune-stubs", "--json"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got struct {
		Stubs  []string `json:"stubs"`
		Pruned []string `json:"pruned"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if len(got.Stubs) != 1 || len(got.Pruned) != 1 {
		t.Fatalf("json shape: stubs=%v pruned=%v", got.Stubs, got.Pruned)
	}
}

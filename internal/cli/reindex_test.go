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

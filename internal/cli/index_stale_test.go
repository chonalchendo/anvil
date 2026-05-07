package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestListReadyReturnsIndexStaleWhenVaultEditedExternally(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t, vault)

	// External edit + bump dir mtime so CheckFreshness sees drift.
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "demo.bar.md"),
		[]byte("---\ntype: issue\nid: demo.bar\nstatus: open\n---\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(1 * time.Second)
	if err := os.Chtimes(vault, now, now); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"list", "issue", "--ready"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected ErrIndexStale; output: %s", out.String())
	}
	if !strings.Contains(err.Error(), "index_stale") {
		t.Fatalf("expected index_stale in error message: %v", err)
	}
}

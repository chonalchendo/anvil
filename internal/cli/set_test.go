package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestSet_Status_Succeeds(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "status", "resolved"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v\n%s", err, out.String())
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "resolved" {
		t.Errorf("status = %v", a.FrontMatter["status"])
	}
}

func TestSet_InvalidEnum_ReturnsSchemaInvalid(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "status", "bogus"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Errorf("err = %v, want ErrSchemaInvalid", err)
	}
}

func TestSet_ListField_Rejected(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "anything"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error")
	}
}

func TestSet_MissingArtifact_NotFound(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "ghost", "status", "external"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("err = %v, want ErrArtifactNotFound", err)
	}
}

func TestSetPlan_StatusLocked_ValidatesFirst(t *testing.T) {
	vault := setupVault(t)
	src, err := os.ReadFile(filepath.Join("testdata", "plan_dangling.md"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(vault, "80-plans", "ANV-142-streaming-token-counter.md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, src, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "plan", "ANV-142-streaming-token-counter", "status", "locked"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err = cmd.Execute()
	if !errors.Is(err, core.ErrPlanDAG) {
		t.Errorf("err = %v, want ErrPlanDAG", err)
	}
}

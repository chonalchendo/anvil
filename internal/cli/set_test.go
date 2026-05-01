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
	cmd.SetArgs([]string{"set", "issue", "ghost", "status", "open"})
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

func TestSet_IssueMilestone_RoundTrip(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "anvil", "x", "X")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "anvil.x", "milestone", "[[milestone.anvil.cli-substrate]]"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "anvil.x.md"))
	if got, _ := a.FrontMatter["milestone"].(string); got != "[[milestone.anvil.cli-substrate]]" {
		t.Errorf("got %q", got)
	}
}

func TestSet_MilestoneSystemDesign_RoundTrip(t *testing.T) {
	vault := setupVault(t)
	// Write a minimal valid milestone fixture matching the new schema.
	mPath := filepath.Join(vault, "85-milestones", "anvil.cli-substrate.md")
	if err := os.MkdirAll(filepath.Dir(mPath), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &core.Artifact{
		Path: mPath,
		FrontMatter: map[string]any{
			"type": "milestone", "title": "CLI substrate", "created": "2026-04-29",
			"updated": "2026-04-29", "status": "planned", "project": "anvil",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "milestone", "anvil.cli-substrate", "system_design", "[[system-design.anvil]]"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, _ := core.LoadArtifact(mPath)
	if v, _ := got.FrontMatter["system_design"].(string); v != "[[system-design.anvil]]" {
		t.Errorf("system_design = %q", v)
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

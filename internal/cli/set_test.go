package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestSet_ArraySingleArg_Replaces(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "domain/dev-tools"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["tags"].([]any)
	if len(got) != 1 || got[0] != "domain/dev-tools" {
		t.Errorf("tags = %v", got)
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
			"type": "milestone", "title": "CLI substrate", "description": "fixture description", "created": "2026-04-29",
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
	t.Setenv("ANVIL_VAULT", vault)
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

	// Seed the index at the on-disk state so the post-rollback assertion
	// can detect a leak from the transient `locked` write.
	reindex := newRootCmd()
	reindex.SetArgs([]string{"reindex"})
	reindex.SetOut(&bytes.Buffer{})
	reindex.SetErr(&bytes.Buffer{})
	if err := reindex.Execute(); err != nil {
		t.Fatalf("seed reindex: %v", err)
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

	// Index must reflect the rolled-back state, not the transient `locked`
	// the file briefly held before the dangling-DAG validator rejected it.
	row, ierr := openIndex(t, vault).GetArtifact("ANV-142")
	if ierr != nil {
		t.Fatalf("expected plan in index after rollback: %v", ierr)
	}
	if row.Status != "draft" {
		t.Errorf("index status after rollback = %q, want draft", row.Status)
	}
}

func TestSet_ArrayReplace_Multiple(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "criterion 1", "criterion 2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 2 || got[0] != "criterion 1" || got[1] != "criterion 2" {
		t.Errorf("acceptance = %v", got)
	}
}

func TestSet_ArrayAdd_Appends(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	c1 := newRootCmd()
	c1.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x", "y"})
	if err := c1.Execute(); err != nil {
		t.Fatal(err)
	}
	c2 := newRootCmd()
	c2.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", "z"})
	if err := c2.Execute(); err != nil {
		t.Fatal(err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 3 || got[2] != "z" {
		t.Errorf("acceptance = %v", got)
	}
}

func TestSet_ArrayRemove_Index(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	c1 := newRootCmd()
	c1.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x", "y", "z"})
	if err := c1.Execute(); err != nil {
		t.Fatal(err)
	}
	c2 := newRootCmd()
	c2.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--remove", "0"})
	if err := c2.Execute(); err != nil {
		t.Fatal(err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 2 || got[0] != "y" || got[1] != "z" {
		t.Errorf("acceptance = %v", got)
	}
}

func TestSet_ArrayRemove_OOB_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	c1 := newRootCmd()
	c1.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x"})
	_ = c1.Execute()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--remove", "5"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected OOB error")
	}
}

func TestSet_AddAndRemove_Together_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", "x", "--remove", "0"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected mutual-exclusion error")
	}
}

func TestSet_AddOnScalar_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "status", "--add", "open"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: --add on scalar field")
	}
}

func TestSet_ScalarMultipleArgs_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "status", "open", "extra"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: scalar with multiple positional values")
	}
}

func TestSet_UnknownField_ScalarPath(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "ad_hoc_field", "value"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err != nil && !errors.Is(err, ErrSchemaInvalid) {
		t.Errorf("expected ErrSchemaInvalid (or success), got %v", err)
	}
}

func TestSet_Tags_CommaSeparatedValueSplits(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "domain/dev-tools,activity/cleanup", "--allow-new-facet=activity"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["tags"].([]any)
	if len(got) != 2 || got[0] != "domain/dev-tools" || got[1] != "activity/cleanup" {
		t.Errorf("tags = %v, want [domain/dev-tools activity/cleanup]", got)
	}
}

func TestSet_Tags_RejectsUnknownFacetValue(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "--add", "domain/quantum-physics"})
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)
	cmd.SetOut(&errOut)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected rejection for unknown domain via set --add")
	}
	if !strings.Contains(errOut.String(), "unknown_facet_value") {
		t.Errorf("expected unknown_facet_value: %q", errOut.String())
	}
}

func TestSet_Tags_AllowNewFacetAccepts(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "--add", "domain/quantum-physics", "--allow-new-facet=domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success: %v", err)
	}
}

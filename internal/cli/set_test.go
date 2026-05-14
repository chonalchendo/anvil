package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestSet_PrintsConfirmation_Scalar(t *testing.T) {
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
	got := strings.TrimSpace(out.String())
	if got != "foo.a: status open → resolved" {
		t.Errorf("output = %q, want %q", got, "foo.a: status open → resolved")
	}
}

func TestSet_JSONEnvelope_Scalar(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "status", "resolved", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v\n%s", err, out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if got["id"] != "foo.a" {
		t.Errorf("id = %v", got["id"])
	}
	if got["field"] != "status" {
		t.Errorf("field = %v", got["field"])
	}
	if got["from"] != "open" {
		t.Errorf("from = %v", got["from"])
	}
	if got["to"] != "resolved" {
		t.Errorf("to = %v", got["to"])
	}
	if got["status"] != "set" {
		t.Errorf("status = %v", got["status"])
	}
	if _, ok := got["path"].(string); !ok {
		t.Errorf("path missing or non-string: %v", got["path"])
	}
}

func TestSet_PrintsConfirmation_ArrayAdd(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v\n%s", err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "foo.a: acceptance") || !strings.Contains(got, "x") {
		t.Errorf("output = %q", got)
	}
}

func TestSet_PrintsConfirmation_ArrayRemove(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	addCmd := newRootCmd()
	addCmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", "x"})
	addCmd.SetOut(&bytes.Buffer{})
	addCmd.SetErr(&bytes.Buffer{})
	if err := addCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--remove", "0"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "foo.a: acceptance") || !strings.Contains(got, "x") {
		t.Errorf("output = %q", got)
	}
}

func TestSet_JSONEnvelope_ArrayAdd(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", "x", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v\n%s", err, out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if got["id"] != "foo.a" || got["field"] != "acceptance" || got["status"] != "added" {
		t.Errorf("envelope = %#v", got)
	}
	to, ok := got["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "x" {
		t.Errorf("to = %#v", got["to"])
	}
}

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

func TestSet_ArrayPositional_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "criterion"})
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)
	cmd.SetOut(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for positional value on array field")
	}
	if !strings.Contains(err.Error(), "field_is_array") {
		t.Errorf("expected field_is_array token, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "--add") {
		t.Errorf("expected error to mention --add: %q", err.Error())
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	if _, present := a.FrontMatter["acceptance"]; present {
		t.Errorf("acceptance must not be written after error, got %v", a.FrontMatter["acceptance"])
	}
}

func TestSet_TagsPositional_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "domain/dev-tools"})
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)
	cmd.SetOut(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for positional value on tags (array)")
	}
	if !strings.Contains(err.Error(), "field_is_array") {
		t.Errorf("expected field_is_array token, got %q", err.Error())
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

func TestSet_ArrayAdd_Appends(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	for _, v := range []string{"x", "y", "z"} {
		c := newRootCmd()
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", v})
		if err := c.Execute(); err != nil {
			t.Fatalf("--add %s: %v", v, err)
		}
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 3 || got[0] != "x" || got[1] != "y" || got[2] != "z" {
		t.Errorf("acceptance = %v", got)
	}
}

func TestSet_ArrayRemove_Index(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	for _, v := range []string{"x", "y", "z"} {
		c := newRootCmd()
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", v})
		if err := c.Execute(); err != nil {
			t.Fatalf("--add %s: %v", v, err)
		}
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
	c1.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", "x"})
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

func TestSet_ArrayRemove_ByValue(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	for _, v := range []string{"x", "y", "z"} {
		c := newRootCmd()
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", v})
		if err := c.Execute(); err != nil {
			t.Fatalf("--add %s: %v", v, err)
		}
	}
	c2 := newRootCmd()
	c2.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--remove", "y"})
	if err := c2.Execute(); err != nil {
		t.Fatal(err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 2 || got[0] != "x" || got[1] != "z" {
		t.Errorf("acceptance = %v", got)
	}
}

func TestSet_ArrayRemove_UnknownValue_CleanError(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	c1 := newRootCmd()
	c1.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--add", "x"})
	_ = c1.Execute()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--remove", "domain/skills"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown value")
	}
	if strings.Contains(err.Error(), "strconv") {
		t.Errorf("error leaked strconv internals: %v", err)
	}
	if !strings.Contains(err.Error(), "domain/skills") {
		t.Errorf("error did not name the offending value: %v", err)
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

// Removing the only tag matching a required facet pattern must surface
// `missing_required_facet` (actionable: name the facet, suggest the fix),
// not the raw schema diagnostic citing /tags/0 and a pattern mismatch on the
// surviving tag.
func TestSet_Tags_RemoveLastRequiredFacet_ReportsMissingFacet(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	// Fixture starts with [domain/dev-tools]. Removing index 0 leaves [] —
	// issue requires ≥1 domain/* tag.
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "--remove", "0"})
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)
	cmd.SetOut(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected schema-invalid")
	}
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Errorf("err = %v, want ErrSchemaInvalid", err)
	}
	got := errOut.String()
	if !strings.Contains(got, "missing_required_facet") {
		t.Errorf("expected missing_required_facet, got: %s", got)
	}
	if !strings.Contains(got, "^domain/[a-z0-9-]+$") {
		t.Errorf("expected ^domain/ pattern in expected[], got: %s", got)
	}
	// The misleading "does not match pattern" raw schema text must NOT leak.
	if strings.Contains(got, "does not match pattern") {
		t.Errorf("raw schema diagnostic leaked instead of structured error: %s", got)
	}
}

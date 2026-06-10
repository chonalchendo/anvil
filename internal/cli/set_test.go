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
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x", "--add"})
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
	addCmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x"})
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
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x", "--json"})
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

// Positional values on array fields are treated as implicit appends; no --add flag required.
func TestSet_ArrayPositional_Appends(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "criterion"})
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success for positional value on array field: %v\n%s", err, out.String())
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 1 || got[0] != "criterion" {
		t.Errorf("acceptance = %v, want [criterion]", got)
	}
}

// Positional values on array fields work for tags too; facet validation still applies.
func TestSet_TagsPositional_Appends(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	cmd := newRootCmd()
	// Fixture already has [domain/dev-tools]; appending the same value via positional form.
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "domain/dev-tools"})
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success for positional value on tags: %v\n%s", err, out.String())
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["tags"].([]any)
	if len(got) < 1 {
		t.Errorf("tags = %v, expected at least 1", got)
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

// TestSet_IssueMilestone_BareIDNormalised asserts that a bare project.slug id is
// stored as the canonical [[milestone.project.slug]] wikilink, so the issue
// remains reachable under --milestone filters and graph edges.
func TestSet_IssueMilestone_BareIDNormalised(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "anvil", "x", "X")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "anvil.x", "milestone", "anvil.cli-substrate"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "anvil.x.md"))
	if got, _ := a.FrontMatter["milestone"].(string); got != "[[milestone.anvil.cli-substrate]]" {
		t.Errorf("got %q, want %q", got, "[[milestone.anvil.cli-substrate]]")
	}
}

func TestSet_MilestoneSystemDesign_RoundTrip(t *testing.T) {
	vault := setupVault(t)
	// Write a minimal valid milestone fixture matching the new schema.
	mPath := filepath.Join(vault, "85-milestones", "anvil.cli-substrate.md")
	if err := os.MkdirAll(filepath.Dir(mPath), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	a := &core.Artifact{
		Path: mPath,
		FrontMatter: map[string]any{
			"type": "milestone", "title": "CLI substrate", "description": "fixture description", "created": "2026-04-29",
			"updated": "2026-04-29", "status": "planned", "project": "anvil",
			"goal": "CLI substrate ships and all attached issues are resolved",
			"kind": "scoped",
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
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, src, 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", v})
		if err := c.Execute(); err != nil {
			t.Fatalf("positional %s: %v", v, err)
		}
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 3 || got[0] != "x" || got[1] != "y" || got[2] != "z" {
		t.Errorf("acceptance = %v", got)
	}
}

// Multiple positional values in a single invocation must all be appended.
func TestSet_ArrayAdd_MultiplePositionals_AppendsAll(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	c := newRootCmd()
	c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x", "y", "z"})
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	if err := c.Execute(); err != nil {
		t.Fatalf("set: %v\n%s", err, out.String())
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 3 || got[0] != "x" || got[1] != "y" || got[2] != "z" {
		t.Errorf("acceptance = %v, want [x y z]", got)
	}
}

// Repeated --remove flags in a single invocation must each remove; same
// scalar-overwrite bug as --add. Removing by value (not index) so the order
// of removal doesn't shift indices unsafely.
func TestSet_ArrayRemove_RepeatedFlags_RemovesAll(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	for _, v := range []string{"x", "y", "z"} {
		c := newRootCmd()
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", v})
		if err := c.Execute(); err != nil {
			t.Fatalf("--add %s: %v", v, err)
		}
	}
	c2 := newRootCmd()
	c2.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--remove", "x", "--remove", "z"})
	var out bytes.Buffer
	c2.SetOut(&out)
	c2.SetErr(&out)
	if err := c2.Execute(); err != nil {
		t.Fatalf("set: %v\n%s", err, out.String())
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 1 || got[0] != "y" {
		t.Errorf("acceptance = %v, want [y]", got)
	}
}

func TestSet_ArrayRemove_DuplicateStringTarget_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	for _, v := range []string{"x", "y"} {
		c := newRootCmd()
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", v})
		if err := c.Execute(); err != nil {
			t.Fatalf("--add %s: %v", v, err)
		}
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "--remove", "x", "--remove", "x"})
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected duplicate-target error")
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	got, _ := a.FrontMatter["acceptance"].([]any)
	if len(got) != 2 {
		t.Errorf("acceptance mutated on error: %v", got)
	}
}

func TestSet_ArrayRemove_Index(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	for _, v := range []string{"x", "y", "z"} {
		c := newRootCmd()
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", v})
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

func TestSet_ArrayRemove_ByValue(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")
	for _, v := range []string{"x", "y", "z"} {
		c := newRootCmd()
		c.SetArgs([]string{"set", "issue", "foo.a", "acceptance", v})
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
	c1.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x"})
	if err := c1.Execute(); err != nil {
		t.Fatalf("seed append failed: %v", err)
	}
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
	cmd.SetArgs([]string{"set", "issue", "foo.a", "acceptance", "x", "--add", "--remove", "0"})
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
	cmd.SetArgs([]string{"set", "issue", "foo.a", "status", "open", "--add"})
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
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "domain/quantum-physics"})
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
	cmd.SetArgs([]string{"set", "issue", "foo.a", "tags", "domain/quantum-physics", "--allow-new-facet=domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success: %v", err)
	}
}

// Removing the only tag matching a required facet pattern must surface
// `missing_required_facet` (actionable: name the facet, suggest the fix),
// not the raw schema diagnostic citing /tags/0 and a pattern mismatch on the
// surviving tag.
func TestSet_ReproductionAnchor_Sets(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "reproduction_anchor", "--command", "anvil --version", "--expected", "sha:deadbeef"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set reproduction_anchor: %v\n%s", err, out.String())
	}

	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	if err != nil {
		t.Fatal(err)
	}
	anchor, ok := a.FrontMatter["reproduction_anchor"].(map[string]any)
	if !ok {
		t.Fatalf("reproduction_anchor not a map: %T", a.FrontMatter["reproduction_anchor"])
	}
	if anchor["command"] != "anvil --version" {
		t.Errorf("command = %v", anchor["command"])
	}
	if anchor["expected"] != "sha:deadbeef" {
		t.Errorf("expected = %v", anchor["expected"])
	}
}

func TestSet_ReproductionAnchor_EmptyExpected(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "reproduction_anchor", "--command", "true"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set reproduction_anchor without --expected: %v\n%s", err, out.String())
	}

	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	anchor, ok := a.FrontMatter["reproduction_anchor"].(map[string]any)
	if !ok {
		t.Fatalf("reproduction_anchor not a map")
	}
	if anchor["command"] != "true" {
		t.Errorf("command = %v", anchor["command"])
	}
}

func TestSet_ReproductionAnchor_MissingCommand_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "reproduction_anchor"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --command is missing")
	}
	if !strings.Contains(err.Error(), "--command") {
		t.Errorf("error should mention --command, got: %v", err)
	}
}

func TestSet_ReproductionAnchor_PositionalValue_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "reproduction_anchor", "somevalue", "--command", "true"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for positional value on reproduction_anchor")
	}
	if !strings.Contains(err.Error(), "positional") {
		t.Errorf("error should mention positional, got: %v", err)
	}
}

func TestSet_OtherObjectField_StillErrors(t *testing.T) {
	// plan.verification is a KindObject field with no CLI authoring path.
	// The guard at set.go:173-174 must reject it with the "edit the file
	// directly" message, not route it through the reproduction_anchor handler.
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "p1", "P1")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "plan", "foo.p1", "verification", "somevalue"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: object field without CLI authoring path")
	}
	if !strings.Contains(err.Error(), "edit the file directly") {
		t.Errorf("expected 'edit the file directly' error, got: %v", err)
	}
}

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

// TestSet_Issue_ByOrdinal pins the write-path counterpart to show_test.go's
// TestShow_Issue_ByOrdinal: a bare ordinal ("0001") resolves to the full issue
// ID on the set write path, matching the read path the fix unified them with.
func TestSet_Issue_ByOrdinal(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	path := createIssueGetPath(t,
		"create", "issue",
		"--title", "Set me by ordinal",
		"--description", "d",
		"--goal", "g",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	)
	id := strings.TrimSuffix(filepath.Base(path), ".md")
	if !strings.HasPrefix(id, "foo.0001.") {
		t.Fatalf("expected first issue at ordinal 0001, got %q", id)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "0001", "milestone", "foo.cli-substrate"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set issue 0001: %v\n%s", err, out.String())
	}
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := a.FrontMatter["milestone"].(string); got != "[[milestone.foo.cli-substrate]]" {
		t.Errorf("milestone = %q, want [[milestone.foo.cli-substrate]]", got)
	}
}

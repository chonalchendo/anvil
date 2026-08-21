package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeFixtureIssueWithLinks writes an issue artifact with optional related wikilinks.
func writeFixtureIssueWithLinks(t *testing.T, vault, project, slug, title string, related []string) string {
	t.Helper()
	id := project + "." + slug
	path := filepath.Join(vault, "70-issues", id+".md")
	fm := map[string]any{
		"type": "issue", "title": title, "description": "fixture description",
		"created": "2026-04-29", "updated": "2026-04-29",
		"status": "open", "project": project, "severity": "medium",
		"tags": []any{"domain/dev-tools"}, "goal": "fixture goal is done",
	}
	if len(related) > 0 {
		raw := make([]any, len(related))
		for i, r := range related {
			raw[i] = r
		}
		fm["related"] = raw
	}
	a := &core.Artifact{
		Path:        path,
		FrontMatter: fm,
		Body:        "## Context\n\nfixture body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRename_Issue_RenamesFileAndFrontmatter(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.old-slug", "--title", "New Title"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename: %v\n%s", err, out.String())
	}

	// Old file gone; the rename mints the canonical prefixed filename.
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "foo.old-slug.md")); err == nil {
		t.Error("old file still exists")
	}
	newPath := filepath.Join(vault, "70-issues", "issue.foo.new-title.md")
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new file missing: %v", err)
	}

	a, err := core.LoadArtifact(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.FrontMatter["title"]; got != "New Title" {
		t.Errorf("title = %v, want %q", got, "New Title")
	}
	if a.FrontMatter["updated"] == "" {
		t.Error("updated not set")
	}
}

func TestRename_Issue_OutputShowsTransition(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.old-slug", "--title", "New Title"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename: %v\n%s", err, out.String())
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "foo.old-slug") || !strings.Contains(got, "foo.new-title") {
		t.Errorf("output missing transition: %q", got)
	}
}

func TestRename_Issue_JSONEnvelope(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.old-slug", "--title", "New Title", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename: %v\n%s", err, out.String())
	}
	var r renameResult
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &r); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if r.OldID != "issue.foo.old-slug" {
		t.Errorf("old_id = %q", r.OldID)
	}
	if r.NewID != "issue.foo.new-title" {
		t.Errorf("new_id = %q", r.NewID)
	}
	if r.Status != "renamed" {
		t.Errorf("status = %q", r.Status)
	}
}

func TestRename_Issue_RewritesInboundWikilinks(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")
	// Another issue that links to the first.
	writeFixtureIssueWithLinks(t, vault, "foo", "linker", "Linker",
		[]string{"[[issue.foo.old-slug]]"})

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.old-slug", "--title", "New Title"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename: %v\n%s", err, out.String())
	}

	// Linker file should now reference the new wikilink.
	linkerPath := filepath.Join(vault, "70-issues", "foo.linker.md")
	b, err := os.ReadFile(linkerPath) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if strings.Contains(content, "[[issue.foo.old-slug]]") {
		t.Error("old wikilink still present in linker file")
	}
	if !strings.Contains(content, "[[issue.foo.new-title]]") {
		t.Error("new wikilink not found in linker file")
	}
}

// The reviewer's blocker probe: a canonical prefixed, ordinal-numbered file
// renamed via its full id must keep project AND ordinal, not collapse to
// `issue.<slug>`.
func TestRename_Issue_PrefixedNumberedFile_PreservesProjectAndOrdinal(t *testing.T) {
	vault := setupVault(t)
	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", "issue.demo.0001.probe-issue-one.md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": "Probe issue one", "description": "fixture description",
			"created": "2026-04-29", "updated": "2026-04-29",
			"status": "open", "project": "demo", "severity": "medium",
			"tags": []any{"domain/dev-tools"}, "goal": "fixture goal is done",
		},
		Body: "## Context\n\nfixture body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "issue.demo.0001.probe-issue-one", "--title", "Renamed probe title", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename: %v\n%s", err, out.String())
	}
	var r renameResult
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &r); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if r.NewID != "issue.demo.0001.renamed-probe-title" {
		t.Errorf("new_id = %q, want %q", r.NewID, "issue.demo.0001.renamed-probe-title")
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.demo.0001.renamed-probe-title.md")); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.demo.0001.probe-issue-one.md")); err == nil {
		t.Error("old file still exists")
	}
}

// A bare back-catalogue file must resolve under the prefixed argument shape
// the CLI prints — rename was the one write verb outside the funnel.
func TestRename_Issue_BareBackCatalogueFile_PrefixedArg(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "issue.foo.old-slug", "--title", "New Title"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename via prefixed arg: %v\n%s", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.foo.new-title.md")); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "foo.old-slug.md")); err == nil {
		t.Error("old bare file still exists")
	}
}

func TestRename_Issue_CosmesticChange_SlugUnchanged(t *testing.T) {
	vault := setupVault(t)
	// "Old Title" and "Old   Title" both slugify to "old-title".
	writeFixtureIssue(t, vault, "foo", "old-title", "Old Title")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.old-title", "--title", "OLD TITLE"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename: %v\n%s", err, out.String())
	}

	// File still at old path.
	origPath := filepath.Join(vault, "70-issues", "foo.old-title.md")
	if _, err := os.Stat(origPath); err != nil {
		t.Fatalf("file missing after cosmetic rename: %v", err)
	}
	// Title updated.
	a, err := core.LoadArtifact(origPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.FrontMatter["title"]; got != "OLD TITLE" {
		t.Errorf("title = %v, want %q", got, "OLD TITLE")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "slug unchanged") {
		t.Errorf("expected cosmetic message, got %q", got)
	}
}

func TestRename_Issue_MissingTitle_Error(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.a"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing --title")
	}
}

func TestRename_Issue_NotFound_Error(t *testing.T) {
	setupVault(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.nonexistent", "--title", "Anything"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing artifact")
	}
}

func TestRename_Issue_TargetAlreadyExists_Error(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")
	writeFixtureIssue(t, vault, "foo", "new-title", "New Title") // collision

	cmd := newRootCmd()
	cmd.SetArgs([]string{"rename", "issue", "foo.old-slug", "--title", "New Title"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for target already existing")
	}
}

func TestRename_SlugFlagOverridesTitleDerivation(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"rename", "issue", "foo.old-slug",
		"--title", "Investigate the very long auto-derived slug that would be cut instead",
		"--slug", "custom-slug",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("rename: %v\n%s", err, out.String())
	}

	newPath := filepath.Join(vault, "70-issues", "issue.foo.custom-slug.md")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("expected %s to exist: %v", newPath, err)
	}
}

func TestRename_InvalidSlug_Error(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "old-slug", "Old Title")

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"rename", "issue", "foo.old-slug",
		"--title", "New Title",
		"--slug", "Bad Slug!",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected validation error for invalid --slug")
	}
}

func TestReplaceSlug_ProjectScoped(t *testing.T) {
	got, err := replaceSlug(core.TypeIssue, "myproject.old-slug", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "issue.myproject.new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_PrefixedNumberedIssue_KeepsProjectAndOrdinal(t *testing.T) {
	got, err := replaceSlug(core.TypeIssue, "issue.demo.0001.probe-issue-one", "renamed-probe-title", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "issue.demo.0001.renamed-probe-title" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_Milestone_KeepsPrefixAndProject(t *testing.T) {
	got, err := replaceSlug(core.TypeMilestone, "milestone.demo.old-slug", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "milestone.demo.new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_Inbox(t *testing.T) {
	got, err := replaceSlug(core.TypeInbox, "2026-05-13-old-slug", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2026-05-13-new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_Thread(t *testing.T) {
	got, err := replaceSlug(core.TypeThread, "old-slug", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_Decision(t *testing.T) {
	got, err := replaceSlug(core.TypeDecision, "mytopic.0001-old-slug", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mytopic.0001-new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_SystemDesign_ScopedShard_KeepsProject(t *testing.T) {
	got, err := replaceSlug(core.TypeSystemDesign, "mentat.ingestion-pipeline", "ingestion-cadence", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mentat.ingestion-cadence" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_SystemDesign_Singleton_Unchanged(t *testing.T) {
	got, err := replaceSlug(core.TypeSystemDesign, "mentat", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mentat" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_SystemDesign_Singleton_ExplicitSlugBecomesShard(t *testing.T) {
	got, err := replaceSlug(core.TypeSystemDesign, "mentat", "shard-one", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mentat.shard-one" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_ProductDesign_Singleton_Unchanged(t *testing.T) {
	got, err := replaceSlug(core.TypeProductDesign, "mentat", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "mentat" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_ProductDesign_ExplicitSlug_Error(t *testing.T) {
	_, err := replaceSlug(core.TypeProductDesign, "mentat", "shard-one", true)
	if err == nil {
		t.Fatal("expected error for --slug on product-design")
	}
}

func TestReplaceSlug_Convention(t *testing.T) {
	got, err := replaceSlug(core.TypeConvention, "convention.old-slug", "new-slug", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "convention.new-slug" {
		t.Errorf("got %q", got)
	}
}

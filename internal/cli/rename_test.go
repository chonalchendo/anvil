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
		"tags": []any{"domain/dev-tools"},
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

	// Old file gone, new file exists.
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "foo.old-slug.md")); err == nil {
		t.Error("old file still exists")
	}
	newPath := filepath.Join(vault, "70-issues", "foo.new-title.md")
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
	if r.OldID != "foo.old-slug" {
		t.Errorf("old_id = %q", r.OldID)
	}
	if r.NewID != "foo.new-title" {
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
	b, err := os.ReadFile(linkerPath)
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

func TestRename_Issue_CosmesticChange_SlugUnchanged(t *testing.T) {
	vault := setupVault(t)
	// "Old Title" and "Old   Title" both slugify to "old-title".
	writeFixtureIssue(t, vault, "foo", "old-title", "Old Title")

	cmd := newRootCmd()
	// This title slugifies identically to "old-title".
	cmd.SetArgs([]string{"rename", "issue", "foo.old-title", "--title", "Old Title Updated"})
	// "old-title-updated" != "old-title" so this will rename — let's test cosmetic properly.
	// Use a title that gives same slug: "OLD TITLE" → "old-title".
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
	vault := setupVault(t)
	_ = vault

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

func TestReplaceSlug_ProjectScoped(t *testing.T) {
	got := replaceSlug(core.TypeIssue, "myproject.old-slug", "new-slug")
	if got != "myproject.new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_Inbox(t *testing.T) {
	got := replaceSlug(core.TypeInbox, "2026-05-13-old-slug", "new-slug")
	if got != "2026-05-13-new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_Thread(t *testing.T) {
	got := replaceSlug(core.TypeThread, "old-slug", "new-slug")
	if got != "new-slug" {
		t.Errorf("got %q", got)
	}
}

func TestReplaceSlug_Decision(t *testing.T) {
	got := replaceSlug(core.TypeDecision, "mytopic.0001-old-slug", "new-slug")
	if got != "mytopic.0001-new-slug" {
		t.Errorf("got %q", got)
	}
}

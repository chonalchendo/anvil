package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeFixturePlan(t *testing.T, vault, project, slug, title string) string {
	t.Helper()
	path := filepath.Join(vault, "80-plans", project+"."+slug+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "plan", "id": project + "-" + slug, "slug": slug, "title": title,
			"description": "fixture description",
			"created":     "2026-04-29", "updated": "2026-04-29", "status": "draft",
			"plan_version": 1, "project": project,
			"issue": "[[issue." + project + "." + slug + "]]",
			"tasks": []any{map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{}, "depends_on": []any{}, "verify": "true",
			}},
		},
		Body: "## Task: T1\n\nfixture task body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLink_PlanToMilestone(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "milestone", "foo.m1-bar"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[milestone.foo.m1-bar]]" {
		t.Errorf("related = %v", related)
	}
}

func TestLink_ExternalAppendsURI(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "--external", "https://github.com/chonalchendo/anvil/pull/13"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	ext, _ := a.FrontMatter["external_links"].([]any)
	if len(ext) != 1 || ext[0] != "https://github.com/chonalchendo/anvil/pull/13" {
		t.Fatalf("external_links = %v", ext)
	}
}

func TestLink_ExternalIdempotent(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	for i := 0; i < 2; i++ {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"link", "plan", "foo.q2", "--external", "abc1234"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	ext, _ := a.FrontMatter["external_links"].([]any)
	if len(ext) != 1 {
		t.Fatalf("external_links len = %d, want 1 (idempotent): %v", len(ext), ext)
	}
}

func TestLink_ExternalRejectsTargetArgs(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "issue", "foo.x", "--external", "https://x"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error, got: %s", buf.String())
	}
}

func TestLink_ExternalRejectsReadMode(t *testing.T) {
	_ = setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "--from", "demo.a", "--external", "https://x"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLink_AnyPair_WritesToRelated(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "decision", "auth.0001-x"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[decision.auth.0001-x]]" {
		t.Errorf("related = %v", related)
	}
}

package cli

import (
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

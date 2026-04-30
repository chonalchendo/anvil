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
			"type": "plan", "title": title, "created": "2026-04-29", "updated": "2026-04-29",
			"status": "draft", "horizon": "quarter", "target_date": "2026-06-30",
			"project": project,
		},
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
	ms := a.FrontMatter["milestones"].([]any)
	if len(ms) != 1 || ms[0] != "[[milestone.foo.m1-bar]]" {
		t.Errorf("milestones = %v", ms)
	}
}

func TestLink_UnsupportedPair_Errors(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "decision", "auth.0001-x"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error")
	}
}

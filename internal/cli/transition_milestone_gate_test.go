package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

const formBody = "## Objective\n\n**Design change**\n\n- none\n\n**Components changed:** none\n"

func gateVault(t *testing.T, body string) string {
	t.Helper()
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureMilestone(t, vault, "demo.loop", "planned")
	path := filepath.Join(vault, "85-milestones", "demo.loop.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.Body = body
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")
	return vault
}

func gateTransition(t *testing.T, to string) (string, error) {
	t.Helper()
	c := newRootCmd()
	c.SetArgs([]string{"transition", "milestone", "demo.loop", to, "--json"})
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&bytes.Buffer{})
	err := c.Execute()
	return out.String(), err
}

func TestMilestoneApprovalGate(t *testing.T) {
	t.Run("bare objective refused", func(t *testing.T) {
		gateVault(t, "## Objective\n\nGoal only.\n")
		out, err := gateTransition(t, "in-progress")
		if err == nil || !strings.Contains(out, "milestone_gate_design_change") {
			t.Fatalf("want milestone_gate_design_change, err=%v out=%s", err, out)
		}
	})
	t.Run("missing components refused", func(t *testing.T) {
		gateVault(t, "## Objective\n\n**Design change**\n\n- none\n")
		out, err := gateTransition(t, "in-progress")
		if err == nil || !strings.Contains(out, "milestone_gate_components_changed") {
			t.Fatalf("want milestone_gate_components_changed, err=%v out=%s", err, out)
		}
	})
	t.Run("open routed inbox refused then dropped passes and stamps", func(t *testing.T) {
		vault := gateVault(t, formBody)
		item := &core.Artifact{
			Path: filepath.Join(vault, "00-inbox", "2026-10-07-routed.md"),
			FrontMatter: map[string]any{
				"type": "inbox", "title": "routed", "created": "2026-10-07", "status": "raw",
			},
			Body: "## Route\n\nFold into [[milestone.demo.loop]].\n",
		}
		if err := item.Save(); err != nil {
			t.Fatal(err)
		}
		out, err := gateTransition(t, "in-progress")
		if err == nil || !strings.Contains(out, "inbox_unread") || !strings.Contains(out, "2026-10-07-routed") {
			t.Fatalf("want inbox_unread naming the item, err=%v out=%s", err, out)
		}
		item.FrontMatter["status"] = "dropped"
		if err := item.Save(); err != nil {
			t.Fatal(err)
		}
		if out, err := gateTransition(t, "in-progress"); err != nil {
			t.Fatalf("approval after drop: %v out=%s", err, out)
		}
		m, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", "demo.loop.md"))
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := m.FrontMatter["approved"].(string); len(got) != 10 {
			t.Fatalf("approved stamp = %q, want a date", got)
		}
		if out, err := gateTransition(t, "planned"); err != nil {
			t.Fatalf("amend exit: %v out=%s", err, out)
		}
	})
}

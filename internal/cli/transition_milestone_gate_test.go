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

const formBody = "## Objective\n\n**Design change**\n\n- none\n\n**Components changed:** none\n"

func setMilestoneBody(t *testing.T, vault, id, body string) {
	t.Helper()
	a, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	a.Body = body
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

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
	t.Run("frontmatter-slot link counts as routed", func(t *testing.T) {
		vault := gateVault(t, formBody)
		item := &core.Artifact{
			Path: filepath.Join(vault, "00-inbox", "2026-10-07-slot.md"),
			FrontMatter: map[string]any{
				"type": "inbox", "title": "slot", "created": "2026-10-07", "status": "raw",
				"related": []any{"[[milestone.demo.loop]]"},
			},
			Body: "no body link\n",
		}
		if err := item.Save(); err != nil {
			t.Fatal(err)
		}
		out, err := gateTransition(t, "in-progress")
		if err == nil || !strings.Contains(out, "2026-10-07-slot") {
			t.Fatalf("want inbox_unread naming the slot-linked item, err=%v out=%s", err, out)
		}
	})
	t.Run("malformed inbox file refuses with milestone_scan_failed", func(t *testing.T) {
		vault := gateVault(t, formBody)
		bad := filepath.Join(vault, "00-inbox", "2026-10-07-bad.md")
		if err := os.WriteFile(bad, []byte("---\n: : not yaml [\n---\nbody\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := gateTransition(t, "in-progress")
		if err == nil || !strings.Contains(out, "milestone_scan_failed") {
			t.Fatalf("want milestone_scan_failed, err=%v out=%s", err, out)
		}
	})
}

func TestMilestoneGateRefusesProseAcceptance(t *testing.T) {
	setAcceptance := func(t *testing.T, vault string, entries ...any) {
		t.Helper()
		path := filepath.Join(vault, "85-milestones", "demo.loop.md")
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		a.FrontMatter["acceptance"] = entries
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("prose entry refused by index", func(t *testing.T) {
		vault := gateVault(t, formBody)
		repo := t.TempDir()
		prevRepo := resolveProjectRepoFn
		t.Cleanup(func() { resolveProjectRepoFn = prevRepo })
		resolveProjectRepoFn = func(string) (string, error) { return repo, nil }
		setAcceptance(t, vault, "touch ran", "The dashboard shows each issue")
		out, err := gateTransition(t, "in-progress")
		if err == nil || !strings.Contains(out, "milestone_gate_acceptance_not_command") || !strings.Contains(out, `"entries":[1]`) {
			t.Fatalf("want refusal naming only entry 1, err=%v out=%s", err, out)
		}
		if _, serr := os.Stat(filepath.Join(repo, "ran")); serr != nil {
			t.Fatalf("acceptance command did not run in the repo dir: %v", serr)
		}
		a, lerr := core.LoadArtifact(filepath.Join(vault, "85-milestones", "demo.loop.md"))
		if lerr != nil {
			t.Fatal(lerr)
		}
		if got := a.FrontMatter["status"]; got != "planned" {
			t.Fatalf("status = %v, want planned", got)
		}
		if _, ok := a.FrontMatter["approved"]; ok {
			t.Fatal("approved stamped despite refusal")
		}
	})
	t.Run("several prose entries listed highest first", func(t *testing.T) {
		vault := gateVault(t, formBody)
		prevRepo := resolveProjectRepoFn
		t.Cleanup(func() { resolveProjectRepoFn = prevRepo })
		resolveProjectRepoFn = func(string) (string, error) { return t.TempDir(), nil }
		setAcceptance(t, vault, "Alpha prose", "true", "Beta prose", "Gamma prose")
		out, err := gateTransition(t, "in-progress")
		if err == nil || !strings.Contains(out, `"entries":[3,2,0]`) || !strings.Contains(out, "highest first") {
			t.Fatalf("want entries [3,2,0] and a highest-first hint, err=%v out=%s", err, out)
		}
	})
	t.Run("unresolved repo warns", func(t *testing.T) {
		vault := gateVault(t, formBody)
		prevRepo := resolveProjectRepoFn
		t.Cleanup(func() { resolveProjectRepoFn = prevRepo })
		resolveProjectRepoFn = func(string) (string, error) { return "", errors.New("no repo") }
		setAcceptance(t, vault, "true")
		c := newRootCmd()
		c.SetArgs([]string{"transition", "milestone", "demo.loop", "in-progress", "--json"})
		var stderr bytes.Buffer
		c.SetOut(&bytes.Buffer{})
		c.SetErr(&stderr)
		_ = c.Execute()
		if !strings.Contains(stderr.String(), "project repo not resolved (no repo)") {
			t.Fatalf("want unresolved-repo warning, stderr=%s", stderr.String())
		}
	})
	t.Run("red command approved", func(t *testing.T) {
		vault := gateVault(t, formBody)
		setAcceptance(t, vault, "false")
		if out, err := gateTransition(t, "in-progress"); err != nil {
			t.Fatalf("red command must approve: %v out=%s", err, out)
		}
	})
}

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestSetRefusesApprovedMilestoneScope(t *testing.T) {
	root := setupVault(t)
	initVaultRepo(t, root)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	path := createIssueGetPath(t, "create", "milestone", "--project", "foo", "--title", "Loop",
		"--description", "fixture", "--goal", "demo passes", "--acceptance", "true")
	setState := func(status string) {
		t.Helper()
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		a.FrontMatter["approved"] = "2026-10-10"
		a.FrontMatter["status"] = status
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	field := func(k string) any {
		t.Helper()
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		return a.FrontMatter[k]
	}
	set := func(args ...string) (string, error) {
		out, _, err := runCmd(t, newRootCmd(), append([]string{"set", "milestone", "foo.loop"}, append(args, "--json")...)...)
		return out, err
	}
	cases := map[string][]string{
		"goal":           {"goal", "demo passes twice"},
		"acceptance add": {"acceptance", "--add", "false"},
		"acceptance rm":  {"acceptance", "--remove", "0"},
		"acceptance set": {"acceptance", "false"},
	}

	setState("in-progress")
	for name, args := range cases {
		out, err := set(args...)
		if err == nil {
			t.Fatalf("%s on approved milestone: want refusal, got %s", name, out)
		}
		if !strings.Contains(out, "update_approved_milestone_scope") || !strings.Contains(out, "transition milestone") {
			t.Errorf("%s refusal envelope lacks code or fix: %s", name, out)
		}
	}
	if field("goal") != "demo passes" || fmt.Sprint(field("acceptance")) != "[true]" {
		t.Errorf("refused set wrote: goal=%v acceptance=%v", field("goal"), field("acceptance"))
	}
	if out, err := set("description", "rewritten"); err != nil {
		t.Fatalf("description set on approved milestone: %v\n%s", err, out)
	}

	for name, args := range map[string][]string{
		"approved value": {"approved", "2026-01-01"},
		"approved unset": {"approved", ""},
	} {
		out, err := set(args...)
		if err == nil || !strings.Contains(out, "approved_via_set") || !strings.Contains(out, "transition milestone milestone.foo.loop planned") {
			t.Errorf("%s: want approved_via_set refusal, got err=%v %s", name, err, out)
		}
	}
	if field("approved") != "2026-10-10" {
		t.Errorf("refused set changed approved: %v", field("approved"))
	}

	setState("done")
	if out, err := set("approved", ""); err == nil || !strings.Contains(out, `planned --reason`) {
		t.Errorf("done approved set: want --reason in hint, got err=%v %s", err, out)
	}

	setState("planned")
	if out, err := set("goal", "demo passes twice"); err != nil {
		t.Fatalf("goal set on planned milestone: %v\n%s", err, out)
	}
	if out, err := set("acceptance", "--add", "false"); err != nil {
		t.Fatalf("acceptance add on planned milestone: %v\n%s", err, out)
	}
	if field("goal") != "demo passes twice" || fmt.Sprint(field("acceptance")) != "[true false]" {
		t.Errorf("planned set: goal=%v acceptance=%v", field("goal"), field("acceptance"))
	}
}

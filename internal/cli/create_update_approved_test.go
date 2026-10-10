package cli

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestCreate_UpdateApprovedMilestone_RefusesScopeChange(t *testing.T) {
	root := setupVault(t)
	initVaultRepo(t, root)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	base := func(goal, acceptance, desc string) []string {
		return []string{
			"create", "milestone", "--project", "foo", "--title", "Loop",
			"--description", desc, "--goal", goal, "--acceptance", acceptance,
		}
	}
	path := createIssueGetPath(t, base("demo passes", "true", "fixture")...)

	setApproved := func(on bool) {
		t.Helper()
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		if on {
			a.FrontMatter["approved"] = "2026-10-10"
			a.FrontMatter["status"] = "in-progress"
		} else {
			delete(a.FrontMatter, "approved")
		}
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
	// A goal-only change reports already_exists (anvil drift check), so each
	// case also changes the description to reach the update path.
	update := func(goal, acceptance, desc string) (string, error) {
		stdout, _, err := runCmd(t, newRootCmd(), append(base(goal, acceptance, desc), "--update", "--json")...)
		return stdout, err
	}

	setApproved(true)
	commits := func() string { return strings.TrimSpace(vaultGit(t, root, "rev-list", "--count", "HEAD")) }
	beforeCommits := commits()
	readBytes := func() string {
		t.Helper()
		b, err := os.ReadFile(path) //nolint:gosec // G304: test-controlled temp path
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	beforeBytes := readBytes()
	for name, args := range map[string][3]string{
		"goal":       {"demo passes twice", "true", "fixture edited"},
		"acceptance": {"demo passes", "false", "fixture edited"},
	} {
		out, err := update(args[0], args[1], args[2])
		if err == nil {
			t.Fatalf("%s change on approved milestone: want refusal, got success: %s", name, out)
		}
		if !strings.Contains(out, "update_approved_milestone_scope") || !strings.Contains(out, "transition milestone") || !strings.Contains(out, "planned") || strings.Contains(out, "schema_invalid") {
			t.Errorf("%s refusal envelope lacks code or fix: %s", name, out)
		}
		if got := commits(); got != beforeCommits {
			t.Errorf("%s refusal committed a snapshot: %s -> %s", name, beforeCommits, got)
		}
		if readBytes() != beforeBytes {
			t.Errorf("%s refusal changed file bytes", name)
		}
	}
	if field("goal") != "demo passes" || fmt.Sprint(field("acceptance")) != "[true]" {
		t.Errorf("refused update wrote: goal=%v acceptance=%v", field("goal"), field("acceptance"))
	}

	if field("description") != "fixture" {
		t.Errorf("refused update wrote description = %v", field("description"))
	}

	// Non-scope fields still update on an approved milestone.
	if out, err := update("demo passes", "true", "fixture rewritten"); err != nil {
		t.Fatalf("description update on approved milestone: %v\n%s", err, out)
	}
	if field("description") != "fixture rewritten" {
		t.Errorf("description = %v, want updated", field("description"))
	}

	// Without approved, goal still updates.
	setApproved(false)
	if out, err := update("demo passes twice", "true", "fixture rewritten again"); err != nil {
		t.Fatalf("goal update on planned milestone: %v\n%s", err, out)
	}
	if field("goal") != "demo passes twice" {
		t.Errorf("goal = %v, want updated", field("goal"))
	}

	// A done -> planned reverse move keeps `approved`; the milestone must still update.
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.FrontMatter["approved"] = "2026-10-10"
	a.FrontMatter["status"] = "planned"
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	if out, err := update("demo passes thrice", "true", "fixture rewritten thrice"); err != nil {
		t.Fatalf("goal update on approved planned milestone: %v\n%s", err, out)
	}
	if field("goal") != "demo passes thrice" {
		t.Errorf("goal = %v, want updated", field("goal"))
	}

	// Done and abandoned milestones keep `approved`; the hint depends on the move to planned.
	for _, tc := range []struct {
		status string
		want   []string
		not    string
	}{
		{"done", []string{"planned --reason", "in-progress"}, ""},
		{"abandoned", []string{"has no move to planned", "create a new milestone"}, "transition milestone"},
	} {
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		a.FrontMatter["approved"] = "2026-10-10"
		a.FrontMatter["status"] = tc.status
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
		out, err := update("demo passes four", "true", "fixture "+tc.status)
		if err == nil {
			t.Fatalf("%s: want refusal, got success: %s", tc.status, out)
		}
		for _, w := range tc.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s refusal lacks %q: %s", tc.status, w, out)
			}
		}
		if tc.not != "" && strings.Contains(out, tc.not) {
			t.Errorf("%s refusal must not contain %q: %s", tc.status, tc.not, out)
		}
	}
}

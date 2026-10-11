package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateUpdateAppliesEveryFlag(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	args := func(goal, acc string, extra ...string) []string {
		return append([]string{
			"create", "milestone", "--project", "foo", "--title", "Loop",
			"--description", "fixture", "--goal", goal, "--acceptance", acc,
		}, extra...)
	}
	createIssueGetPath(t, args("demo passes", "true")...)

	if _, _, err := runCmd(t, newRootCmd(), args("demo passes twice", "false")...); err == nil {
		t.Fatal("goal change without --update: want drift error")
	}

	out, _, err := runCmd(t, newRootCmd(), args("demo passes twice", "false", "--update", "--json")...)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Status  string   `json:"status"`
		Changed []string `json:"changed"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &res); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if res.Status != "updated" || strings.Join(res.Changed, ",") != "acceptance,goal" {
		t.Fatalf("got status=%q changed=%v", res.Status, res.Changed)
	}
	out, _, err = runCmd(t, newRootCmd(), args("demo passes twice", "false", "--update", "--json")...)
	if err != nil || !strings.Contains(out, `"already_exists"`) {
		t.Fatalf("unchanged re-run: err=%v out=%s", err, out)
	}
}

func TestCreateDriftOnlyPassedFlags(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Chdir(repo)

	ms := func(title string, acc ...string) []string {
		a := []string{"create", "milestone", "--project", "foo", "--title", title, "--description", "fixture", "--goal", "g"}
		for _, c := range acc {
			a = append(a, "--acceptance", c)
		}
		return a
	}
	createIssueGetPath(t, ms("Loop", "true")...)

	for _, extra := range [][]string{nil, {"--update"}} {
		out, _, err := runCmd(t, newRootCmd(), append([]string{"create", "milestone", "--project", "foo", "--title", "Loop", "--description", "fixture", "--goal", "g", "--json"}, extra...)...)
		if err != nil || !strings.Contains(out, `"already_exists"`) {
			t.Fatalf("milestone re-run without --acceptance %v: err=%v out=%s", extra, err, out)
		}
	}

	iss := []string{"create", "issue", "--project", "foo", "--title", "Sev", "--description", "fixture", "--goal", "g", "--tags", "domain/dev-tools", "--allow-new-facet=domain", "--acceptance", "true"}
	createIssueGetPath(t, append(iss, "--severity", "high")...)
	for _, extra := range [][]string{nil, {"--update"}} {
		out, _, err := runCmd(t, newRootCmd(), append(append([]string{}, iss...), append([]string{"--json"}, extra...)...)...)
		if err != nil || !strings.Contains(out, `"already_exists"`) {
			t.Fatalf("issue re-run without --severity %v: err=%v out=%s", extra, err, out)
		}
	}

	// Empty flags are not caller intent: no drift, no rewrite, severity kept.
	for _, extra := range [][]string{nil, {"--update"}} {
		for _, args := range [][]string{
			append(append([]string{}, ms("Loop", "true")...), "--goal", ""),
			append(append([]string{}, iss...), "--severity", ""),
		} {
			out, _, err := runCmd(t, newRootCmd(), append(append([]string{}, args...), append([]string{"--json"}, extra...)...)...)
			if err != nil || !strings.Contains(out, `"already_exists"`) {
				t.Fatalf("empty-flag re-run %v: err=%v out=%s", extra, err, out)
			}
		}
	}

	if _, _, err := runCmd(t, newRootCmd(), ms("Split", "a b")...); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCmd(t, newRootCmd(), ms("Split", "a", "b")...); err == nil {
		t.Fatal(`["a b"] vs ["a","b"]: want drift error`)
	}
}

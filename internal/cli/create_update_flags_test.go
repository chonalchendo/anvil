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

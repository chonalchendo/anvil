package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// replayFixture builds a repo with base and merge commits, a resolved issue
// linking PR 7, and stubs for the project repo and gh.
func replayFixture(t *testing.T, status string, links []any) (vault, repo, base, id string) {
	t.Helper()
	vault = setupVault(t)
	repo = t.TempDir()
	gitIn(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-qm", "base")
	base = gitIn(t, repo, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-qm", "merge")
	merge := gitIn(t, repo, "rev-parse", "HEAD")
	gitIn(t, repo, "update-ref", "refs/remotes/origin/HEAD", merge)

	prevRepo, prevView, prevFetch := resolveProjectRepoFn, ghPRViewJSONFn, gitFetchOriginFn
	t.Cleanup(func() { resolveProjectRepoFn, ghPRViewJSONFn, gitFetchOriginFn = prevRepo, prevView, prevFetch })
	resolveProjectRepoFn = func(string) (string, error) { return repo, nil }
	gitFetchOriginFn = func(string) error { return nil }
	ghPRViewJSONFn = func(int, string) ([]byte, error) {
		return []byte(`{"state":"MERGED","mergeCommit":{"oid":"` + merge + `"}}`), nil
	}

	id = "issue.anvil.0001.ok"
	fm := map[string]any{
		"type": "issue", "title": id, "description": "fixture", "created": "2026-05-15", "updated": "2026-05-15",
		"status": status, "project": "anvil", "severity": "medium", "tags": []any{"domain/cli"},
		"verified_verdict": "pass", "verified_commit": "abc", "outcome_first_verdict": "pass",
		"cost_rounds": 1, "cost_diff": 40, "cost_files": 3, "cost_tokens": 900,
	}
	if links != nil {
		fm["external_links"] = links
	}
	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", id+".md"), FrontMatter: fm,
		Body: "## Verification\n\n### Direct\n\n```bash\ntest -f a.txt\n```\n\n### Indirect\n\n```bash\ntest ! -f b.txt\n```\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return vault, repo, base, id
}

func TestReplayCutsWorktreeAtMergeParent(t *testing.T) {
	vault, _, base, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	before, _ := os.ReadFile( //nolint:gosec // test path
		filepath.Join(vault, "70-issues", id+".md"))
	wt := filepath.Join(t.TempDir(), "wt")
	out, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != wt {
		t.Errorf("stdout = %q, want %q", out, wt)
	}
	if got := gitIn(t, wt, "rev-parse", "HEAD"); got != base {
		t.Errorf("worktree HEAD = %s, want merge parent %s", got, base)
	}
	if got := gitIn(t, wt, "branch", "--show-current"); got != "replay/0001.ok" {
		t.Errorf("branch = %q", got)
	}
	after, _ := os.ReadFile( //nolint:gosec // test path
		filepath.Join(vault, "70-issues", id+".md"))
	if string(before) != string(after) {
		t.Error("replay changed the issue file")
	}
}

func TestReplayRefusals(t *testing.T) {
	cases := []struct {
		name, status, want string
		links              []any
	}{
		{"unresolved", "in-progress", "replay_not_resolved", []any{"https://github.com/o/r/pull/7"}},
		{"no PR", "resolved", "replay_no_merged_pr", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, id := replayFixture(t, c.status, c.links)
			wt := filepath.Join(t.TempDir(), "wt")
			out, stderr, err := runCmd(t, newReplayCmd(), id, "--worktree", wt)
			if err == nil || !strings.Contains(out+stderr+err.Error(), c.want) {
				t.Errorf("err = %v, out = %q, stderr = %q; want %s", err, out, stderr, c.want)
			}
			if _, serr := os.Stat(wt); serr == nil {
				t.Error("refusal cut a worktree")
			}
		})
	}
}

func TestReplayRefusesUnmergedPR(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	ghPRViewJSONFn = func(int, string) ([]byte, error) { return []byte(`{"state":"OPEN","mergeCommit":null}`), nil }
	out, stderr, err := runCmd(t, newReplayCmd(), id, "--worktree", filepath.Join(t.TempDir(), "wt"))
	if err == nil || !strings.Contains(out+stderr+err.Error(), "replay_no_merged_pr") {
		t.Errorf("err = %v, out = %q", err, out)
	}
}

func TestReplayVerifyAppendsSectionAndKeepsLandedRecord(t *testing.T) {
	vault, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	wt := filepath.Join(t.TempDir(), "wt")
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "c.txt"), []byte("1\n2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, wt, "add", ".")
	t.Chdir(wt)
	path := filepath.Join(vault, "70-issues", id+".md")
	before, _ := core.LoadArtifact(path)
	if _, _, err := runCmd(t, newVerifyCmd(), id, "--replay", "--tokens", "123", "--json"); err != nil {
		t.Fatal(err)
	}
	after, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"verified_verdict", "verified_commit", "verified_at", "outcome_first_verdict"} {
		if before.FrontMatter[k] != after.FrontMatter[k] {
			t.Errorf("%s changed: %v -> %v", k, before.FrontMatter[k], after.FrontMatter[k])
		}
	}
	i := strings.Index(after.Body, "## Replay — ")
	if i < 0 {
		t.Fatalf("no Replay section:\n%s", after.Body)
	}
	sec := after.Body[i:]
	for _, want := range []string{"verdict: pass (2 checks)", "replay: diff 2, files 1, tokens 123", "landed: diff 40, files 3, tokens 900"} {
		if !strings.Contains(sec, want) {
			t.Errorf("section missing %q:\n%s", want, sec)
		}
	}
}

func TestReplayVerifyFlagPairing(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", nil)
	for _, args := range [][]string{{"--replay"}, {"--tokens", "5"}} {
		_, stderr, err := runCmd(t, newVerifyCmd(), append([]string{id}, args...)...)
		if err == nil || !strings.Contains(stderr+err.Error(), "verify_replay_flags") {
			t.Errorf("%v: err = %v, stderr = %q", args, err, stderr)
		}
	}
}

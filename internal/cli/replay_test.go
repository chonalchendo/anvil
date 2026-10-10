package cli

import (
	"encoding/json"
	"errors"
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

	prevRepo, prevView, prevFetch := resolveProjectRepoFn, ghPRViewByURLFn, gitFetchOriginFn
	t.Cleanup(func() { resolveProjectRepoFn, ghPRViewByURLFn, gitFetchOriginFn = prevRepo, prevView, prevFetch })
	resolveProjectRepoFn = func(string) (string, error) { return repo, nil }
	gitFetchOriginFn = func(string) error { return nil }
	ghPRViewByURLFn = func(string, string) ([]byte, error) {
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
	if want, _ := filepath.EvalSymlinks(wt); strings.TrimSpace(out) != want {
		t.Errorf("stdout = %q, want %q", out, want)
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
	ghPRViewByURLFn = func(string, string) ([]byte, error) { return []byte(`{"state":"OPEN","mergeCommit":null}`), nil }
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
	gitIn(t, wt, "commit", "-qm", "work")
	commit := gitIn(t, wt, "rev-parse", "HEAD")
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
	for _, want := range []string{"verdict: pass (2 checks)", "replay: diff 2, files 1, tokens 123", "landed: diff 40, files 3, tokens 900", "commit: " + commit} {
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

func TestReplayRecutsStaleBranchAtBase(t *testing.T) {
	_, repo, base, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	wt := filepath.Join(t.TempDir(), "wt")
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "c.txt"), []byte("c\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, wt, "add", ".")
	gitIn(t, wt, "commit", "-qm", "work")
	gitIn(t, repo, "worktree", "remove", "--force", wt)
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt); err != nil {
		t.Fatalf("second replay: %v", err)
	}
	if got := gitIn(t, wt, "rev-parse", "HEAD"); got != base {
		t.Errorf("HEAD = %s, want %s", got, base)
	}
}

func TestReplayRefusesLiveWorktreeAndMissingGh(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	wt := filepath.Join(t.TempDir(), "wt")
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runCmd(t, newReplayCmd(), id, "--worktree", filepath.Join(t.TempDir(), "wt2"))
	got := out + stderr + errString(err)
	for _, want := range []string{"replay_worktree_exists", "--remove", "message"} {
		if !strings.Contains(got, want) {
			t.Errorf("live worktree: missing %q in %q", want, got)
		}
	}
	_, _, _, id2 := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	ghPRViewByURLFn = func(string, string) ([]byte, error) { return nil, errGhUnavailable }
	out, stderr, err = runCmd(t, newReplayCmd(), id2, "--worktree", filepath.Join(t.TempDir(), "wt3"))
	got = out + stderr + errString(err)
	for _, want := range []string{"replay_gh_unavailable", "fix_hint"} {
		if !strings.Contains(got, want) {
			t.Errorf("gh missing: missing %q in %q", want, got)
		}
	}
}

func TestReplayPassesFullPRURLToGh(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	var seen string
	inner := ghPRViewByURLFn
	ghPRViewByURLFn = func(u, f string) ([]byte, error) { seen = u; return inner(u, f) }
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", filepath.Join(t.TempDir(), "wt")); err != nil {
		t.Fatal(err)
	}
	if seen != "https://github.com/o/r/pull/7" {
		t.Errorf("gh saw %q", seen)
	}
}

func TestReplayVerifyRefusesOutsideReplayWorktree(t *testing.T) {
	_, repo, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	t.Chdir(repo)
	out, stderr, err := runCmd(t, newVerifyCmd(), id, "--replay", "--tokens", "5", "--json")
	got := out + stderr + errString(err)
	for _, want := range []string{"replay_not_replay_worktree", "anvil replay " + id} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("--json printed no envelope: %q", out)
	}
}

func TestReplayVerifyHonoursLockAndRefusesAcceptAndAt(t *testing.T) {
	vault, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	path := filepath.Join(vault, "70-issues", id+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.FrontMatter["verification_lock"] = "stale-lock"
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt); err != nil {
		t.Fatal(err)
	}
	t.Chdir(wt)
	_, stderr, err := runCmd(t, newVerifyCmd(), id, "--replay", "--tokens", "5")
	if err == nil || !strings.Contains(stderr+err.Error(), "verification_changed") {
		t.Errorf("lock: err = %v, stderr = %q", err, stderr)
	}
	if strings.Contains(stderr+err.Error(), "--accept-change") || !strings.Contains(stderr+err.Error(), "restore the ## Verification section") {
		t.Errorf("replay lock hint must not offer --accept-change: %q", stderr+err.Error())
	}
	for _, extra := range [][]string{{"--accept-change"}, {"--at", "HEAD"}} {
		_, stderr, err := runCmd(t, newVerifyCmd(), append([]string{id, "--replay", "--tokens", "5"}, extra...)...)
		if err == nil || !strings.Contains(stderr+err.Error(), "verify_replay_flags") {
			t.Errorf("%v: err = %v, stderr = %q", extra, err, stderr)
		}
	}
	after, _ := core.LoadArtifact(path)
	if strings.Contains(after.Body, "## Replay") {
		t.Error("a refused replay wrote a section")
	}
}

func TestReplayVerifyNotesDirtyTree(t *testing.T) {
	vault, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	wt := filepath.Join(t.TempDir(), "wt")
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "d.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(wt)
	if _, _, err := runCmd(t, newVerifyCmd(), id, "--replay", "--tokens", "5"); err != nil {
		t.Fatal(err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", id+".md"))
	if !strings.Contains(a.Body, "- dirty:") || !strings.Contains(a.Body, "replay: diff 0, files 0") {
		t.Errorf("dirty tree not noted or counted:\n%s", a.Body)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestReplayVersionSha7Pattern(t *testing.T) {
	for in, want := range map[string]string{"dev-23b326f-dirty": "23b326f-dirty", "dev-23b326f": "23b326f", "dev-571dba3e-dirty": "571dba3-dirty", "v0.0.0-20240101000000-abcdef123456": ""} {
		got := ""
		if m := versionSha7.FindStringSubmatch(in); m != nil {
			got = m[1] + m[2]
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}
}

func TestReplayJSONAndRerunHints(t *testing.T) {
	_, _, base, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	wt := filepath.Join(t.TempDir(), "wt")
	out, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	want, _ := filepath.EvalSymlinks(wt)
	if err := json.Unmarshal([]byte(out), &got); err != nil || got["worktree"] != want || got["base"] != base {
		t.Errorf("json = %q (%v), want worktree %s base %s", out, err, want, base)
	}
	ghPRViewByURLFn = func(string, string) ([]byte, error) { return nil, errGhUnavailable }
	_, stderr, err := runCmd(t, newReplayCmd(), id, "--worktree", filepath.Join(t.TempDir(), "w2"))
	if h := stderr + errString(err); !strings.Contains(h, "re-run anvil replay "+id) {
		t.Errorf("replay hint: %q", h)
	}
	t.Chdir(wt)
	_, stderr, err = runCmd(t, newVerifyCmd(), id, "--replay", "--tokens", "5")
	if h := stderr + errString(err); !strings.Contains(h, "re-run anvil verify "+id+" --replay --tokens 5") {
		t.Errorf("verify hint: %q", h)
	}
}

func TestReplaySectionFailedLineOmitsPredicateText(t *testing.T) {
	exit := 3
	sec := replaySection(verifyRecord{
		Commit: "abc", Verdict: "fail", Checks: 1,
		Failed: []verifyFailure{{Check: "Direct#1", Exit: &exit, Preview: "SECRET-predicate"}},
	}, 0, 0, 5, nil)
	if !strings.Contains(sec, "- failed: Direct#1 (exit 3)") || strings.Contains(sec, "SECRET") {
		t.Errorf("section = %s", sec)
	}
}

func TestReplayRemoveDeletesWorktreeAndBranch(t *testing.T) {
	_, repo, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	wt := filepath.Join(t.TempDir(), "wt")
	cut, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt)
	if err != nil {
		t.Fatal(err)
	}
	resolved := strings.TrimSpace(cut)
	if err := os.WriteFile(filepath.Join(wt, "c.txt"), []byte("c\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, wt, "add", ".")
	gitIn(t, wt, "commit", "-qm", "replay work")
	out, _, err := runCmd(t, newReplayCmd(), id, "--remove", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil || got["worktree"] != resolved || got["branch"] == "" {
		t.Errorf("json = %q (%v), want worktree %s", out, err, resolved)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree still exists: %v", err)
	}
	if gitIn(t, repo, "branch", "--list", "replay/*") != "" {
		t.Errorf("replay branch left behind")
	}
}

func TestReplayRemoveRemovesBranchWithoutWorktree(t *testing.T) {
	_, repo, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	wt := filepath.Join(t.TempDir(), "wt")
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", wt); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "worktree", "remove", "--force", wt)
	if _, _, err := runCmd(t, newReplayCmd(), id, "--remove"); err != nil {
		t.Fatal(err)
	}
	if gitIn(t, repo, "branch", "--list", "replay/*") != "" {
		t.Errorf("replay branch left behind")
	}
}

func TestReplayRemoveWithNothingIsIdempotent(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	for i := 0; i < 2; i++ {
		out, _, err := runCmd(t, newReplayCmd(), id, "--remove", "--json")
		if err != nil || strings.TrimSpace(out) != `{"branch":"","worktree":""}` {
			t.Errorf("run %d: out=%q err=%v", i, out, err)
		}
	}
}

func TestReplayRemoveRejectsWorktreeFlag(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	out, _, err := runCmd(t, newReplayCmd(), id, "--remove", "--worktree", "/tmp/x", "--json")
	if err == nil || !strings.Contains(out+errString(err), "replay_remove_flags") {
		t.Errorf("want replay_remove_flags, got %q (%v)", out, err)
	}
}

func TestReplayRefusalNeedsNoNetwork(t *testing.T) {
	_, _, _, id := replayFixture(t, "open", []any{"https://github.com/o/r/pull/7"})
	gitFetchOriginFn = func(string) error { t.Error("fetch ran before the status refusal"); return nil }
	if _, _, err := runCmd(t, newReplayCmd(), id, "--worktree", filepath.Join(t.TempDir(), "wt")); err == nil {
		t.Error("want a not-resolved refusal")
	}
}

func TestReplayUnparseableGhOutputIsGhFailed(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	ghPRViewByURLFn = func(string, string) ([]byte, error) { return []byte("not json"), nil }
	out, stderr, err := runCmd(t, newReplayCmd(), id, "--worktree", filepath.Join(t.TempDir(), "wt"))
	got := out + stderr + errString(err)
	if !strings.Contains(got, "replay_gh_failed") || !strings.Contains(got, "https://github.com/o/r/pull/7") || strings.Contains(got, "replay_no_merged_pr") {
		t.Errorf("got %q", got)
	}
}

func TestReplayGhFailureCarriesStderrAndURL(t *testing.T) {
	_, _, _, id := replayFixture(t, "resolved", []any{"https://github.com/o/r/pull/7"})
	ghPRViewByURLFn = func(string, string) ([]byte, error) { return nil, errors.New("exit status 1: HTTP 404") }
	out, stderr, err := runCmd(t, newReplayCmd(), id, "--worktree", filepath.Join(t.TempDir(), "wt"))
	got := out + stderr + errString(err)
	for _, want := range []string{"replay_gh_failed", "HTTP 404", "https://github.com/o/r/pull/7", "check gh auth status and the url in external_links"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "replay_gh_unavailable") {
		t.Errorf("gh failure reported as unavailable: %q", got)
	}
}

func TestGhPRViewByURLRealCarriesStderr(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'HTTP 404: not found' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o700); err != nil { //nolint:gosec // test stub must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := ghPRViewByURLReal("https://github.com/o/r/pull/7", "state")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404: not found") {
		t.Errorf("err = %v, want gh stderr carried", err)
	}
}

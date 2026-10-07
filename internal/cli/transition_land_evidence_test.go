package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

const (
	landHeadFields = "headRefOid,headRefName"
	landTestHead   = "0123456789abcdef0123456789abcdef01234567"
)

func passingEvidence() landEvidence {
	return landEvidence{
		id: "issue.demo.foo", lock: "L", currentLock: "L", branch: "demo/foo",
	}
}

func headJSON(oid, ref string) []byte {
	return []byte(`{"headRefOid":"` + oid + `","headRefName":"` + ref + `"}`)
}

func TestLandEvidenceCheck(t *testing.T) {
	cases := []struct {
		name string
		ev   func(*landEvidence)
		head []byte
		want string
	}{
		{"passes", func(*landEvidence) {}, headJSON(landTestHead, "demo/foo"), ""},
		{"changed lock", func(e *landEvidence) { e.currentLock = "other" }, headJSON(landTestHead, "demo/foo"), "verification_changed"},
		{"no lock is intact", func(e *landEvidence) { e.lock = ""; e.currentLock = "other" }, headJSON(landTestHead, "demo/foo"), ""},
		{"foreign pr", func(*landEvidence) {}, headJSON(landTestHead, "demo/sibling"), "land_pr_not_issue_pr"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := stubSideFX(t)
			s.viewByField[landHeadFields] = c.head
			ev := passingEvidence()
			c.ev(&ev)
			_, err := ev.check(42, "", false)
			if c.want == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %s", err, c.want)
			}
		})
	}
}

func TestLandEvidenceViewFailureRefuses(t *testing.T) {
	s := stubSideFX(t)
	s.viewByFieldE[landHeadFields] = errors.New("boom")
	if _, err := passingEvidence().check(42, "", false); err == nil || !strings.Contains(err.Error(), "land_pr_view_failed") {
		t.Fatalf("err = %v", err)
	}
}

// A renamed branch is the issue's own when it is checked out at the issue's
// worktree path; any other path is foreign.
func TestLandEvidenceRenamedBranch(t *testing.T) {
	wt := t.TempDir()
	for _, c := range []struct {
		name, listed, want string
	}{
		{"own worktree", wt, ""},
		{"other worktree", t.TempDir(), "land_pr_not_issue_pr"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := stubSideFX(t)
			s.viewByField[landHeadFields] = headJSON(landTestHead, "anvil/renamed")
			s.listEntries["anvil/renamed"] = worktreeInfo{path: c.listed}
			head, err := passingEvidence().check(42, wt, false)
			if c.want == "" {
				if err != nil || head.branch != "anvil/renamed" {
					t.Fatalf("head=%+v err=%v", head, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %s", err, c.want)
			}
		})
	}
}

// An already-MERGED PR skips the lock and ownership checks: refusing after
// the merge would strand the issue in-progress.
func TestLandEvidenceSkippedWhenAlreadyMerged(t *testing.T) {
	s := stubSideFX(t)
	s.viewByField[landHeadFields] = headJSON("other-head", "demo/sibling")
	ev := passingEvidence()
	ev.currentLock = "other"
	head, err := ev.check(42, "", true)
	if err != nil || head.branch != "demo/sibling" {
		t.Fatalf("head=%+v err=%v", head, err)
	}
}

const landCleanBody = "## Problem\n\nfixture.\n\n## Non-goals\n\n- none\n\n## Verification\n\n### Direct\n\n```bash\n%s\n```\n\n### Indirect\n\n```bash\ntrue\n```\n\n## Links\n\n- none\n"

// landCleanFixture claims an issue whose Direct block is direct, builds a temp
// repo as the main root, and points the PR head at its commit. The real clean
// run, fetch-stub and real checkout removal are wired, so the checkout leaves
// no worktree registered in the temp repo.
func landCleanFixture(t *testing.T, direct string) (s *sideFXStub, repo, sha, wt string) {
	t.Helper()
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	path := filepath.Join(vault, "70-issues", "demo.foo.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.Body = strings.Replace(landCleanBody, "%s", direct, 1)
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	repo = t.TempDir()
	gitIn(t, repo, "init", "-q")
	gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	sha = gitIn(t, repo, "rev-parse", "HEAD")

	s = stubSideFX(t)
	s.mainRoot = repo
	landCleanRunFn = landCleanRun
	gitWorktreeRemoveForceFn = gitWorktreeRemoveForceReal
	s.viewByField[landHeadFields] = headJSON(sha, "demo/foo")
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	wt = t.TempDir()
	return s, repo, sha, wt
}

func landCleanRecord(t *testing.T) (verdict, commit string) {
	t.Helper()
	a, err := core.LoadArtifact(filepath.Join(os.Getenv("ANVIL_VAULT"), "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	verdict, _ = a.FrontMatter["verified_verdict"].(string)
	commit, _ = a.FrontMatter["verified_commit"].(string)
	return verdict, commit
}

func landCleanArgs(wt string, extra ...string) []string {
	return append([]string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--worktree", wt, "--json"}, extra...)
}

func worktreeCount(t *testing.T, repo string) int {
	return strings.Count(gitIn(t, repo, "worktree", "list"), "\n")
}

func TestLandPRCleanRedRunRefusesWithoutMerge(t *testing.T) {
	s, repo, sha, wt := landCleanFixture(t, "false")
	n0 := worktreeCount(t, repo)
	out, _, err := runCmd(t, newRootCmd(), landCleanArgs(wt)...)
	if err == nil || !strings.Contains(out, "land_pr_verification_failed") || !strings.Contains(out, "Direct#1") {
		t.Fatalf("err=%v out=%s", err, out)
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge calls = %v, want none", s.mergeCalls)
	}
	if v, c := landCleanRecord(t); v != "fail" || c != sha {
		t.Errorf("record = %s at %s, want fail at %s", v, c, sha)
	}
	if s.fetchCalls != 1 {
		t.Errorf("fetch calls = %d, want 1", s.fetchCalls)
	}
	if n := worktreeCount(t, repo); n != n0 {
		t.Errorf("worktree count %d, want %d", n, n0)
	}
}

func TestLandPRCleanPassMergesOnceAndStampsHead(t *testing.T) {
	s, repo, sha, wt := landCleanFixture(t, "true")
	n0 := worktreeCount(t, repo)
	if out, _, err := runCmd(t, newRootCmd(), landCleanArgs(wt)...); err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	if len(s.mergeCalls) != 1 {
		t.Errorf("merge calls = %v, want one", s.mergeCalls)
	}
	if v, c := landCleanRecord(t); v != "pass" || c != sha {
		t.Errorf("record = %s at %s, want pass at %s", v, c, sha)
	}
	if n := worktreeCount(t, repo); n != n0 {
		t.Errorf("worktree count %d, want %d", n, n0)
	}
}

// An old record on the issue neither blocks nor satisfies the land.
func TestLandPRCleanIgnoresAnOldRecord(t *testing.T) {
	s, _, sha, wt := landCleanFixture(t, "true")
	path := filepath.Join(os.Getenv("ANVIL_VAULT"), "70-issues", "demo.foo.md")
	a, _ := core.LoadArtifact(path)
	a.FrontMatter["verified_verdict"] = "fail"
	a.FrontMatter["verified_commit"] = "deadbeef-dirty"
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	out, _, err := runCmd(t, newRootCmd(), landCleanArgs(wt)...)
	if err != nil || strings.Contains(out, "verification_stale") || len(s.mergeCalls) != 1 {
		t.Fatalf("err=%v merges=%v out=%s", err, s.mergeCalls, out)
	}
	if v, c := landCleanRecord(t); v != "pass" || c != sha {
		t.Errorf("record = %s at %s, want pass at %s", v, c, sha)
	}
}

func TestLandPRCleanRunsUnderLocalValidated(t *testing.T) {
	s, _, _, wt := landCleanFixture(t, "false")
	out, _, err := runCmd(t, newRootCmd(), landCleanArgs(wt, "--local-validated")...)
	if err == nil || !strings.Contains(out, "land_pr_verification_failed") || len(s.mergeCalls) != 0 {
		t.Fatalf("err=%v merges=%v out=%s", err, s.mergeCalls, out)
	}
}

func TestLandPRCleanSkippedWhenAlreadyMerged(t *testing.T) {
	s, _, _, wt := landCleanFixture(t, "false")
	s.viewSeq["state"] = nil
	if out, _, err := runCmd(t, newRootCmd(), landCleanArgs(wt)...); err != nil {
		t.Fatalf("err=%v out=%s", err, out)
	}
	if len(s.mergeCalls) != 0 || s.fetchCalls != 0 {
		t.Errorf("merges=%v fetches=%d, want none", s.mergeCalls, s.fetchCalls)
	}
	if v, _ := landCleanRecord(t); v != "" {
		t.Errorf("verdict = %q, want no run", v)
	}
}

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
		id: "issue.demo.foo", verdict: "pass", commit: landTestHead,
		lock: "L", currentLock: "L", branch: "demo/foo",
	}
}

// stampLandEvidence writes a verification record onto an already-claimed
// fixture issue.
func stampLandEvidence(t *testing.T, id, verdict, commit string) {
	t.Helper()
	path := filepath.Join(os.Getenv("ANVIL_VAULT"), "70-issues", id+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.FrontMatter["verified_verdict"] = verdict
	a.FrontMatter["verified_commit"] = commit
	if err := a.Save(); err != nil {
		t.Fatal(err)
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
		{"failed verdict", func(e *landEvidence) { e.verdict = "fail" }, headJSON(landTestHead, "demo/foo"), "verification_failed"},
		{"stale commit", func(e *landEvidence) { e.commit = "deadbeef" }, headJSON(landTestHead, "demo/foo"), "verification_stale"},
		{"dirty commit", func(e *landEvidence) { e.commit = landTestHead + "-dirty" }, headJSON(landTestHead, "demo/foo"), "verification_stale"},
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

// The missing-record refusal fires before any gh call and carries a fix hint
// through --json.
func TestLandPRMissingEvidenceRefusesBeforeGh(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
	s := stubSideFX(t)

	out, _, err := runCmd(t, newRootCmd(), "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json")
	if err == nil {
		t.Fatal("want refusal")
	}
	if !strings.Contains(out, "verification_missing") || !strings.Contains(out, "anvil verify") {
		t.Errorf("output = %s", out)
	}
	if len(s.viewCalls) != 0 || len(s.mergeCalls) != 0 {
		t.Errorf("gh touched: views=%v merges=%v", s.viewCalls, s.mergeCalls)
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
			branch, err := passingEvidence().check(42, wt, false)
			if c.want == "" {
				if err != nil || branch != "anvil/renamed" {
					t.Fatalf("branch=%q err=%v", branch, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %s", err, c.want)
			}
		})
	}
}

// An already-MERGED PR skips the verdict checks: refusing after the merge
// would strand the issue in-progress.
func TestLandEvidenceSkippedWhenAlreadyMerged(t *testing.T) {
	s := stubSideFX(t)
	s.viewByField[landHeadFields] = headJSON("other-head", "demo/foo")
	ev := passingEvidence()
	ev.verdict = "fail"
	branch, err := ev.check(42, "", true)
	if err != nil || branch != "demo/foo" {
		t.Fatalf("branch=%q err=%v", branch, err)
	}
}

// A stale head refuses through the verb, with and without --local-validated,
// before any merge call.
func TestLandPRStaleEvidenceRefusesThroughVerb(t *testing.T) {
	for _, local := range []bool{false, true} {
		name := "ci-gated"
		if local {
			name = "local-validated"
		}
		t.Run(name, func(t *testing.T) {
			vault := t.TempDir()
			t.Setenv("ANVIL_VAULT", vault)
			execCmd(t, "init", vault)
			createDemoIssue(t)
			execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")
			s := stubSideFX(t)
			stampLandEvidence(t, "demo.foo", "pass", "deadbeef")
			s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
			s.viewByField["state"] = []byte(`{"state":"OPEN"}`)
			args := []string{"transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json"}
			if local {
				args = append(args, "--local-validated")
			}
			out, _, err := runCmd(t, newRootCmd(), args...)
			if err == nil {
				t.Fatal("want refusal")
			}
			if !strings.Contains(out, "verification_stale") {
				t.Errorf("output = %s", out)
			}
			if len(s.mergeCalls) != 0 {
				t.Errorf("merge calls = %v, want none", s.mergeCalls)
			}
		})
	}
}

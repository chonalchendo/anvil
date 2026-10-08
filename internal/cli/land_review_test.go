package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestLatestReviewRoundPicksHighestRoundAndIgnoresLow(t *testing.T) {
	body := "## Problem\n\nquotes [blocker] mid-line and a [high] too.\n\n" +
		"## Review findings — PR 42, round 2 @ bbbbbbb\n\n[low] x.go:1 — nit.\nprose says [medium] here.\n\nFindings: 1\n\n" +
		"## Review findings — PR 42, round 1 @ aaaaaaa\n\n[medium] x.go:2 — old.\n\n" +
		"## Review findings — PR 7, round 9 @ ccccccc\n\n[blocker] other PR.\n"
	rr, ok := latestReviewRound(body, 42)
	if !ok || rr.round != 2 || rr.sha != "bbbbbbb" || rr.blocked {
		t.Fatalf("got %+v ok=%v, want round 2 @ bbbbbbb unblocked", rr, ok)
	}
	if _, ok := latestReviewRound(body, 99); ok {
		t.Error("PR 99 has no round")
	}
}

func TestLatestReviewRoundBlockedStopsAtNextHeading(t *testing.T) {
	body := "## Review findings — PR 42, round 1 @ aaaaaaa\n\n[high] x.go:2 — bad.\n"
	if rr, _ := latestReviewRound(body, 42); !rr.blocked {
		t.Error("[high] line must block")
	}
	body = "## Review findings — PR 42, round 1 @ aaaaaaa\n\nFindings: 0\n\n## Links\n\n[blocker] outside the section.\n"
	if rr, _ := latestReviewRound(body, 42); rr.blocked {
		t.Error("a band after the next heading must not block")
	}
}

func TestLatestReviewRoundMatchesRealBody(t *testing.T) {
	body := "## Review findings — PR 482, round 1 @ 2b13bab\n\n[medium] a.go:1 — x.\n\n## Review findings — PR 482, round 2 @ 72748da\n\n[low] b.go:1 — y.\n"
	rr, ok := latestReviewRound(body, 482)
	if !ok || rr.sha != "72748da" || rr.blocked {
		t.Fatalf("got %+v ok=%v", rr, ok)
	}
}

// landReviewSetup builds the happy-path land with the fixture's review
// sections replaced by review, then claims the issue.
func landReviewSetup(t *testing.T, review string) (*sideFXStub, string) {
	t.Helper()
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "demo.foo.md"))
	if err != nil {
		t.Fatal(err)
	}
	head, _, _ := strings.Cut(fixtureIssueBody, "## Review findings")
	a.Body = head + review
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	s := stubSideFX(t)
	s.viewByField["mergeable,mergeStateStatus"] = []byte(`{"mergeable":"MERGEABLE","mergeStateStatus":"CLEAN"}`)
	s.viewSeq["state"] = [][]byte{[]byte(`{"state":"OPEN"}`)}
	s.viewByField["state"] = []byte(`{"state":"MERGED"}`)
	s.viewByField[landHeadFields] = headJSON(landTestHead, "demo/foo")
	s.listEntries["demo/foo"] = worktreeInfo{path: "/worktrees/foo"}
	return s, vault
}

func landReviewRefusal(t *testing.T, review, code string) {
	t.Helper()
	s, _ := landReviewSetup(t, review)
	out, _, err := runCmd(t, newRootCmd(), "transition", "issue", "demo.foo", "resolved", "--land-pr", "42", "--json")
	if err == nil || !strings.Contains(out, code) {
		t.Fatalf("err=%v out=%s, want %s", err, out, code)
	}
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge calls = %v, want none", s.mergeCalls)
	}
}

func TestLandPRReviewMissingRefuses(t *testing.T) {
	landReviewRefusal(t, "", "land_pr_review_missing")
}

func TestLandPRReviewStaleRefuses(t *testing.T) {
	landReviewRefusal(t, "## Review findings — PR 42, round 1 @ deadbee\n\nFindings: 0\n", "land_pr_review_stale")
}

func TestLandPRReviewBlockedRefuses(t *testing.T) {
	landReviewRefusal(t, "## Review findings — PR 42, round 1 @ 0123456\n\n[medium] x.go:1 — bad.\n", "land_pr_review_blocked")
}

func TestLandPRReviewCleanMergesAndStampsHead(t *testing.T) {
	s, vault := landReviewSetup(t, "## Review findings — PR 42, round 1 @ 0123456\n\n[low] x.go:1 — nit.\n")
	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")
	if len(s.mergeCalls) != 1 {
		t.Errorf("merge calls = %v, want one", s.mergeCalls)
	}
	if got := loadIssueDoc(t, vault, "demo.foo").FrontMatter["review_head"]; got != landTestHead {
		t.Errorf("review_head = %v, want %s", got, landTestHead)
	}
}

func TestLandPRReviewSkippedWhenAlreadyMerged(t *testing.T) {
	s, vault := landReviewSetup(t, "")
	s.viewSeq["state"] = nil
	execCmd(t, "transition", "issue", "demo.foo", "resolved", "--land-pr", "42")
	if len(s.mergeCalls) != 0 {
		t.Errorf("merge calls = %v, want none", s.mergeCalls)
	}
	if _, ok := loadIssueDoc(t, vault, "demo.foo").FrontMatter["review_head"]; ok {
		t.Error("an already-merged retry must not stamp review_head")
	}
}

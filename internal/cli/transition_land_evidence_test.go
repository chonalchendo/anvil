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
// fixture issue; it is a no-op when the fixture does not exist yet.
func stampLandEvidence(t *testing.T, id, verdict, commit string) {
	t.Helper()
	path := filepath.Join(os.Getenv("ANVIL_VAULT"), "70-issues", id+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		return
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
			err := ev.check(42)
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
	if err := passingEvidence().check(42); err == nil || !strings.Contains(err.Error(), "land_pr_view_failed") {
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
	// stubSideFX stamps the fixture; strip the record to model an unverified issue.
	path := filepath.Join(vault, "70-issues", "demo.foo.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	delete(a.FrontMatter, "verified_verdict")
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

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

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLandReviewStaleByPatch(t *testing.T) {
	cases := []struct {
		name string
		ids  map[string]string // sha prefix -> patch-id; absent = unreachable
		want string
	}{
		{"same patch is fresh", map[string]string{"deadbee": "p1", landTestShort: "p1"}, ""},
		{"changed patch is stale", map[string]string{"deadbee": "p1", landTestShort: "p2"}, "land_pr_review_stale"},
		{"unreachable round sha is stale", map[string]string{landTestShort: "p1"}, "land_pr_review_stale"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := stubSideFX(t)
			s.viewByField[landHeadFields] = headJSON(landTestHead, "demo/foo")
			s.viewByField["baseRefName"] = []byte(`{"baseRefName":"master"}`)
			old := gitLayerPatchIDFn
			t.Cleanup(func() { gitLayerPatchIDFn = old })
			gitLayerPatchIDFn = func(_, base, commit string, _ bool) (string, error) {
				if base != "origin/master" {
					t.Errorf("base = %s", base)
				}
				if id, ok := c.ids[commit[:7]]; ok {
					return id, nil
				}
				return "", errors.New("unknown revision")
			}
			ev := passingEvidence()
			ev.body = "## Review findings — PR 42, round 1 @ deadbee\n\nFindings: 0\n"
			_, err := ev.check(42, "", false)
			if c.want == "" {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %s", err, c.want)
			}
		})
	}
}

func TestLayerPatchIDReal(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	d := filepath.Join(root, "work")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", remote)
	gitIn(t, root, "clone", "-q", remote, d)
	write := func(f, s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(d, f), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(msg string) string {
		gitIn(t, d, "add", ".")
		gitIn(t, d, "commit", "-q", "-m", msg)
		return gitIn(t, d, "rev-parse", "HEAD")
	}
	write("a", "1\n")
	commit("base")
	gitIn(t, d, "push", "-q", "origin", "HEAD:main")

	// Stack: main <- lower <- upper. The upper layer's base ref is origin/lower.
	gitIn(t, d, "checkout", "-q", "-b", "lower")
	write("b", "lower\n")
	oldLower := commit("lower")
	gitIn(t, d, "push", "-q", "origin", "lower")
	gitIn(t, d, "checkout", "-q", "-b", "upper")
	write("c", "upper\n")
	reviewed := commit("upper")

	// Main moves; lower is rebased and force-pushed; upper is rebased onto it.
	gitIn(t, d, "checkout", "-q", "main")
	write("m", "moved\n")
	commit("moved")
	gitIn(t, d, "push", "-q", "origin", "main")
	gitIn(t, d, "checkout", "-q", "lower")
	gitIn(t, d, "rebase", "-q", "main")
	gitIn(t, d, "push", "-q", "--force", "origin", "lower")
	gitIn(t, d, "checkout", "-q", "upper")
	gitIn(t, d, "rebase", "-q", "--onto", "lower", oldLower)
	restacked := gitIn(t, d, "rev-parse", "HEAD")
	if reviewed == restacked {
		t.Fatal("rebase did not move the head")
	}
	if !samePatch(d, "origin/lower", reviewed, restacked) {
		t.Error("restack of a stacked layer changed the patch-id")
	}
	write("c", "edited\n")
	gitIn(t, d, "commit", "-q", "-am", "edit")
	if samePatch(d, "origin/lower", reviewed, gitIn(t, d, "rev-parse", "HEAD")) {
		t.Error("an edit kept the patch-id")
	}
	if samePatch(d, "origin/lower", "feedface", restacked) {
		t.Error("unreachable sha read as same")
	}
}

// A commit dropped from the base must not hide behind the fork point: the
// upper layer lost U1, so its patch changed and the review is stale.
func TestLayerPatchIDDroppedBaseCommit(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	d := filepath.Join(root, "work")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", remote)
	gitIn(t, root, "clone", "-q", remote, d)
	commitFile := func(f, s string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(d, f), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
		gitIn(t, d, "add", ".")
		gitIn(t, d, "commit", "-q", "-m", f)
		return gitIn(t, d, "rev-parse", "HEAD")
	}
	commitFile("a", "1\n")
	gitIn(t, d, "push", "-q", "origin", "HEAD:main")
	gitIn(t, d, "checkout", "-q", "-b", "lower")
	lowerTip := commitFile("b", "lower\n")
	gitIn(t, d, "push", "-q", "origin", "lower")
	gitIn(t, d, "checkout", "-q", "-b", "upper")
	commitFile("u1", "u1\n")
	commitFile("u2", "u2\n")
	reviewed := gitIn(t, d, "rev-parse", "HEAD")

	// origin/lower fast-forwards to U1, then is forced back to its old tip.
	gitIn(t, d, "push", "-q", "origin", "HEAD~1:refs/heads/lower")
	gitIn(t, d, "push", "-q", "--force", "origin", lowerTip+":refs/heads/lower")
	gitIn(t, d, "rebase", "-q", "--onto", lowerTip, "HEAD~1")
	head := gitIn(t, d, "rev-parse", "HEAD")
	if samePatch(d, "origin/lower", reviewed, head) {
		t.Error("a dropped base commit read as the same patch")
	}
}

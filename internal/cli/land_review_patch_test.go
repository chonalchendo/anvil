package cli

import (
	"errors"
	"os"
	"os/exec"
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
			old := layerPatchIDFn
			t.Cleanup(func() { layerPatchIDFn = old })
			layerPatchIDFn = func(_, base, commit string) (string, error) {
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

// A restack onto a moved base keeps the layer patch-id; an edit changes it.
func TestLayerPatchIDReal(t *testing.T) {
	d := t.TempDir()
	g := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-c", "user.email=a@b", "-c", "user.name=a"}, args...)...)
		c.Dir = d
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(f, s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(d, f), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g("init", "-b", "main")
	write("a", "1\n")
	g("add", ".")
	g("commit", "-m", "base")
	g("checkout", "-b", "layer")
	write("b", "layer\n")
	g("add", ".")
	g("commit", "-m", "layer")
	old := g("rev-parse", "HEAD")
	g("checkout", "main")
	write("c", "other\n")
	g("add", ".")
	g("commit", "-m", "moved")
	g("checkout", "layer")
	g("rebase", "main")
	restacked := g("rev-parse", "HEAD")
	if old == restacked {
		t.Fatal("rebase did not move the head")
	}
	if !samePatch(d, "main", old, restacked) {
		t.Error("restack changed the patch-id")
	}
	write("b", "edited\n")
	g("commit", "-am", "edit")
	if samePatch(d, "main", old, g("rev-parse", "HEAD")) {
		t.Error("an edit kept the patch-id")
	}
	if samePatch(d, "main", "feedface", restacked) {
		t.Error("unreachable sha read as same")
	}
}

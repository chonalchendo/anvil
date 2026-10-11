package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMilestoneStatusAt(t *testing.T) {
	finishLineVault(t, "test -f marker")
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", repo, "-c", "user.email=a@b.c", "-c", "user.name=n"}, args...)
		if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	git("checkout", "-q", "-b", "marked")
	if err := os.WriteFile(filepath.Join(repo, "marker"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "marker")
	git("commit", "-q", "-m", "marker")
	git("checkout", "-q", "main")
	stubBranches(t, "base")
	resolveProjectRepoFn = func(string) (string, error) { return repo, nil }

	met := func(args ...string) bool {
		t.Helper()
		var st struct {
			Acceptance []acceptanceResult `json:"acceptance"`
		}
		args = append([]string{"milestone", "status", "demo.line", "--json"}, args...)
		if err := json.Unmarshal([]byte(execCmdJSON(t, args...)), &st); err != nil {
			t.Fatal(err)
		}
		return st.Acceptance[0].Met
	}
	if met() {
		t.Fatal("without --at the marker is absent on main")
	}
	if !met("--at", "marked") {
		t.Fatal("--at marked should see the marker")
	}
}

package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

const statusFixtureBody = "## Objective\n\nx\n\n## Status\n\nMeasured: 2026-01-01. stale.\n\n| AC | Met | Measured |\n|---|---|---|\n| old | met | old |\n\n## Links\n\n- none\n"

// finishLineVault seeds a vault with an in-progress milestone carrying
// the given acceptance predicates.
func finishLineVault(t *testing.T, acceptance ...any) string {
	t.Helper()
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureMilestone(t, vault, "demo.line", "in-progress")
	p := filepath.Join(vault, "85-milestones", "demo.line.md")
	m, err := core.LoadArtifact(p)
	if err != nil {
		t.Fatal(err)
	}
	m.FrontMatter["acceptance"] = acceptance
	m.Body = statusFixtureBody
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")
	return vault
}

func TestMilestoneStatusReportsAcceptance(t *testing.T) {
	finishLineVault(t, "true", "exit 3")

	var st struct {
		Acceptance []acceptanceResult `json:"acceptance"`
	}
	if err := json.Unmarshal([]byte(execCmdJSON(t, "milestone", "status", "demo.line", "--json")), &st); err != nil {
		t.Fatal(err)
	}
	if len(st.Acceptance) != 2 || !st.Acceptance[0].Met || st.Acceptance[1].Met || st.Acceptance[1].Exit != 3 {
		t.Fatalf("acceptance = %+v", st.Acceptance)
	}

	out := execCmd(t, "milestone", "status", "demo.line")
	if !strings.Contains(out, "AC 1\tmet\texit 0\ttrue") || !strings.Contains(out, "AC 2\tnot met\texit 3\texit 3") {
		t.Fatalf("human output = %q", out)
	}
}

func TestTransitionMilestoneDoneGates(t *testing.T) {
	failCode := func(t *testing.T) string {
		t.Helper()
		stdout, _, err := runCmd(t, newRootCmd(), "transition", "milestone", "demo.line", "done", "--json")
		if err == nil {
			t.Fatal("expected refusal")
		}
		var env map[string]any
		if jerr := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &env); jerr != nil {
			t.Fatalf("stdout %q: %v", stdout, jerr)
		}
		code, _ := env["code"].(string)
		return code
	}

	t.Run("red acceptance refuses", func(t *testing.T) {
		finishLineVault(t, "false")
		if got := failCode(t); got != "acceptance_unmet" {
			t.Fatalf("code = %q", got)
		}
	})

	t.Run("open issue refuses before running acceptance", func(t *testing.T) {
		vault := finishLineVault(t, "false")
		writeFixtureIssueWithMilestone(t, vault, "demo", "child", "demo.line")
		execCmd(t, "reindex")
		if got := failCode(t); got != "milestone_open_issues" {
			t.Fatalf("code = %q", got)
		}
	})

	t.Run("escalated issue refuses", func(t *testing.T) {
		vault := finishLineVault(t, "true")
		writeFixtureIssueWithMilestone(t, vault, "demo", "child", "demo.line")
		p := filepath.Join(vault, "70-issues", "demo.child.md")
		a, err := core.LoadArtifact(p)
		if err != nil {
			t.Fatal(err)
		}
		a.FrontMatter["status"] = "escalated"
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
		execCmd(t, "reindex")
		if got := failCode(t); got != "milestone_open_issues" {
			t.Fatalf("code = %q", got)
		}
	})

	t.Run("green line rewrites the status block", func(t *testing.T) {
		vault := finishLineVault(t, "true")
		execCmd(t, "transition", "milestone", "demo.line", "done")
		raw, err := os.ReadFile(filepath.Join(vault, "85-milestones", "demo.line.md")) //nolint:gosec // test-controlled path
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		for _, want := range []string{"status: done", "Every acceptance predicate passes.", "| 1 | `true` | met | exit 0 |", "## Links"} {
			if !strings.Contains(body, want) {
				t.Errorf("missing %q in:\n%s", want, body)
			}
		}
		if strings.Contains(body, "stale.") || strings.Contains(body, "| old |") {
			t.Errorf("old status block survived:\n%s", body)
		}
	})
}

func TestMilestoneCloseAdvisoryNeedsGreenLine(t *testing.T) {
	for name, tc := range map[string]struct {
		acceptance string
		advised    bool
	}{"green": {"true", true}, "red": {"false", false}} {
		t.Run(name, func(t *testing.T) {
			vault := finishLineVault(t, tc.acceptance)
			writeFixtureIssueWithMilestone(t, vault, "demo", "child", "demo.line")
			execCmd(t, "reindex")
			execCmd(t, "transition", "issue", "demo.child", "in-progress", "--owner", "claude")
			out := execCmd(t, "transition", "issue", "demo.child", "resolved")
			if got := strings.Contains(out, "consider: anvil transition milestone"); got != tc.advised {
				t.Fatalf("advised = %v, want %v; out: %s", got, tc.advised, out)
			}
		})
	}
}

// stubBranches stands in for a project repo whose HEAD is at headSHA while the
// default branch tip is at "base".
func stubBranches(t *testing.T, headSHA string) {
	t.Helper()
	prevRepo, prevHead, prevRev := resolveProjectRepoFn, gitResolveOriginHEADFn, gitRevParseFn
	t.Cleanup(func() {
		resolveProjectRepoFn, gitResolveOriginHEADFn, gitRevParseFn = prevRepo, prevHead, prevRev
	})
	resolveProjectRepoFn = func(string) (string, error) { return t.TempDir(), nil }
	gitResolveOriginHEADFn = func(string) (string, error) { return "origin/master", nil }
	gitRevParseFn = func(_ string, args ...string) (string, error) {
		if args[len(args)-1] != "HEAD" {
			return "base", nil
		}
		if args[0] == "--short" {
			return "abc1234", nil
		}
		return headSHA, nil
	}
}

func TestMilestoneDoneRefusesOffDefaultBranch(t *testing.T) {
	t.Run("done refuses", func(t *testing.T) {
		finishLineVault(t, "true")
		stubBranches(t, "feat")
		stdout, _, err := runCmd(t, newRootCmd(), "transition", "milestone", "demo.line", "done", "--json")
		if err == nil || !strings.Contains(stdout, "finish_line_not_on_base") {
			t.Fatalf("err = %v, stdout = %q", err, stdout)
		}
	})
	t.Run("done passes on the default branch", func(t *testing.T) {
		finishLineVault(t, "true")
		stubBranches(t, "base")
		execCmd(t, "transition", "milestone", "demo.line", "done")
	})
	t.Run("status only warns", func(t *testing.T) {
		finishLineVault(t, "true")
		stubBranches(t, "feat")
		stdout, stderr, err := runCmd(t, newRootCmd(), "milestone", "status", "demo.line")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stderr, "warning: HEAD feat is not the default branch tip base") || !strings.Contains(stdout, "AC 1\tmet") {
			t.Fatalf("stdout = %q stderr = %q", stdout, stderr)
		}
	})
}

func TestMilestoneStatusDoneNeedsGreenLineAndNoOpenIssues(t *testing.T) {
	finishLineVault(t, "false")
	var st struct {
		Done bool `json:"done"`
	}
	if err := json.Unmarshal([]byte(execCmdJSON(t, "milestone", "status", "demo.line", "--json")), &st); err != nil {
		t.Fatal(err)
	}
	if st.Done {
		t.Fatal("done = true with a red predicate")
	}
}

func TestAcceptanceNoticeAndDetail(t *testing.T) {
	finishLineVault(t, "true", "true")
	_, stderr, err := runCmd(t, newRootCmd(), "milestone", "status", "demo.line")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(stderr, "anvil: running acceptance predicate"); got != 2 {
		t.Fatalf("notices = %d, stderr = %q", got, stderr)
	}
	if got := (acceptanceResult{TimedOut: true}).detail(); got != "timed out" {
		t.Fatalf("detail = %q", got)
	}
	if got := (acceptanceResult{RunError: "boom"}).detail(); got != "boom" {
		t.Fatalf("detail = %q", got)
	}
}

func TestReplaceStatusBlock(t *testing.T) {
	const block = "## Status\n\nNEW\n"
	t.Run("H3 above is untouched", func(t *testing.T) {
		body := "## Objective\n\n### Status\n\nkeep\n\n## Status\n\nold\n\n## Links\n\n- x\n"
		want := "## Objective\n\n### Status\n\nkeep\n\n## Status\n\nNEW\n\n## Links\n\n- x\n"
		if got := replaceStatusBlock(body, block); got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("fenced Status is untouched", func(t *testing.T) {
		body := "## Objective\n\n```\n## Status\nfenced\n```\n\n## Status\n\nold\n"
		want := "## Objective\n\n```\n## Status\nfenced\n```\n\n## Status\n\nNEW\n"
		if got := replaceStatusBlock(body, block); got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("missing section appends", func(t *testing.T) {
		if got := replaceStatusBlock("## Objective\n\nx\n", block); got != "## Objective\n\nx\n\n## Status\n\nNEW\n" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestTableCellTruncatesByRunes(t *testing.T) {
	got := tableCell(strings.Repeat("é", 80))
	if r := []rune(got); len(r) != 60 || !strings.HasSuffix(got, "...") {
		t.Fatalf("got %q", got)
	}
}

func TestMilestoneDoneStampsCommit(t *testing.T) {
	vault := finishLineVault(t, "true")
	stubBranches(t, "base")
	execCmd(t, "transition", "milestone", "demo.line", "done")
	raw, err := os.ReadFile(filepath.Join(vault, "85-milestones", "demo.line.md")) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\nMeasured: ") || !strings.Contains(string(raw), ", at `abc1234`. Every") {
		t.Fatalf("body:\n%s", raw)
	}
}

func TestMilestoneStatusWarnsBaseUnchecked(t *testing.T) {
	finishLineVault(t, "true")
	prev := resolveProjectRepoFn
	t.Cleanup(func() { resolveProjectRepoFn = prev })
	resolveProjectRepoFn = func(string) (string, error) { return "", errors.New("no repo") }
	_, stderr, err := runCmd(t, newRootCmd(), "milestone", "status", "demo.line")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "warning: base branch not checked (no repo)") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestMilestoneScanFailure(t *testing.T) {
	vault := finishLineVault(t, "true")
	issues := filepath.Join(vault, "70-issues")
	if err := os.MkdirAll(issues, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issues, "demo.broken.md"), []byte("---\n: [unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runCmd(t, newRootCmd(), "transition", "milestone", "demo.line", "done", "--json")
	if err == nil || !strings.Contains(stdout, "milestone_scan_failed") {
		t.Fatalf("err = %v, stdout = %q", err, stdout)
	}
	_, stderr, err := runCmd(t, newRootCmd(), "milestone", "status", "demo.line")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "warning: issue scan failed:") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestAcceptanceRunsInProjectRepo(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "master"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "x"},
	} {
		c := exec.Command("git", args...) //nolint:gosec // test-controlled args
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	finishLineVault(t, `test "$(git rev-parse --abbrev-ref HEAD)" = master`)
	prevRepo, prevHead := resolveProjectRepoFn, gitResolveOriginHEADFn
	t.Cleanup(func() { resolveProjectRepoFn, gitResolveOriginHEADFn = prevRepo, prevHead })
	resolveProjectRepoFn = func(string) (string, error) { return repo, nil }
	gitResolveOriginHEADFn = func(string) (string, error) { return "master", nil }

	// The test process cwd is this package, not repo, and is on another branch.
	execCmd(t, "transition", "milestone", "demo.line", "done")
}

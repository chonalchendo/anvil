package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

const statusFixtureBody = "## Objective\n\nx\n\n## Status\n\nMeasured: 2026-01-01. stale.\n\n| AC | Met | Measured |\n|---|---|---|\n| old | met | old |\n\n## Links\n\n- none\n"

// finishLineVault seeds a vault with an in-progress scoped milestone carrying
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

func stubBranches(t *testing.T, current string) {
	t.Helper()
	prevRepo, prevHead, prevBranch := resolveProjectRepoFn, gitResolveOriginHEADFn, gitCurrentBranchFn
	t.Cleanup(func() {
		resolveProjectRepoFn, gitResolveOriginHEADFn, gitCurrentBranchFn = prevRepo, prevHead, prevBranch
	})
	resolveProjectRepoFn = func(string) (string, error) { return t.TempDir(), nil }
	gitResolveOriginHEADFn = func(string) (string, error) { return "origin/master", nil }
	gitCurrentBranchFn = func(string) (string, error) { return current, nil }
}

func TestMilestoneDoneRefusesOffDefaultBranch(t *testing.T) {
	t.Run("done refuses", func(t *testing.T) {
		finishLineVault(t, "true")
		stubBranches(t, "anvil/feature")
		stdout, _, err := runCmd(t, newRootCmd(), "transition", "milestone", "demo.line", "done", "--json")
		if err == nil || !strings.Contains(stdout, "finish_line_not_on_base") {
			t.Fatalf("err = %v, stdout = %q", err, stdout)
		}
	})
	t.Run("done passes on the default branch", func(t *testing.T) {
		finishLineVault(t, "true")
		stubBranches(t, "master")
		execCmd(t, "transition", "milestone", "demo.line", "done")
	})
	t.Run("status only warns", func(t *testing.T) {
		finishLineVault(t, "true")
		stubBranches(t, "anvil/feature")
		stdout, stderr, err := runCmd(t, newRootCmd(), "milestone", "status", "demo.line")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stderr, "warning: on branch anvil/feature, not master") || !strings.Contains(stdout, "AC 1\tmet") {
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

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
	if !strings.Contains(out, "met\texit 0\ttrue") || !strings.Contains(out, "not met\texit 3\texit 3") {
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
		for _, want := range []string{"status: done", "Every acceptance predicate passes.", "| `true` | met | exit 0 |", "## Links"} {
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

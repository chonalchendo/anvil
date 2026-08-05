package installer

import (
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// run-verification.sh is a shipped shell script every dispatched worker gates on
// (`… | jq -r .verdict`): its stdout must stay exactly one JSON verdict line, with
// the human summary on stderr, or an orchestrator cannot mechanically tell a real
// pass from a worker's narrated one. Nothing else in the tree executes the script,
// so a regression in that contract would only surface mid-fleet.
//
// It lives beside the installer — the package that materialises the script into
// ~/.claude/skills — for the same reason as the genericity gate.

type verdict struct {
	Verdict string `json:"verdict"`
	Checks  int    `json:"checks"`
	Failed  []struct {
		Check   string `json:"check"`
		Exit    *int   `json:"exit"`
		Preview string `json:"preview"`
	} `json:"failed"`
}

func issueDoc(direct, indirect string) string {
	return "## Verification\n\n### Direct (unit/integration)\n```bash\n" + direct +
		"\n```\n\n### Indirect (live smoke)\n```bash\n" + indirect + "\n```\n"
}

// runVerification pipes doc through the shipped runner, returning its parsed
// stdout verdict, raw stderr and exit code.
func runVerification(t *testing.T, doc string) (verdict, string, int) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "anvil", "skills", "completing-issue", "scripts", "run-verification.sh"))
	if err != nil {
		t.Fatalf("resolve script path: %v", err)
	}
	cmd := exec.Command("bash", script)
	cmd.Stdin = strings.NewReader(doc)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	exit := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run %s: %v", script, err)
		}
		exit = exitErr.ExitCode()
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %d lines, want exactly 1 verdict line:\n%s", len(lines), stdout.String())
	}
	var v verdict
	if err := json.Unmarshal([]byte(lines[0]), &v); err != nil {
		t.Fatalf("parse verdict %q: %v", lines[0], err)
	}
	return v, stderr.String(), exit
}

func TestRunVerification_PassEmitsVerdictLine(t *testing.T) {
	v, stderr, exit := runVerification(t, issueDoc("true", "true"))

	if v.Verdict != "pass" || v.Checks != 2 || len(v.Failed) != 0 {
		t.Errorf("verdict = %+v, want pass with 2 checks and no failures", v)
	}
	if exit != 0 {
		t.Errorf("exit = %d, want 0", exit)
	}
	if !strings.Contains(stderr, "PASS [Direct#1]") || !strings.Contains(stderr, "All checks passed.") {
		t.Errorf("stderr lost the human summary:\n%s", stderr)
	}
}

func TestRunVerification_FailNamesTheFailedCheck(t *testing.T) {
	v, stderr, exit := runVerification(t, issueDoc("true", "false"))

	if v.Verdict != "fail" || v.Checks != 2 {
		t.Errorf("verdict = %+v, want fail with 2 checks", v)
	}
	if len(v.Failed) != 1 || v.Failed[0].Check != "Indirect#1" {
		t.Fatalf("failed = %+v, want one entry naming Indirect#1", v.Failed)
	}
	if exit != 1 {
		t.Errorf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr, "FAIL [Indirect#1]") {
		t.Errorf("stderr lost the human summary:\n%s", stderr)
	}
}

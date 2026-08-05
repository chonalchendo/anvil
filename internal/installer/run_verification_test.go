package installer

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/anvil/skills"
)

// run-verification.sh is a shipped shell script every dispatched worker gates on
// (`… | jq -r .verdict`): its stdout must stay exactly one JSON verdict line, with
// the human summary on stderr, or an orchestrator cannot mechanically tell a real
// pass from a worker's narrated one — convention.cli-tooling rule 7 (stdout content,
// stderr diagnostics, `--json` pipes cleanly to jq) is the contract under test.
// Nothing else in the tree executes the script, so a regression would only surface
// mid-fleet.
//
// It lives beside the installer — the package that materialises the script into
// ~/.claude/skills — for the same reason as the genericity gate, and runs the copy
// read out of skills.FS so the assertion binds the artifact that actually ships,
// not a checkout file the embed could have drifted from.

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
	b, err := skills.FS.ReadFile("completing-issue/scripts/run-verification.sh")
	if err != nil {
		t.Fatalf("read embedded runner: %v", err)
	}
	script := filepath.Join(t.TempDir(), "run-verification.sh")
	if err := os.WriteFile(script, b, 0o755); err != nil { //nolint:gosec // the shipped script is executable by design
		t.Fatalf("write runner: %v", err)
	}
	cmd := exec.Command("bash", script) //nolint:gosec // script path is a test temp dir holding the embedded runner, not user input
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
	// The failing predicate carries double quotes so the runner's hand-rolled
	// JSON escaping in add_fail is exercised: a preview that mangles quotes
	// yields unparseable stdout, which is the same fleet-wide break as a
	// missing verdict line.
	failing := `grep -q '"verdict"' /dev/null`
	v, stderr, exit := runVerification(t, issueDoc("true", failing))

	if v.Verdict != "fail" || v.Checks != 2 {
		t.Errorf("verdict = %+v, want fail with 2 checks", v)
	}
	if len(v.Failed) != 1 || v.Failed[0].Check != "Indirect#1" {
		t.Fatalf("failed = %+v, want one entry naming Indirect#1", v.Failed)
	}
	if v.Failed[0].Preview != failing {
		t.Errorf("preview = %q, want the quote-bearing predicate %q", v.Failed[0].Preview, failing)
	}
	if exit != 1 {
		t.Errorf("exit = %d, want 1", exit)
	}
	if !strings.Contains(stderr, "FAIL [Indirect#1]") {
		t.Errorf("stderr lost the human summary:\n%s", stderr)
	}
}

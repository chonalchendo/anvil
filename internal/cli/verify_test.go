package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

func writeVerifyIssue(t *testing.T, vault, id, direct, indirect string) {
	t.Helper()
	a := &core.Artifact{
		Path: filepath.Join(vault, "70-issues", id+".md"),
		FrontMatter: map[string]any{
			"type": "issue", "title": id, "description": "fixture",
			"created": "2026-05-15", "updated": "2026-05-15",
			"status": "open", "project": "anvil", "severity": "medium",
			"tags": []any{"domain/cli"},
		},
		Body: "## Verification\n\n### Direct\n\n```bash\n" + direct + "\n```\n\n### Indirect\n\n```bash\n" + indirect + "\n```\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func runVerify(t *testing.T, vault, id string) (verifyRecord, *core.Artifact, error) {
	t.Helper()
	t.Setenv("ANVIL_VAULT", vault)
	out, _, err := runCmd(t, newVerifyCmd(), id, "--json")
	var rec verifyRecord
	if jerr := json.Unmarshal([]byte(strings.SplitN(out, "\n", 2)[0]), &rec); jerr != nil {
		t.Fatalf("no JSON record on stdout: %v\n%q", jerr, out)
	}
	a, lerr := core.LoadArtifact(filepath.Join(vault, "70-issues", id+".md"))
	if lerr != nil {
		t.Fatal(lerr)
	}
	return rec, a, err
}

func TestVerifyRecordsPass(t *testing.T) {
	vault := setupVault(t)
	writeVerifyIssue(t, vault, "issue.anvil.0001.ok", "true", "true")
	rec, a, err := runVerify(t, vault, "issue.anvil.0001.ok")
	if err != nil {
		t.Fatalf("pass verdict must exit 0: %v", err)
	}
	if rec.Verdict != "pass" || rec.Checks != 2 || len(rec.Failed) != 0 {
		t.Errorf("record = %+v", rec)
	}
	if a.FrontMatter["verified_verdict"] != "pass" || a.FrontMatter["verified_at"] == "" || a.FrontMatter["verified_at"] != rec.RanAt {
		t.Errorf("frontmatter not stamped: %v", a.FrontMatter)
	}
	if _, ok := a.FrontMatter["verified_commit"]; !ok {
		t.Error("verified_commit missing")
	}
}

func intp(n int) *int { return &n }

func TestVerifyScriptContractCases(t *testing.T) {
	const wrap = "## Verification\n\n### Direct\n\n```bash\n%s\n```\n\n### Indirect\n\n```bash\n%s\n```\n"
	cases := []struct {
		name     string
		body     string
		verdict  string
		checks   int
		failed   []verifyFailure
		deferred []verifyFailure
	}{
		{
			name: "set -e red mid-block", body: fmt.Sprintf(wrap, "true", "echo setup\nfalse"), verdict: "fail", checks: 2,
			failed: []verifyFailure{{Check: "Indirect#1", Exit: intp(1), Line: "false", Preview: "echo setup"}},
		},
		{
			name: "explicit exit", body: fmt.Sprintf(wrap, "exit 4", "true"), verdict: "fail", checks: 2,
			failed: []verifyFailure{{Check: "Direct#1", Exit: intp(4), Preview: "exit 4"}},
		},
		{
			name: "all-comment block", body: fmt.Sprintf(wrap, "# nothing", "true"), verdict: "fail", checks: 2,
			failed: []verifyFailure{{Check: "Direct#1", Preview: "block has no executable command"}},
		},
		{
			name: "missing Direct section", body: "## Verification\n\n### Indirect\n\n```bash\ntrue\n```\n", verdict: "fail", checks: 2,
			failed: []verifyFailure{{Check: "Direct", Preview: "no executable bash block"}},
		},
		{
			name: "no Verification at all", body: "## Notes\n", verdict: "fail", checks: 2,
			failed: []verifyFailure{{Check: "Direct", Preview: "no executable bash block"}, {Check: "Indirect", Preview: "no executable bash block"}},
		},
		{
			name: "non-gating negation", body: fmt.Sprintf(wrap, "! false\ntrue", "true"), verdict: "fail", checks: 2,
			failed: []verifyFailure{{Check: "Direct#1", Preview: "non-gating negation: ! false"}},
		},
		{
			name: "post-land marker in Direct still fails", body: fmt.Sprintf(wrap, "# anvil:post-land\nfalse", "true"), verdict: "fail", checks: 2,
			failed: []verifyFailure{{Check: "Direct#1", Exit: intp(1), Line: "false", Preview: "false"}},
		},
		{
			name: "post-land red defers", body: fmt.Sprintf(wrap, "true", "# anvil:post-land\nfalse"), verdict: "pass", checks: 2,
			deferred: []verifyFailure{{Check: "Indirect#1", Exit: intp(1), Line: "false", Preview: "false"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			writeVerifyIssue(t, vault, "issue.anvil.0002.case", "true", "true")
			path := filepath.Join(vault, "70-issues", "issue.anvil.0002.case.md")
			a, err := core.LoadArtifact(path)
			if err != nil {
				t.Fatal(err)
			}
			a.Body = tc.body
			if err := a.Save(); err != nil {
				t.Fatal(err)
			}
			rec, got, err := runVerify(t, vault, "issue.anvil.0002.case")
			if (err != nil) != (tc.verdict == "fail") {
				t.Errorf("err = %v for verdict %s", err, tc.verdict)
			}
			want := verifyRecord{Verdict: tc.verdict, Checks: tc.checks, Failed: tc.failed, Deferred: tc.deferred}
			if want.Failed == nil {
				want.Failed = []verifyFailure{}
			}
			if want.Deferred == nil {
				want.Deferred = []verifyFailure{}
			}
			rec.Commit, rec.RanAt = "", ""
			if diff := cmp.Diff(want, rec); diff != "" {
				t.Errorf("record mismatch (-want +got):\n%s", diff)
			}
			if got.FrontMatter["verified_verdict"] != tc.verdict {
				t.Errorf("verdict not stamped: %v", got.FrontMatter)
			}
		})
	}
}

func TestVerifyMarshalsNeverRanExitAsNull(t *testing.T) {
	b, _ := json.Marshal(verifyFailure{Check: "Direct"})
	if !strings.Contains(string(b), `"exit":null`) {
		t.Errorf("got %s", b)
	}
}

func TestVerifyKeepsWritesMadeDuringTheRun(t *testing.T) {
	vault := setupVault(t)
	id := "issue.anvil.0004.race"
	path := filepath.Join(vault, "70-issues", id+".md")
	writeVerifyIssue(t, vault, id, "true", "true")
	// The Direct block edits the issue file while verify runs.
	script := "sed -i.bak 's/^severity: medium/severity: high/' " + path + " && rm -f " + path + ".bak"
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.Body = strings.Replace(a.Body, "```bash\ntrue\n```", "```bash\n"+script+"\n```", 1)
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	_, got, err := runVerify(t, vault, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.FrontMatter["severity"] != "high" {
		t.Errorf("mid-run write lost: severity = %v", got.FrontMatter["severity"])
	}
}

func TestRunFeasibilityBlockTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout time.Duration
		want    bool
	}{
		{"cap hit sets timedOut", 100 * time.Millisecond, true},
		{"zero means no cap", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if r := runFeasibilityBlock("sleep 1", "", tc.timeout); r.timedOut != tc.want {
				t.Errorf("timedOut = %v, want %v", r.timedOut, tc.want)
			}
		})
	}
}

func TestVerifyLeavesExitNilWhenTheBlockNeverRan(t *testing.T) {
	vault := setupVault(t)
	writeVerifyIssue(t, vault, "issue.anvil.0005.neverran", "true", "true")
	// An unusable TMPDIR makes the script temp file fail: runErr, not an exit code.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	rec, _, _ := runVerify(t, vault, "issue.anvil.0005.neverran")
	if len(rec.Failed) == 0 {
		t.Fatalf("a block that never ran must fail: %+v", rec)
	}
	for _, f := range rec.Failed {
		if f.Exit != nil {
			t.Errorf("%s: Exit = %d, want nil", f.Check, *f.Exit)
		}
	}
}

func TestVerifyLock(t *testing.T) {
	const id = "issue.anvil.0003.lock"
	vault := setupVault(t)
	writeVerifyIssue(t, vault, id, "true", "true")
	path := filepath.Join(vault, "70-issues", id+".md")
	edit := func(f func(a *core.Artifact)) {
		t.Helper()
		a, err := core.LoadArtifact(path)
		if err != nil {
			t.Fatal(err)
		}
		f(a)
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	// Claim stamping.
	edit(func(a *core.Artifact) { stampIssueGate(a, "in-progress", "", time.Now()) })
	t.Setenv("ANVIL_VAULT", vault)

	// Body edit outside the section does not trip the lock.
	edit(func(a *core.Artifact) { a.Body = "## Notes\n\nhi\n\n" + a.Body })
	if _, _, err := runCmd(t, newVerifyCmd(), id, "--json"); err != nil {
		t.Fatalf("edit outside section tripped lock: %v", err)
	}

	// Section edit refuses, unstamped, before running.
	edit(func(a *core.Artifact) {
		delete(a.FrontMatter, "verified_verdict")
		a.Body = strings.Replace(a.Body, "```bash\ntrue\n```\n\n### Indirect", "```bash\ntrue; true\n```\n\n### Indirect", 1)
	})
	stdout, _, err := runCmd(t, newVerifyCmd(), id, "--json")
	var se *errfmt.Structured
	if !errors.As(err, &se) || se.Code != "verification_changed" {
		t.Fatalf("want verification_changed, got %v", err)
	}
	if !strings.Contains(stdout, "verification_changed") {
		t.Errorf("--json refusal must emit the envelope on stdout, got %q", stdout)
	}
	a, _ := core.LoadArtifact(path)
	if _, ok := a.FrontMatter["verified_verdict"]; ok {
		t.Error("refusal must not stamp a verdict")
	}

	// --accept-change re-locks and runs.
	if _, _, err := runCmd(t, newVerifyCmd(), id, "--accept-change", "--json"); err != nil {
		t.Fatalf("accept-change: %v", err)
	}
	a, _ = core.LoadArtifact(path)
	if a.FrontMatter["verification_lock"] != core.VerificationLock(a.Body) || a.FrontMatter["verified_verdict"] != "pass" {
		t.Errorf("not re-locked/run: %v", a.FrontMatter)
	}
	if _, _, err := runCmd(t, newVerifyCmd(), id, "--json"); err != nil {
		t.Fatalf("after re-lock: %v", err)
	}
}

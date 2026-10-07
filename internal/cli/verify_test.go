package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

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

func TestVerifyRecordsFailAndDeferral(t *testing.T) {
	vault := setupVault(t)
	writeVerifyIssue(t, vault, "issue.anvil.0002.bad", "true", "echo setup\nfalse")
	rec, a, err := runVerify(t, vault, "issue.anvil.0002.bad")
	if err == nil {
		t.Fatal("fail verdict must exit non-zero")
	}
	if rec.Verdict != "fail" || len(rec.Failed) != 1 || rec.Failed[0].Check != "Indirect#1" || rec.Failed[0].Exit != 1 || rec.Failed[0].Line != "false" {
		t.Errorf("record = %+v", rec)
	}
	if a.FrontMatter["verified_verdict"] != "fail" {
		t.Errorf("fail verdict not recorded: %v", a.FrontMatter)
	}

	writeVerifyIssue(t, vault, "issue.anvil.0003.late", "true", "# anvil:post-land\nfalse")
	rec, _, err = runVerify(t, vault, "issue.anvil.0003.late")
	if err != nil || rec.Verdict != "pass" || len(rec.Deferred) != 1 {
		t.Errorf("post-land red must defer, not fail: err=%v rec=%+v", err, rec)
	}
}

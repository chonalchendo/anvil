package cli

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestMilestoneStatus_CostRowsAndTotal(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeFixtureMilestone(t, vault, "demo.m1", "open")
	writeMilestoneIssue(t, vault, "demo.a", "resolved", "demo.m1")
	writeMilestoneIssue(t, vault, "demo.b", "resolved", "demo.m1")
	writeMilestoneIssue(t, vault, "demo.c", "open", "demo.m1")

	path := filepath.Join(vault, "70-issues", "demo.a.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	a.FrontMatter["cost_rounds"], a.FrontMatter["cost_diff"] = 2, 153
	a.FrontMatter["cost_files"], a.FrontMatter["cost_tokens"] = 2, 1010
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	execCmd(t, "reindex")

	var got struct {
		Issues []struct {
			ID, Status string
			Cost       *issueCost
		}
		CostTotal costTotal `json:"cost_total"`
	}
	out := execCmdJSON(t, "milestone", "status", "demo.m1", "--json")
	if err := jsonUnmarshal(t, strings.TrimSpace(out), &got); err != nil {
		t.Fatalf("json: %v\nout: %s", err, out)
	}
	if len(got.Issues) != 3 || got.Issues[0].Cost == nil || got.Issues[1].Cost != nil || got.Issues[2].Cost != nil {
		t.Fatalf("rows mismatch: %+v", got.Issues)
	}
	want := costTotal{issueCost{2, 153, 2, 1010}, 1, 3}
	if got.CostTotal != want {
		t.Fatalf("cost_total = %+v, want %+v", got.CostTotal, want)
	}
	if line := (milestoneIssueRow{ID: "i", Status: "resolved", Cost: got.Issues[0].Cost}).line(); line != "i\tresolved\t2r 153l 2f 1010t" {
		t.Fatalf("line = %q", line)
	}
	if line := (milestoneIssueRow{ID: "i", Status: "open"}).line(); line != "i\topen\t—" {
		t.Fatalf("line = %q", line)
	}
}

func TestSet_IntegerField_WritesInteger(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "cost_rounds", "3"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v\n%s", err, out.String())
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got, ok := a.FrontMatter["cost_rounds"].(int); !ok || got != 3 {
		t.Errorf("cost_rounds = %#v, want int 3", a.FrontMatter["cost_rounds"])
	}
}

func TestSet_IntegerField_NonNumericRefused(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "A")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"set", "issue", "foo.a", "cost_rounds", "many"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Fatalf("err = %v, want ErrSchemaInvalid", err)
	}
	for _, want := range []string{"type_mismatch", "cost_rounds", "integer"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

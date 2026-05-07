package build

import (
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestAssembleInstruction_BodyAndSuccessCriteriaAndFilesAndVerify(t *testing.T) {
	task := core.Task{
		ID:    "T1",
		Title: "Define adapter contract",
		Body: "## Task: T1\n\nDefine the AgentAdapter interface in internal/build/adapter.go.\n" +
			"Use accept-interfaces-return-structs per go-conventions.md.",
		SuccessCriteria: []string{
			"AgentAdapter interface defined",
			"RunRequest / RunResult value types defined",
		},
		Files:  []string{"internal/build/adapter.go"},
		Verify: "go build ./... && go test ./internal/build/...",
	}
	got := assembleInstruction(task)

	for _, want := range []string{
		"Define the AgentAdapter interface in internal/build/adapter.go.",
		"## Success criteria",
		"- AgentAdapter interface defined",
		"- RunRequest / RunResult value types defined",
		"## Files most relevant",
		"- internal/build/adapter.go",
		"## Verification",
		"go build ./... && go test ./internal/build/...",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestAssembleInstruction_OmitsEmptySections(t *testing.T) {
	task := core.Task{
		ID:     "T1",
		Title:  "Trivial",
		Body:   "## Task: T1\n\nDo the thing.",
		Verify: "true",
	}
	got := assembleInstruction(task)
	if strings.Contains(got, "## Success criteria") {
		t.Errorf("expected no success-criteria section, got:\n%s", got)
	}
	if strings.Contains(got, "## Files most relevant") {
		t.Errorf("expected no files section, got:\n%s", got)
	}
	if !strings.Contains(got, "## Verification") {
		t.Errorf("verify section must always appear (validator enforces non-empty Verify):\n%s", got)
	}
}

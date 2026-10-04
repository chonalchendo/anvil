package build

import (
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func TestAssembleInstruction_IsTrimmedBodyOnly(t *testing.T) {
	task := core.Task{ID: "T1", Body: "\n## Task: T1\n\nDo the thing.\n\n"}
	got := assembleInstruction(task)
	want := "## Task: T1\n\nDo the thing.\n"
	if got != want {
		t.Errorf("assembleInstruction = %q, want %q", got, want)
	}
}

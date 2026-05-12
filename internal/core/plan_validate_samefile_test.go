package core

import (
	"errors"
	"strings"
	"testing"
)

// ValidatePlan must surface same-file-in-wave conflicts with a structured
// SameFileInWaveError carrying code=same_file_in_wave, the conflicting tasks,
// and the file. The error must also unwrap to ErrPlanDAG so the CLI's
// exit-code mapping still works.
func TestValidatePlan_SameFileInWave_StructuredError(t *testing.T) {
	p := &Plan{
		Tasks: []Task{
			{
				ID: "demo.a", Verify: "go test ./...",
				Files: []string{"internal/cli/list.go"},
				Body:  strings.Repeat("x ", 110),
			},
			{
				ID: "demo.b", Verify: "go test ./...",
				Files: []string{"internal/cli/list.go"},
				Body:  strings.Repeat("y ", 110),
			},
		},
	}

	err := ValidatePlan(p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPlanDAG) {
		t.Errorf("err does not wrap ErrPlanDAG: %v", err)
	}
	var sfe *SameFileInWaveError
	if !errors.As(err, &sfe) {
		t.Fatalf("err is not *SameFileInWaveError: %v", err)
	}
	if sfe.Code != "same_file_in_wave" {
		t.Errorf("Code = %q, want same_file_in_wave", sfe.Code)
	}
	if sfe.File != "internal/cli/list.go" {
		t.Errorf("File = %q, want internal/cli/list.go", sfe.File)
	}
	if len(sfe.Tasks) != 2 || sfe.Tasks[0] != "demo.a" || sfe.Tasks[1] != "demo.b" {
		t.Errorf("Tasks = %v, want [demo.a demo.b]", sfe.Tasks)
	}
	msg := err.Error()
	for _, want := range []string{"same_file_in_wave", "demo.a", "demo.b", "internal/cli/list.go"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error text missing %q:\n%s", want, msg)
		}
	}
}

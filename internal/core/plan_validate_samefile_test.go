package core

import (
	"encoding/json"
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
	b, _ := json.Marshal(sfe)
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("json: %v", err)
	}
	if parsed["code"] != "same_file_in_wave" {
		t.Errorf("code = %v, want same_file_in_wave", parsed["code"])
	}
	if parsed["file"] != "internal/cli/list.go" {
		t.Errorf("file = %v, want internal/cli/list.go", parsed["file"])
	}
	tasks, _ := parsed["tasks"].([]any)
	if len(tasks) != 2 || tasks[0] != "demo.a" || tasks[1] != "demo.b" {
		t.Errorf("tasks = %v, want [demo.a demo.b]", tasks)
	}
	msg := err.Error()
	for _, want := range []string{"same_file_in_wave", "demo.a", "demo.b", "internal/cli/list.go"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error text missing %q:\n%s", want, msg)
		}
	}
}

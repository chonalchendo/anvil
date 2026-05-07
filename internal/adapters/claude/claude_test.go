package claude

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/build"
)

// shimPath returns the absolute path to a testdata shim script.
func shimPath(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestRun_HappyPath_ParsesUsageAndCost(t *testing.T) {
	a := New(shimPath(t, "shim_success.sh"))

	req := build.RunRequest{
		Model:       "claude-sonnet-4-6",
		Effort:      "medium",
		Instruction: "do the thing",
		Cwd:         t.TempDir(),
		Timeout:     30 * time.Second,
	}
	res, err := a.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if res.Tokens.Input != 100 || res.Tokens.Output != 50 ||
		res.Tokens.CacheWrite != 7 || res.Tokens.CacheRead != 3 {
		t.Errorf("Tokens = %+v, want {Input:100 Output:50 CacheRead:3 CacheWrite:7}", res.Tokens)
	}
	if res.CostUSD != 0.0042 {
		t.Errorf("CostUSD = %v, want 0.0042", res.CostUSD)
	}
	if res.AgentTime != 1234*time.Millisecond {
		t.Errorf("AgentTime = %v, want 1.234s", res.AgentTime)
	}
	if res.Duration <= 0 {
		t.Errorf("Duration must be > 0, got %v", res.Duration)
	}
}

func TestRun_NameIsClaudeCode(t *testing.T) {
	if got := New("").Name(); got != "claude-code" {
		t.Errorf("Name() = %q, want \"claude-code\"", got)
	}
}

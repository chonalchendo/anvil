package claude

import (
	"context"
	"errors"
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

func TestRun_NonZeroExit_ReturnsFailureExitCodeNoErr(t *testing.T) {
	a := New(shimPath(t, "shim_failure.sh"))
	req := build.RunRequest{
		Model: "claude-sonnet-4-6", Effort: "medium",
		Instruction: "x", Cwd: t.TempDir(), Timeout: 10 * time.Second,
	}
	res, err := a.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run err = %v, want nil (non-zero exit is reported via RunResult.ExitCode)", err)
	}
	if res.ExitCode == 0 {
		t.Errorf("ExitCode = 0, want non-zero")
	}
	// Partial usage parsed before failure must still be present.
	if res.Tokens.Input != 10 {
		t.Errorf("Tokens.Input = %d, want 10 (parsed from result event before exit 1)", res.Tokens.Input)
	}
}

func TestRun_QuotaPhrase_ReturnsErrQuotaExhausted(t *testing.T) {
	a := New(shimPath(t, "shim_quota.sh"))
	req := build.RunRequest{
		Model: "claude-sonnet-4-6", Effort: "medium",
		Instruction: "x", Cwd: t.TempDir(), Timeout: 10 * time.Second,
	}
	res, err := a.Run(context.Background(), req)
	if !errors.Is(err, build.ErrQuotaExhausted) {
		t.Fatalf("err = %v, want ErrQuotaExhausted", err)
	}
	// Partial result fields populated even on the quota path.
	if res.Duration <= 0 {
		t.Errorf("Duration must be > 0, got %v", res.Duration)
	}
}

func TestRun_CtxCancel_ReturnsCancelled(t *testing.T) {
	a := New(shimPath(t, "shim_slow.sh"))
	ctx, cancel := context.WithCancel(context.Background())
	req := build.RunRequest{
		Model: "claude-sonnet-4-6", Effort: "medium",
		Instruction: "x", Cwd: t.TempDir(), Timeout: 30 * time.Second,
	}
	// Cancel after 50ms — well before the shim's 30s sleep.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, err := a.Run(ctx, req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	// Adapter must always set Duration on the error path (spec §1).
	if res.Duration <= 0 {
		t.Errorf("Duration must be > 0 on cancel, got %v", res.Duration)
	}
	// Shim sleeps 30s; if we got here in <30s the cancel propagated.
	if res.Duration >= 30*time.Second {
		t.Errorf("Duration = %v >= 30s — cancel did not propagate to subprocess", res.Duration)
	}
}

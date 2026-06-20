package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/build"
)

// TestMain stubs keychainCredential absent for the whole package: the real
// security(1) shell-out can block on a macOS Keychain access prompt, which would
// hang the shim-based Run tests. The fallback test installs its own value.
func TestMain(m *testing.M) {
	keychainCredential = func() ([]byte, bool) { return nil, false }
	os.Exit(m.Run())
}

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
	if res.Diagnostic != "Done." {
		t.Errorf("Diagnostic = %q, want \"Done.\"", res.Diagnostic)
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
	if res.Diagnostic != "Tool failed." {
		t.Errorf("Diagnostic = %q, want \"Tool failed.\"", res.Diagnostic)
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
	if !strings.Contains(res.Diagnostic, "usage limit reached") {
		t.Errorf("Diagnostic = %q, want to contain \"usage limit reached\"", res.Diagnostic)
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

func TestSeedConfigDir_CopiesPresentEntries(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", src)

	if err := os.WriteFile(filepath.Join(src, ".credentials.json"), []byte(`{"token":"abc"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "settings.json"), []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := seedConfigDir(dst); err != nil {
		t.Fatalf("seedConfigDir: %v", err)
	}
	for _, name := range []string{".credentials.json", "settings.json"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Errorf("seeded dst missing %s: %v", name, err)
		}
	}
}

func TestSeedConfigDir_MissingSourceEntriesNotAnError(t *testing.T) {
	// Source dir exists but holds no .credentials.json, and no Keychain
	// credential is available (TestMain stubs it absent). A missing entry must
	// be skipped, not a hard error, leaving dst empty.
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	dst := t.TempDir()

	if err := seedConfigDir(dst); err != nil {
		t.Fatalf("seedConfigDir with empty source = %v, want nil", err)
	}
	if entries, _ := os.ReadDir(dst); len(entries) != 0 {
		t.Errorf("dst should be empty when source has nothing to seed, got %d entries", len(entries))
	}
}

func TestSeedConfigDir_KeychainFallbackWhenNoDiskCredential(t *testing.T) {
	// The macOS case: source config dir has no .credentials.json (OAuth token
	// is Keychain-backed). seedConfigDir must fall back to the Keychain and
	// write the blob into the per-spawn dir, or the isolated spawn no-ops with
	// "Not logged in" (anvil.0107).
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir()) // source: no .credentials.json on disk
	dst := t.TempDir()

	want := []byte(`{"token":"from-keychain"}`)
	orig := keychainCredential
	keychainCredential = func() ([]byte, bool) { return want, true }
	t.Cleanup(func() { keychainCredential = orig })

	if err := seedConfigDir(dst); err != nil {
		t.Fatalf("seedConfigDir: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, ".credentials.json")) //nolint:gosec // dst is our own t.TempDir; name is a hardcoded literal, not untrusted input
	if err != nil {
		t.Fatalf("expected .credentials.json seeded from keychain: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("seeded credential = %q, want %q", got, want)
	}
}

func TestSettingsJSON_BudgetAndSkills(t *testing.T) {
	req := build.RunRequest{Effort: "medium", Skills: []string{"alpha", "beta"}}
	got, err := settingsJSON(req)
	if err != nil {
		t.Fatalf("settingsJSON: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	et, _ := parsed["extendedThinking"].(map[string]any)
	if et["budget"] != "low" {
		t.Errorf("budget = %v, want \"low\" (medium → low)", et["budget"])
	}
	sk, _ := parsed["skills"].(map[string]any)
	allow, _ := sk["allow"].([]any)
	if len(allow) != 2 || allow[0] != "alpha" || allow[1] != "beta" {
		t.Errorf("skills.allow = %v, want [alpha beta]", allow)
	}
}

func TestSettingsJSON_NoSkills_OmitsAllow(t *testing.T) {
	req := build.RunRequest{Effort: "low"}
	got, err := settingsJSON(req)
	if err != nil {
		t.Fatalf("settingsJSON: %v", err)
	}
	if strings.Contains(got, `"allow"`) {
		t.Errorf("expected omitempty to drop allow when no skills, got %q", got)
	}
}

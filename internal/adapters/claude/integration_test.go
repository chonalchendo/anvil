//go:build integration

package claude

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chonalchendo/anvil/internal/build"
)

// TestRun_Integration_TranscriptSurvivesConfigDirCleanup spawns the real shim
// subprocess (not a mock) and asserts the full stream-json transcript is
// written to a durable path that outlives the per-spawn CLAUDE_CONFIG_DIR
// RemoveAll — the honest live leg per the contract's smoke topology
// (anvil.0161): a full `anvil build` would mutate the real vault and spend
// tokens, so this subprocess spawn is the exercise instead.
func TestRun_Integration_TranscriptSurvivesConfigDirCleanup(t *testing.T) {
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

	if res.TranscriptPath == "" {
		t.Fatal("TranscriptPath is empty, want a persisted transcript file")
	}
	if _, err := os.Stat(res.ConfigDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ConfigDir %s still exists, want removed post-run", res.ConfigDir)
	}

	got, err := os.ReadFile(res.TranscriptPath) //nolint:gosec // res.TranscriptPath is adapter-created, not untrusted input
	if err != nil {
		t.Fatalf("transcript did not survive config-dir cleanup: %v", err)
	}
	want := []string{`"type":"system"`, `"type":"assistant"`, `"type":"result"`}
	for _, w := range want {
		if !strings.Contains(string(got), w) {
			t.Errorf("transcript missing %s event; got %q", w, got)
		}
	}
}

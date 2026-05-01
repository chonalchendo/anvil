package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/state"
)

func setupThreadEnv(t *testing.T) string {
	t.Helper()
	vault := setupVault(t)
	t.Setenv("ANVIL_STATE_DIR", t.TempDir())
	dir := filepath.Join(vault, "60-threads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
type: thread
title: "Research ducklake"
created: 2026-05-01
updated: 2026-05-01
status: open
diataxis: explanation
tags: [type/thread]
---
`
	if err := os.WriteFile(filepath.Join(dir, "research-ducklake.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return vault
}

func TestThread_Activate_WritesState(t *testing.T) {
	setupThreadEnv(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"thread", "activate", "research-ducklake"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("activate: %v", err)
	}

	got, err := state.ReadActiveThread()
	if err != nil {
		t.Fatalf("ReadActiveThread: %v", err)
	}
	if got != "research-ducklake" {
		t.Errorf("active thread = %q, want %q", got, "research-ducklake")
	}
}

func TestThread_Activate_UnknownThread(t *testing.T) {
	setupThreadEnv(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"thread", "activate", "no-such-thread"})
	var errOut bytes.Buffer
	cmd.SetErr(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown thread")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "not found")
	}
}

func TestThread_Deactivate_ClearsState(t *testing.T) {
	setupThreadEnv(t)
	if err := state.WriteActiveThread("research-ducklake"); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"thread", "deactivate"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	got, err := state.ReadActiveThread()
	if err != nil {
		t.Fatalf("ReadActiveThread: %v", err)
	}
	if got != "" {
		t.Errorf("active thread = %q, want empty", got)
	}
}

func TestThread_Current(t *testing.T) {
	setupThreadEnv(t)

	// No active thread.
	cmd := newRootCmd()
	cmd.SetArgs([]string{"thread", "current"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("current (none): %v", err)
	}
	if !strings.Contains(out.String(), "(none)") {
		t.Errorf("output = %q, want to contain %q", out.String(), "(none)")
	}

	// With active thread.
	if err := state.WriteActiveThread("research-ducklake"); err != nil {
		t.Fatal(err)
	}
	cmd = newRootCmd()
	cmd.SetArgs([]string{"thread", "current"})
	out.Reset()
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("current (set): %v", err)
	}
	if !strings.Contains(out.String(), "research-ducklake") {
		t.Errorf("output = %q, want to contain %q", out.String(), "research-ducklake")
	}
}

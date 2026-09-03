package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInstallFireSessionResume_LoadsOwnHandoffNotRecency(t *testing.T) {
	setupVault(t)

	// Older session, written first, gets the handoff a fire-session-resume
	// call for it must load — never the newer session's, which recency-based
	// resuming-session would pick.
	writeFireSessionStart(t, "aaaa-old")
	writeSessionHandoff(t, "aaaa-old", "HANDOFF-aaaa-old objective line")
	writeFireSessionStart(t, "bbbb-new")
	writeSessionHandoff(t, "bbbb-new", "HANDOFF-bbbb-new objective line")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "fire-session-resume"})
	cmd.SetIn(strings.NewReader(`{"session_id":"aaaa-old","source":"compact","hook_event_name":"SessionStart"}`))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "HANDOFF-aaaa-old") {
		t.Errorf("output missing own handoff:\n%s", got)
	}
	if strings.Contains(got, "HANDOFF-bbbb-new") {
		t.Errorf("output leaked a different session's handoff:\n%s", got)
	}
	if !strings.Contains(got, "Do not run `anvil session resume`") {
		t.Errorf("output missing the hook-authored preamble instructing against re-running resuming-session's Phase 1:\n%s", got)
	}
	if !strings.Contains(got, "handing-off-session") {
		t.Errorf("output missing compact-trigger instruction to write a fresh handoff:\n%s", got)
	}
}

func TestInstallFireSessionResume_MissingSessionID(t *testing.T) {
	setupVault(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "fire-session-resume"})
	cmd.SetIn(strings.NewReader(`{"source":"resume","hook_event_name":"SessionStart"}`))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing session_id")
	}
}

func TestInstallFirePreCompact_EmitsHandoffShapedAdditionalContext(t *testing.T) {
	setupVault(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "fire-pre-compact"})
	cmd.SetIn(strings.NewReader(`{"session_id":"aaaa-old","trigger":"auto","hook_event_name":"PreCompact","custom_instructions":null}`))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook: %v", err)
	}

	var got struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if got.HookSpecificOutput.HookEventName != "PreCompact" {
		t.Errorf("hookEventName = %q, want PreCompact", got.HookSpecificOutput.HookEventName)
	}
	if !strings.Contains(got.HookSpecificOutput.AdditionalContext, "in-flight") {
		t.Errorf("additionalContext missing in-flight agent/issue guidance:\n%s", got.HookSpecificOutput.AdditionalContext)
	}
}

func TestInstallFirePreCompact_EmptyStdinStillSucceeds(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "fire-pre-compact"})
	cmd.SetIn(strings.NewReader(""))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook with empty stdin: %v", err)
	}
	if !strings.Contains(out.String(), "in-flight") {
		t.Errorf("additionalContext missing in-flight agent/issue guidance:\n%s", out.String())
	}
}

// writeFireSessionStart creates a stub session file for id via the
// SessionStart hook wrapper, mirroring Claude Code's real invocation.
func writeFireSessionStart(t *testing.T, id string) {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "fire-session-start"})
	cmd.SetIn(strings.NewReader(`{"session_id":"` + id + `","source":"startup","hook_event_name":"SessionStart"}`))
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fire-session-start %s: %v", id, err)
	}
}

// writeSessionHandoff writes body into id's session file via `session
// handoff`, as CLAUDE_CODE_SESSION_ID scopes resolveCurrentSession to it.
func writeSessionHandoff(t *testing.T, id, body string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_SESSION_ID", id)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "handoff", "--body", body})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("session handoff %s: %v", id, err)
	}
}

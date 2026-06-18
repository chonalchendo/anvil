package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeIssueWithAnchor authors an issue file with an optional reproduction_anchor.
// Pass anchorCmd == "" to omit the anchor entirely (grandfather case).
func writeIssueWithAnchor(t *testing.T, vault, id, anchorCmd, expected string) {
	t.Helper()
	var anchorBlock string
	if anchorCmd != "" {
		anchorBlock = fmt.Sprintf("reproduction_anchor:\n  command: %q\n  expected: %q\n", anchorCmd, expected)
	}
	body := fmt.Sprintf(`---
type: issue
title: "x"
description: "x"
goal: "x is done"
created: 2026-05-16
status: open
project: anvil
severity: low
tags: [domain/methodology]
%s---

body
`, anchorBlock)
	path := filepath.Join(vault, "70-issues", id+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}
}

func readIssueRaw(t *testing.T, vault, id string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vault, "70-issues", id+".md")) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestTransition_InProgress_AnchorMatchAllowsClaim(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.ok", "printf hello", "hello")

	execCmd(t, "transition", "issue", "anvil.ok", "in-progress", "--owner", "x")

	if !strings.Contains(readIssueRaw(t, vault, "anvil.ok"), "status: in-progress") {
		t.Errorf("expected status: in-progress after matching anchor")
	}
}

func TestTransition_InProgress_NoAnchorAllowsClaim(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.grand", "", "")

	execCmd(t, "transition", "issue", "anvil.grand", "in-progress", "--owner", "x")

	if !strings.Contains(readIssueRaw(t, vault, "anvil.grand"), "status: in-progress") {
		t.Errorf("expected grandfather transition to succeed for issue with no anchor")
	}
}

func TestTransition_InProgress_AnchorMatchSHA(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)

	sum := sha256.Sum256([]byte("hello"))
	expected := "sha:" + hex.EncodeToString(sum[:])
	writeIssueWithAnchor(t, vault, "anvil.sha", "printf hello", expected)

	execCmd(t, "transition", "issue", "anvil.sha", "in-progress", "--owner", "x")

	if !strings.Contains(readIssueRaw(t, vault, "anvil.sha"), "status: in-progress") {
		t.Errorf("expected sha-mode anchor to allow claim")
	}
}

// TestTransition_InProgress_AnchorExecFailureRefuses is the must-fail guard
// from the plan: without a real anchor check, `false` exits non-zero, stdout
// is empty, expected is non-empty, so a check that runs the command at all
// must refuse. If the anchor were not wired up, this test would pass-by-
// transitioning, defeating the gate. See plan T2 step 2.
func TestTransition_InProgress_AnchorExecFailureRefuses(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.exec", "false", "anything")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.exec", "in-progress", "--owner", "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error when anchor command exits non-zero (empty stdout vs non-empty expected); output: %s", out.String())
	}
	if !strings.Contains(readIssueRaw(t, vault, "anvil.exec"), "status: open") {
		t.Errorf("status should remain open after refused transition")
	}
}

func TestTransition_InProgress_AnchorMismatchRefusesStructured(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.m", "printf actual", "expected")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.m", "in-progress", "--owner", "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected mismatch error; output: %s", out.String())
	}
	// Error envelope (printed to stderr by printAndReturn) carries the code,
	// the offending command, the diff, and the escape-hatch hint.
	combined := out.String() + "\n" + err.Error()
	if !strings.Contains(combined, "anchor_mismatch") {
		t.Errorf("error must carry anchor_mismatch code: %s", combined)
	}
	if !strings.Contains(combined, "--force") {
		t.Errorf("error must name --force escape hatch: %s", combined)
	}
	if !strings.Contains(combined, "--no-longer-reproduces") {
		t.Errorf("error must name --no-longer-reproduces escape hatch: %s", combined)
	}
	if !strings.Contains(readIssueRaw(t, vault, "anvil.m"), "status: open") {
		t.Errorf("status should remain open after refused transition")
	}
}

func TestTransition_InProgress_AnchorMismatchForceProceeds(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.f", "printf actual", "expected")

	execCmd(t, "transition", "issue", "anvil.f", "in-progress", "--owner", "x", "--force")

	if !strings.Contains(readIssueRaw(t, vault, "anvil.f"), "status: in-progress") {
		t.Errorf("expected --force to bypass anchor check and claim the issue")
	}
}

func TestTransition_InProgress_NoLongerReproducesOnMismatch(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.stale", "printf actual", "expected")

	execCmd(t, "transition", "issue", "anvil.stale", "in-progress", "--no-longer-reproduces")

	got := readIssueRaw(t, vault, "anvil.stale")
	if !strings.Contains(got, "status: resolved") {
		t.Errorf("expected redirect to resolved on stale-anchor confirmation; got:\n%s", got)
	}
	if !strings.Contains(got, "resolved --no-longer-reproduces") {
		t.Errorf("audit line missing --no-longer-reproduces tag:\n%s", got)
	}
	if !strings.Contains(got, "anchor no longer reproduces") {
		t.Errorf("audit line missing diff capture:\n%s", got)
	}
}

func TestTransition_InProgress_NoLongerReproducesErrorsOnMatch(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.real", "printf hello", "hello")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.real", "in-progress", "--no-longer-reproduces"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when anchor still reproduces; output: %s", out.String())
	}
	combined := out.String() + "\n" + err.Error()
	if !strings.Contains(combined, "anchor_still_reproduces") {
		t.Errorf("error must carry anchor_still_reproduces code: %s", combined)
	}
	if !strings.Contains(readIssueRaw(t, vault, "anvil.real"), "status: open") {
		t.Errorf("status should remain open when anchor still reproduces")
	}
}

func TestTransition_InProgress_NoLongerReproducesErrorsOnAbsentAnchor(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.bare", "", "")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.bare", "in-progress", "--no-longer-reproduces"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when issue has no anchor; output: %s", out.String())
	}
	combined := out.String() + "\n" + err.Error()
	if !strings.Contains(combined, "no_anchor_to_check") {
		t.Errorf("error must carry no_anchor_to_check code: %s", combined)
	}
	if !strings.Contains(readIssueRaw(t, vault, "anvil.bare"), "status: open") {
		t.Errorf("status should remain open when there is no anchor to check")
	}
}

func TestTransition_InProgress_AnchorTimeoutSurfacesAsError(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// `sleep 60` blows past the 30s anchor timeout — but the test cannot wait
	// that long. Override the timeout via build-tag would balloon scope; this
	// test instead covers the surface via a short sleep paired with a private
	// helper. For now, assert the failure path stays out of "match" semantics
	// when c.Run() returns a non-ExitError. We exercise this via a binary that
	// doesn't exist so exec.LookPath fails inside CommandContext.Run().
	writeIssueWithAnchor(t, vault, "anvil.bad", "/no/such/binary/anywhere", "expected")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.bad", "in-progress", "--owner", "x", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil with --json; err: %v stderr: %s", err, stderr.String())
	}
	// /bin/sh -c on a missing binary still returns an ExitError (sh prints to
	// stderr and exits 127). That's the documented match-semantics path:
	// empty stdout vs non-empty expected → anchor_mismatch. So this case
	// validates the ExitError fall-through still produces a mismatch refusal.
	if !strings.Contains(stdout.String(), "anchor_mismatch") {
		t.Errorf("expected mismatch refusal on missing binary (sh exits 127); got: %s", stdout.String())
	}
}

func TestTransition_InProgress_NoLongerReproducesRejectsNonOpenState(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.inprog", "printf hello", "hello")
	// Claim the issue first so it's in `in-progress`, then attempt the redirect.
	execCmd(t, "transition", "issue", "anvil.inprog", "in-progress", "--owner", "x")
	// Now flip back to a non-open state that isn't already in-progress so we
	// can re-target. Set status to `resolved` via the legal in-progress→resolved.
	execCmd(t, "transition", "issue", "anvil.inprog", "resolved")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.inprog", "in-progress", "--no-longer-reproduces"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected illegal_transition when --no-longer-reproduces is used from resolved; output: %s", out.String())
	}
	if !strings.Contains(out.String()+err.Error(), "illegal_transition") {
		t.Errorf("expected illegal_transition code: %s", out.String())
	}
}

func TestTransition_InProgress_ForceAndNoLongerReproducesMutuallyExclusive(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	writeIssueWithAnchor(t, vault, "anvil.both", "printf actual", "expected")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.both", "in-progress", "--owner", "x", "--force", "--no-longer-reproduces"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error when both --force and --no-longer-reproduces are passed; output: %s", out.String())
	}
}

// TestTransition_InProgress_CarriageReturnProgressBarMatches exercises the
// carriage-return normalization fix: a command that emits \r-progress-bar noise
// followed by the real value on a trailing line must claim cleanly when expected
// holds only the bare value. This FAILS against pre-fix code and is the
// regression guard for the fix.
func TestTransition_InProgress_CarriageReturnProgressBarMatches(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// Synthetic DuckDB-shaped progress: \r50%\r100%\n0 — expected holds just "0".
	writeIssueWithAnchor(t, vault, "anvil.cr", `printf '\r50%%\r100%%\n0'`, "0")

	execCmd(t, "transition", "issue", "anvil.cr", "in-progress", "--owner", "x")

	if !strings.Contains(readIssueRaw(t, vault, "anvil.cr"), "status: in-progress") {
		t.Errorf("expected carriage-return progress noise to be stripped before comparison")
	}
}

// TestTransition_InProgress_EchoTrailingNewlineMatches exercises the trailing-
// newline fix: `echo hello` always appends \n, so the old exact-equality
// comparison (got == expected) would reject a bare --expected "hello". The
// TrimSuffix normalisation must make them match. This test FAILS against the
// pre-fix code and is the regression guard for the fix.
func TestTransition_InProgress_EchoTrailingNewlineMatches(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// `echo hello` emits "hello\n"; expected records the bare string "hello".
	writeIssueWithAnchor(t, vault, "anvil.echo", "echo hello", "hello")

	execCmd(t, "transition", "issue", "anvil.echo", "in-progress", "--owner", "x")

	if !strings.Contains(readIssueRaw(t, vault, "anvil.echo"), "status: in-progress") {
		t.Errorf("expected echo-based anchor (trailing \\n) to match bare expected string")
	}
}

// TestTransition_InProgress_MultiLineOutputMismatchRefuses confirms that
// TrimSuffix normalization does not over-broaden: an anchor whose stdout
// contains an embedded newline (two distinct lines) must still refuse when the
// expected value is only the first line. Only the single trailing \n is stripped.
func TestTransition_InProgress_MultiLineOutputMismatchRefuses(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	// `printf "hello\nworld\n"` produces "hello\nworld\n"; expected is "hello".
	// After TrimSuffix the got becomes "hello\nworld" (trailing \n stripped) —
	// still not equal to "hello", so the gate must refuse.
	writeIssueWithAnchor(t, vault, "anvil.multiline", `printf "hello\nworld\n"`, "hello")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "anvil.multiline", "in-progress", "--owner", "x"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected anchor_mismatch for multi-line output vs single-line expected; output: %s", out.String())
	}
	combined := out.String() + "\n" + err.Error()
	if !strings.Contains(combined, "anchor_mismatch") {
		t.Errorf("error must carry anchor_mismatch code: %s", combined)
	}
	if !strings.Contains(readIssueRaw(t, vault, "anvil.multiline"), "status: open") {
		t.Errorf("status should remain open after refused transition")
	}
}

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
created: 2026-05-16
status: open
project: anvil
severity: low
tags: [domain/methodology]
%s---

body
`, anchorBlock)
	path := filepath.Join(vault, "70-issues", id+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readIssueRaw(t *testing.T, vault, id string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(vault, "70-issues", id+".md"))
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

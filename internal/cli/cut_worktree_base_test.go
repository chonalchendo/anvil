package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

func TestCutWorktreeBase(t *testing.T) {
	cut := func(branch, base string) error {
		_, _, err := cutWorktreeIfNeeded(&bytes.Buffer{}, "/repo/demo", "/wt/foo", branch, base)
		return err
	}

	t.Run("StartsAtOriginBase", func(t *testing.T) {
		s := stubSideFX(t)
		s.originBranches = map[string]bool{"stack-top": true}
		if err := cut("demo/foo", "stack-top"); err != nil {
			t.Fatal(err)
		}
		if len(s.addCalls) != 1 || s.addCalls[0].StartPoint != "origin/stack-top" || s.fetchCalls != 1 {
			t.Errorf("add=%+v fetches=%d; want start origin/stack-top after one fetch", s.addCalls, s.fetchCalls)
		}
	})

	t.Run("UnknownBaseRefused", func(t *testing.T) {
		s := stubSideFX(t)
		err := cut("demo/foo", "nope")
		wantCode(t, err, "base_branch_unknown")
		if len(s.addCalls) != 0 {
			t.Errorf("add calls = %+v, want none", s.addCalls)
		}
	})

	t.Run("FetchFailureRefused", func(t *testing.T) {
		s := stubSideFX(t)
		s.fetchErr = errors.New("offline")
		wantCode(t, cut("demo/foo", "stack-top"), "base_fetch_failed")
		if len(s.addCalls) != 0 {
			t.Errorf("add calls = %+v, want none", s.addCalls)
		}
	})

	t.Run("UnknownBaseRefusedEvenWhenIssueBranchExists", func(t *testing.T) {
		s := stubSideFX(t)
		s.originBranches = map[string]bool{"demo/foo": true}
		wantCode(t, cut("demo/foo", "nope"), "base_branch_unknown")
		if len(s.addCalls) != 0 {
			t.Errorf("add calls = %+v, want none", s.addCalls)
		}
	})

	t.Run("ExistingIssueBranchWinsOverBase", func(t *testing.T) {
		s := stubSideFX(t)
		s.originBranches = map[string]bool{"demo/foo": true, "stack-top": true}
		if err := cut("demo/foo", "stack-top"); err != nil {
			t.Fatal(err)
		}
		if got := s.addCalls[0].StartPoint; got != "origin/demo/foo" {
			t.Errorf("start = %q, want origin/demo/foo", got)
		}
	})

	t.Run("AbsentBaseKeepsOriginHEAD", func(t *testing.T) {
		s := stubSideFX(t)
		if err := cut("demo/foo", ""); err != nil {
			t.Fatal(err)
		}
		if got := s.addCalls[0].StartPoint; got != "origin/master" {
			t.Errorf("start = %q, want origin/master", got)
		}
	})
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var se *errfmt.Structured
	if !errors.As(err, &se) || se.Code != code {
		t.Fatalf("err = %v, want structured %s", err, code)
	}
}

// The refusal must fire on an already-in-progress issue, where the
// already-in-state short-circuit would otherwise swallow the stray flag.
func TestTransitionBaseWithoutCutWorktreeRefused(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	createDemoIssue(t)
	execCmd(t, "transition", "issue", "demo.foo", "in-progress", "--owner", "claude")

	stubSideFX(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"transition", "issue", "demo.foo", "in-progress", "--owner", "claude", "--base", "stack-top", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-nil error with --json; stdout: %s", stdout.String())
	}
	for _, want := range []string{"invalid_flag_for_transition", "--base"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q; got: %s", want, stdout.String())
		}
	}
}

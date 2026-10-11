package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
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
		if err := cut("demo/foo", "stack-top"); err == nil || len(s.addCalls) != 0 {
			t.Errorf("err=%v add=%+v; want refusal and no add", err, s.addCalls)
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

func TestCutWorktreeFlagRefusal(t *testing.T) {
	if err := cutWorktreeFlagRefusal(core.TypeIssue, "in-progress", false, "", "", "stack-top", 0); err == nil {
		t.Error("--base without --cut-worktree: want refusal")
	}
	if err := cutWorktreeFlagRefusal(core.TypeIssue, "in-progress", true, "", "", "stack-top", 0); err != nil {
		t.Errorf("--base with --cut-worktree: err = %v, want nil", err)
	}
}

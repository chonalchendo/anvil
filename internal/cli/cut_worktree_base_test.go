package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
)

func TestCutWorktreeBase(t *testing.T) {
	cut := func(s *sideFXStub, branch, base string) error {
		_, _, err := cutWorktreeIfNeeded(&bytes.Buffer{}, "/repo/demo", "/wt/foo", branch, base)
		return err
	}

	t.Run("StartsAtOriginBase", func(t *testing.T) {
		s := stubSideFX(t)
		s.originBranches = map[string]bool{"stack-top": true}
		if err := cut(s, "demo/foo", "stack-top"); err != nil {
			t.Fatal(err)
		}
		if len(s.addCalls) != 1 || s.addCalls[0].StartPoint != "origin/stack-top" || s.fetchCalls != 1 {
			t.Errorf("add=%+v fetches=%d; want start origin/stack-top after one fetch", s.addCalls, s.fetchCalls)
		}
	})

	t.Run("UnknownBaseRefused", func(t *testing.T) {
		s := stubSideFX(t)
		err := cut(s, "demo/foo", "nope")
		var se *errfmt.Structured
		if !errors.As(err, &se) {
			t.Fatalf("err = %v, want structured base_branch_unknown", err)
		}
		if len(s.addCalls) != 0 {
			t.Errorf("add calls = %+v, want none", s.addCalls)
		}
	})

	t.Run("FetchFailureRefused", func(t *testing.T) {
		s := stubSideFX(t)
		s.fetchErr = errors.New("offline")
		if err := cut(s, "demo/foo", "stack-top"); err == nil || len(s.addCalls) != 0 {
			t.Errorf("err=%v add=%+v; want refusal and no add", err, s.addCalls)
		}
	})

	t.Run("ExistingIssueBranchWinsOverBase", func(t *testing.T) {
		s := stubSideFX(t)
		s.originBranches = map[string]bool{"demo/foo": true, "stack-top": true}
		if err := cut(s, "demo/foo", "stack-top"); err != nil {
			t.Fatal(err)
		}
		if got := s.addCalls[0].StartPoint; got != "origin/demo/foo" {
			t.Errorf("start = %q, want origin/demo/foo", got)
		}
	})

	t.Run("AbsentBaseKeepsOriginHEAD", func(t *testing.T) {
		s := stubSideFX(t)
		if err := cut(s, "demo/foo", ""); err != nil {
			t.Fatal(err)
		}
		if got := s.addCalls[0].StartPoint; got != "origin/master" {
			t.Errorf("start = %q, want origin/master", got)
		}
	})
}

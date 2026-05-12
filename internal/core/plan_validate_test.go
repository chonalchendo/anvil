package core

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func mustLoad(t *testing.T, body string) *Plan {
	t.Helper()
	p, err := LoadPlan(writePlanFile(t, body))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestValidatePlan_HappyPath(t *testing.T) {
	if err := ValidatePlan(mustLoad(t, planFixture)); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidatePlan_DanglingDep(t *testing.T) {
	body := strings.Replace(planFixture, "depends_on: [T1]", "depends_on: [T9]", 1)
	err := ValidatePlan(mustLoad(t, body))
	if !errors.Is(err, ErrPlanDAG) {
		t.Errorf("err = %v, want ErrPlanDAG", err)
	}
}

func TestValidatePlan_Cycle(t *testing.T) {
	// Make T1 depend on T2 in addition to T2->T1; that's a cycle.
	body := strings.Replace(planFixture,
		"    depends_on: []\n    verify: \"go test ./...\"",
		"    depends_on: [T2]\n    verify: \"go test ./...\"", 1)
	err := ValidatePlan(mustLoad(t, body))
	if !errors.Is(err, ErrPlanDAG) {
		t.Errorf("err = %v, want ErrPlanDAG", err)
	}
}

func TestValidatePlan_EmptyVerify(t *testing.T) {
	body := strings.Replace(planFixture, "verify: \"go test ./...\"", "verify: \"\"", 1)
	p, err := LoadPlan(writePlanFile(t, body))
	if err != nil {
		t.Skipf("yaml rejected empty verify: %v", err)
	}
	if err := ValidatePlan(p); !errors.Is(err, ErrPlanTDD) {
		t.Errorf("err = %v, want ErrPlanTDD", err)
	}
}

func TestValidatePlan_NoopVerify_Rejected(t *testing.T) {
	noops := make([]string, 0, len(noopVerifies))
	for k := range noopVerifies {
		noops = append(noops, k)
	}
	slices.Sort(noops)
	for _, noop := range noops {
		t.Run(noop, func(t *testing.T) {
			body := strings.Replace(planFixture,
				"verify: \"go test ./...\"",
				"verify: \""+noop+"\"", 1)
			p, err := LoadPlan(writePlanFile(t, body))
			if err != nil {
				t.Fatalf("LoadPlan: %v", err)
			}
			err = ValidatePlan(p)
			if !errors.Is(err, ErrPlanTDD) {
				t.Errorf("err = %v, want ErrPlanTDD for no-op verify %q", err, noop)
			}
		})
	}
}

func TestValidatePlan_NoopVerify_WhitespaceCanonicalized(t *testing.T) {
	// Inputs with non-canonical whitespace must still be detected as no-ops.
	for _, noop := range []string{"exit   0", "exit\t0", "  :  "} {
		t.Run(noop, func(t *testing.T) {
			body := strings.Replace(planFixture,
				"verify: \"go test ./...\"",
				"verify: \""+noop+"\"", 1)
			p, err := LoadPlan(writePlanFile(t, body))
			if err != nil {
				t.Fatalf("LoadPlan: %v", err)
			}
			if err := ValidatePlan(p); !errors.Is(err, ErrPlanTDD) {
				t.Errorf("err = %v, want ErrPlanTDD for whitespace-padded no-op %q", err, noop)
			}
		})
	}
}

func TestValidatePlan_MissingBodySection(t *testing.T) {
	body := strings.Split(planFixture, "## Task: T2")[0]
	err := ValidatePlan(mustLoad(t, body))
	if !errors.Is(err, ErrPlanTDD) {
		t.Errorf("err = %v, want ErrPlanTDD", err)
	}
}

func TestValidatePlan_FilesAbsolutePath_Rejected(t *testing.T) {
	body := strings.Replace(planFixture,
		"files: [\"a.go\", \"a_test.go\"]",
		"files: [\"/abs/a.go\", \"a_test.go\"]", 1)
	err := ValidatePlan(mustLoad(t, body))
	if !errors.Is(err, ErrPlanDAG) {
		t.Errorf("err = %v, want ErrPlanDAG (path validity)", err)
	}
}

func TestValidatePlan_IntraWaveFileOverlap_Rejected(t *testing.T) {
	body := strings.Replace(planFixture,
		"files: [\"b.go\", \"b_test.go\"]\n    depends_on: [T1]",
		"files: [\"a.go\", \"b_test.go\"]\n    depends_on: []", 1)
	err := ValidatePlan(mustLoad(t, body))
	if !errors.Is(err, ErrPlanDAG) {
		t.Errorf("err = %v, want ErrPlanDAG (file overlap)", err)
	}
}

package cli

import (
	"strings"
	"testing"
)

// TestDoctorDuplicateOrdinal covers the observed collision (two issues minted
// as issue.mentat.0291), the legacy unnumbered issue that has no ordinal to
// collide, and the clean per-project case.
func TestDoctorDuplicateOrdinal(t *testing.T) {
	paths := []string{
		"/v/70-issues/mentat.0291.enterprise-value-strikes.md",
		"/v/70-issues/mentat.0291.run-verification-false-passes.md",
		"/v/70-issues/mentat.0292.only-one.md",
		"/v/70-issues/anvil.0291.different-project.md",
		"/v/70-issues/mentat.legacy-unnumbered.md",
	}
	findings := checkDuplicateOrdinals(paths)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Kind != "duplicate-ordinal" || f.ID != "issue.mentat.0291" {
		t.Errorf("finding = (%q, %q), want (duplicate-ordinal, issue.mentat.0291)", f.Kind, f.ID)
	}
	for _, want := range []string{
		"issue.mentat.0291.enterprise-value-strikes",
		"issue.mentat.0291.run-verification-false-passes",
	} {
		if !strings.Contains(f.Evidence, want) {
			t.Errorf("evidence %q missing %q", f.Evidence, want)
		}
	}
}

func TestDoctorDuplicateOrdinal_CleanVault(t *testing.T) {
	paths := []string{"/v/70-issues/mentat.0001.a.md", "/v/70-issues/mentat.0002.b.md"}
	if f := checkDuplicateOrdinals(paths); len(f) != 0 {
		t.Errorf("clean vault yielded %d findings: %+v", len(f), f)
	}
}

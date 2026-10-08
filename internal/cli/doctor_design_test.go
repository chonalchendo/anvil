package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func seedProductDesign(t *testing.T, vault, body string) {
	t.Helper()
	a := &core.Artifact{
		Path: filepath.Join(vault, "05-product-designs", "demo.md"),
		FrontMatter: map[string]any{
			"type": "product-design", "title": "Demo", "description": "d",
			"created": "2026-01-01", "updated": "2026-01-01", "project": "demo",
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorCandidateMilestoneDone(t *testing.T) {
	list := "## Milestones\n\n- [[milestone.demo.loop]]\n"
	cases := []struct {
		name   string
		status string
		pd     string // empty means no product design
		want   int
	}{
		{"done milestone still listed", "done", list, 1},
		{"in-progress milestone listed", "in-progress", list, 0},
		{"done milestone not listed", "done", "## Milestones\n\n- none\n", 0},
		{"no Milestones section", "done", "## Other\n\n- [[milestone.demo.loop]]\n", 0},
		{"no product design", "done", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			writeFixtureMilestone(t, vault, "demo.loop", tc.status)
			if tc.pd != "" {
				seedProductDesign(t, vault, tc.pd)
			}
			got, err := checkCandidateMilestoneDone(&core.Vault{Root: vault}, "demo")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("findings = %+v, want %d", got, tc.want)
			}
			if tc.want == 1 {
				f := got[0]
				if f.Kind != "candidate-milestone-done" || f.ID != "milestone.demo.loop" ||
					!strings.Contains(f.Evidence, "product-design.demo ## Milestones links milestone.demo.loop") {
					t.Errorf("finding = %+v", f)
				}
			}
		})
	}
}

func TestDoctorCandidateMilestoneDoneEmptyProject(t *testing.T) {
	got, err := checkCandidateMilestoneDone(&core.Vault{Root: setupVault(t)}, "")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

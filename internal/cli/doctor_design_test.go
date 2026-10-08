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
	cases := []struct {
		name   string
		title  string // milestone title; empty means "Close the loop"
		status string
		done   string
		pd     string // empty means no product design
		want   string // evidence suffix; empty means no finding
	}{
		{"linked, done and dated", "", "done", "2026-10-08", "## Milestones\n\n- [[milestone.demo.loop]]\n", "(done 2026-10-08)"},
		{"linked, done and undated", "", "done", "", "## Milestones\n\n- [[milestone.demo.loop]]\n", "(done date not stamped)"},
		{"listed by title", "", "done", "2026-10-08", "## Milestones\n\n- Close the loop. Why: x\n", "(done 2026-10-08)"},
		{"title with colon, mixed case", "", "done", "", "## Milestones\n\n- CLOSE THE LOOP: why\n", "(done date not stamped)"},
		{"title containing a dot", "v0.1 polish — dogfood findings", "done", "", "## Milestones\n\n- v0.1 polish — dogfood findings.\n", "(done date not stamped)"},
		{"listed title is a strict prefix of a longer one", "", "done", "", "## Milestones\n\n- Close the loop properly.\n", ""},
		{"title is a prefix of a dotted bullet", "v0", "done", "", "## Milestones\n\n- v0.2: next\n", ""},
		{"in-progress milestone listed", "", "in-progress", "", "## Milestones\n\n- [[milestone.demo.loop]]\n", ""},
		{"done milestone not listed", "", "done", "", "## Milestones\n\n- none\n", ""},
		{"nested evidence link", "", "done", "", "## Milestones\n\n- Other candidate.\n  - Why now: [[milestone.demo.loop]] shipped.\n", ""},
		{"no Milestones section", "", "done", "", "## Other\n\n- [[milestone.demo.loop]]\n", ""},
		{"no product design", "", "done", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			writeFixtureMilestone(t, vault, "demo.loop", tc.status)
			title := tc.title
			if title == "" {
				title = "Close the loop"
			}
			setMilestoneFields(t, vault, "demo.loop", title, tc.done)
			if tc.pd != "" {
				seedProductDesign(t, vault, tc.pd)
			}
			got, err := checkCandidateMilestoneDone(&core.Vault{Root: vault}, "demo")
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("findings = %+v, want none", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("findings = %+v, want 1", got)
			}
			f := got[0]
			if f.Kind != "candidate-milestone-done" || f.ID != "milestone.demo.loop" ||
				!strings.HasPrefix(f.Evidence, "product-design.demo ## Milestones lists milestone.demo.loop") ||
				!strings.HasSuffix(f.Evidence, tc.want) {
				t.Errorf("finding = %+v", f)
			}
		})
	}
}

func setMilestoneFields(t *testing.T, vault, id, title, done string) {
	t.Helper()
	path := filepath.Join(vault, "85-milestones", id+".md")
	m, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	m.FrontMatter["title"] = title
	if done != "" {
		m.FrontMatter["done"] = done
	}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorCandidateMilestoneDoneEmptyProject(t *testing.T) {
	got, err := checkCandidateMilestoneDone(&core.Vault{Root: setupVault(t)}, "")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

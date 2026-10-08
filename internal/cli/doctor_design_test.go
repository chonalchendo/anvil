package cli

import (
	"os"
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

func seedDesign(t *testing.T, vault, dir, typ, name, updated, body string) {
	t.Helper()
	a := &core.Artifact{
		Path: filepath.Join(vault, dir, name+".md"),
		FrontMatter: map[string]any{
			"type": typ, "title": name, "description": "d",
			"created": "2026-01-01", "updated": updated, "project": "demo",
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorDesignUntouchedAfterMilestone(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		done    string
		updated string
		want    bool
	}{
		{"design older than done", "done", "2026-10-08", "2026-01-01", true},
		{"design updated on done date", "done", "2026-10-08", "2026-10-08", false},
		{"design updated after done", "done", "2026-10-08", "2026-10-09", false},
		{"done milestone without date", "done", "", "2026-01-01", false},
		{"milestone not done", "in-progress", "", "2026-01-01", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			writeFixtureMilestone(t, vault, "demo.loop", tc.status)
			seedProductDesign(t, vault, "## Milestones\n")
			setMilestoneFields(t, vault, "demo.loop", "Loop", tc.done)
			m, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", "demo.loop.md"))
			if err != nil {
				t.Fatal(err)
			}
			m.FrontMatter["related"] = []any{"[[product-design.demo]]", "[[milestone.demo.other]]"}
			if err := m.Save(); err != nil {
				t.Fatal(err)
			}
			pd, err := core.LoadArtifact(filepath.Join(vault, "05-product-designs", "demo.md"))
			if err != nil {
				t.Fatal(err)
			}
			pd.FrontMatter["updated"] = tc.updated
			if err := pd.Save(); err != nil {
				t.Fatal(err)
			}
			got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: vault}, "demo")
			if err != nil {
				t.Fatal(err)
			}
			if !tc.want {
				if len(got) != 0 {
					t.Fatalf("findings = %+v, want none", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("findings = %+v, want 1", got)
			}
			f := got[0]
			if f.Kind != "design-untouched-after-milestone" || f.ID != "product-design.demo" ||
				f.Evidence != "updated 2026-01-01; untouched after 1 done milestone(s), latest milestone.demo.loop done 2026-10-08" {
				t.Errorf("finding = %+v", f)
			}
		})
	}
	t.Run("related names one design twice: one finding only", func(t *testing.T) {
		vault := setupVault(t)
		seedProductDesign(t, vault, "## Milestones\n")
		seedDoneMilestone(t, vault, "demo.loop", "2026-10-08")
		m, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", "demo.loop.md"))
		if err != nil {
			t.Fatal(err)
		}
		m.FrontMatter["related"] = []any{"[[product-design.demo]]", "product-design.demo", "[[milestone.demo.other]]"}
		if err := m.Save(); err != nil {
			t.Fatal(err)
		}
		pd, err := core.LoadArtifact(filepath.Join(vault, "05-product-designs", "demo.md"))
		if err != nil {
			t.Fatal(err)
		}
		pd.FrontMatter["updated"] = "2026-01-01"
		if err := pd.Save(); err != nil {
			t.Fatal(err)
		}
		got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: vault}, "demo")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !strings.Contains(got[0].Evidence, "1 done milestone(s)") {
			t.Fatalf("findings = %+v, want 1 with 1 done milestone(s)", got)
		}
	})
	t.Run("spine slot alone", func(t *testing.T) {
		vault := setupVault(t)
		seedProductDesign(t, vault, "## Milestones\n")
		seedDoneMilestone(t, vault, "demo.loop", "2026-10-08")
		m, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", "demo.loop.md"))
		if err != nil {
			t.Fatal(err)
		}
		delete(m.FrontMatter, "related")
		m.FrontMatter["product_design"] = "[[product-design.demo]]"
		if err := m.Save(); err != nil {
			t.Fatal(err)
		}
		pd, err := core.LoadArtifact(filepath.Join(vault, "05-product-designs", "demo.md"))
		if err != nil {
			t.Fatal(err)
		}
		pd.FrontMatter["updated"] = "2026-01-01"
		if err := pd.Save(); err != nil {
			t.Fatal(err)
		}
		got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: vault}, "demo")
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
}

func TestDoctorDesignUntouchedAfterMilestoneEmptyVault(t *testing.T) {
	got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: setupVault(t)}, "demo")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func seedDoneMilestone(t *testing.T, vault, id, done string) {
	t.Helper()
	writeFixtureMilestone(t, vault, id, "done")
	setMilestoneFields(t, vault, id, id, done)
	m, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	m.FrontMatter["related"] = []any{"[[product-design.demo]]"}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorDesignUntouchedAfterMilestoneScope(t *testing.T) {
	t.Run("other project's done milestone skipped", func(t *testing.T) {
		vault := setupVault(t)
		seedProductDesign(t, vault, "## Milestones\n")
		seedDoneMilestone(t, vault, "other.loop", "2026-10-08")
		got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: vault}, "demo")
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("empty slug examines nothing", func(t *testing.T) {
		vault := setupVault(t)
		seedProductDesign(t, vault, "## Milestones\n")
		seedDoneMilestone(t, vault, "demo.loop", "2026-10-08")
		got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: vault}, "")
		if err != nil || len(got) != 0 {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("two milestones one design give one finding", func(t *testing.T) {
		vault := setupVault(t)
		seedProductDesign(t, vault, "## Milestones\n")
		seedDoneMilestone(t, vault, "demo.early", "2026-10-05")
		seedDoneMilestone(t, vault, "demo.late", "2026-10-08")
		pd, err := core.LoadArtifact(filepath.Join(vault, "05-product-designs", "demo.md"))
		if err != nil {
			t.Fatal(err)
		}
		pd.FrontMatter["updated"] = "2026-01-01"
		if err := pd.Save(); err != nil {
			t.Fatal(err)
		}
		got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: vault}, "demo")
		if err != nil {
			t.Fatal(err)
		}
		want := "updated 2026-01-01; untouched after 2 done milestone(s), latest milestone.demo.late done 2026-10-08"
		if len(got) != 1 || got[0].ID != "product-design.demo" || got[0].Evidence != want {
			t.Fatalf("findings = %+v", got)
		}
	})
}

func TestDoctorDesignCodeRefMissing(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "internal"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "internal", "here.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		body string
		root string
		want []string // missing paths reported
	}{
		{"missing path", "see `internal/gone.go`\n", repo, []string{"internal/gone.go"}},
		{"existing path", "see `internal/here.go`\n", repo, nil},
		{"duplicate reported once", "`a/b.go` and `a/b.go`\n", repo, []string{"a/b.go"}},
		{"not path-shaped", "`here.go` `internal/*.go` `internal/<x>.go` `internal/{a}.go` `a/b` `go test ./...`\n", repo, nil},
		{"empty root", "see `internal/gone.go`\n", "", nil},
		{"no body refs", "plain prose\n", repo, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			seedDesign(t, vault, "05-product-designs", "product-design", "demo", "2026-01-01", tc.body)
			got, err := checkDesignCodeRefMissing(&core.Vault{Root: vault}, "demo", tc.root)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("findings = %+v, want %v", got, tc.want)
			}
			for i, f := range got {
				if f.Kind != "design-code-ref-missing" || f.ID != "product-design.demo" || f.Evidence != tc.want[i]+" not in "+repo {
					t.Errorf("finding = %+v", f)
				}
			}
		})
	}
	t.Run("other project's design skipped", func(t *testing.T) {
		vault := setupVault(t)
		seedDesign(t, vault, "05-product-designs", "product-design", "demo", "2026-01-01", "`a/b.go`\n")
		if got, _ := checkDesignCodeRefMissing(&core.Vault{Root: vault}, "other", repo); len(got) != 0 {
			t.Fatalf("findings = %+v, want none", got)
		}
	})
	t.Run("unreadable design dir errors", func(t *testing.T) {
		vault := setupVault(t)
		dir := filepath.Join(vault, "05-product-designs")
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := checkDesignCodeRefMissing(&core.Vault{Root: vault}, "demo", repo); err == nil {
			t.Fatal("want error for an unreadable design dir")
		}
	})
}

func TestDoctorDesignUntouchedSharedSlug(t *testing.T) {
	vault := setupVault(t)
	writeFixtureMilestone(t, vault, "demo.loop", "done")
	setMilestoneFields(t, vault, "demo.loop", "Loop", "2026-10-08")
	seedProductDesign(t, vault, "## Milestones\n")
	seedDesign(t, vault, "06-system-designs", "system-design", "demo", "2026-02-02", "x\n")
	m, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", "demo.loop.md"))
	if err != nil {
		t.Fatal(err)
	}
	m.FrontMatter["related"] = []any{"[[product-design.demo]]", "[[system-design.demo]]"}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	got, err := checkDesignUntouchedAfterMilestone(&core.Vault{Root: vault}, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "product-design.demo" || got[1].ID != "system-design.demo" ||
		!strings.HasPrefix(got[0].Evidence, "updated 2026-01-01; untouched after 1 ") ||
		!strings.HasPrefix(got[1].Evidence, "updated 2026-02-02; untouched after 1 ") {
		t.Fatalf("findings = %+v, want one per design", got)
	}
}

func TestDoctorDesignDriftRootFollowsBinding(t *testing.T) {
	bound, cwdRepo := t.TempDir(), t.TempDir()
	for _, f := range []string{filepath.Join(bound, "internal", "here.go"), filepath.Join(cwdRepo, "internal", "there.go")} {
		if err := os.MkdirAll(filepath.Dir(f), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	t.Setenv("ANVIL_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "projects", "demo"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "projects", "demo", ".binding"), []byte(bound+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := gitToplevelFn
	t.Cleanup(func() { gitToplevelFn = prev })
	gitToplevelFn = func() (string, error) { return cwdRepo, nil }

	vault := setupVault(t)
	seedProductDesign(t, vault, "`internal/there.go` `internal/here.go`\n")
	got, err := checkDesignDrift(&core.Vault{Root: vault}, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Evidence != "internal/there.go not in "+bound {
		t.Fatalf("findings = %+v, want only internal/there.go missing from the bound repo", got)
	}

	t.Run("no binding falls back to cwd repo", func(t *testing.T) {
		if err := os.Remove(filepath.Join(home, "projects", "demo", ".binding")); err != nil {
			t.Fatal(err)
		}
		got, err := checkDesignDrift(&core.Vault{Root: vault}, "demo")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].Evidence != "internal/here.go not in "+cwdRepo {
			t.Fatalf("findings = %+v", got)
		}
	})
}

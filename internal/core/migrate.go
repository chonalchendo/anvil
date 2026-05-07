package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateVault rewrites frontmatter and merges legacy operational issue files
// to match the redesigned schemas. Idempotent.
func MigrateVault(v *Vault) error {
	for _, t := range AllTypes {
		dir := filepath.Join(v.Root, t.Dir())
		if err := walkTypeDir(t, dir); err != nil {
			return err
		}
	}
	if err := walkDesignDocs(v); err != nil {
		return err
	}
	return mergeOperationalIssues(v)
}

func walkTypeDir(t Type, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		a, err := LoadArtifact(path)
		if err != nil {
			return fmt.Errorf("load %s: %w", path, err)
		}
		if migrateFrontMatter(cutsFor(t), a) {
			if err := a.Save(); err != nil {
				return fmt.Errorf("save %s: %w", path, err)
			}
		}
	}
	return nil
}

// walkDesignDocs walks 05-projects/*/{product,system}-design.md applying
// design-doc-specific cuts (these aren't directory-scoped Types).
func walkDesignDocs(v *Vault) error {
	base := filepath.Join(v.Root, "05-projects")
	projects, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, p := range projects {
		if !p.IsDir() {
			continue
		}
		for name, cuts := range designDocCuts {
			path := filepath.Join(base, p.Name(), name+".md")
			if _, err := os.Stat(path); err != nil {
				continue
			}
			a, err := LoadArtifact(path)
			if err != nil {
				return fmt.Errorf("load %s: %w", path, err)
			}
			if migrateFrontMatter(cuts, a) {
				if err := a.Save(); err != nil {
					return fmt.Errorf("save %s: %w", path, err)
				}
			}
		}
	}
	return nil
}

func migrateFrontMatter(cuts fieldCuts, a *Artifact) bool {
	pruned := false
	var bodyAdds strings.Builder
	for _, field := range cuts.toBody {
		v, ok := a.FrontMatter[field]
		if !ok {
			continue
		}
		fmt.Fprintf(&bodyAdds, "\n## %s\n\n%s\n", titleCase(field), formatValue(v))
		delete(a.FrontMatter, field)
		pruned = true
	}
	for _, field := range cuts.drop {
		if _, ok := a.FrontMatter[field]; ok {
			delete(a.FrontMatter, field)
			pruned = true
		}
	}
	for old, newName := range cuts.rename {
		if v, ok := a.FrontMatter[old]; ok {
			a.FrontMatter[newName] = v
			delete(a.FrontMatter, old)
			pruned = true
		}
	}
	if bodyAdds.Len() > 0 {
		a.Body += bodyAdds.String()
	}
	return pruned
}

type fieldCuts struct {
	toBody []string
	drop   []string
	rename map[string]string
}

func cutsFor(t Type) fieldCuts {
	return typeCuts[t]
}

var typeCuts = map[Type]fieldCuts{
	TypeMilestone: {
		toBody: []string{"objectives", "risks"},
		drop:   []string{"target_date", "horizon", "ordinal", "predecessors", "successors", "plans", "issues"},
	},
	TypeIssue: {
		drop: []string{"learnings", "discovered_in", "promoted_from"},
	},
	TypePlan: {
		drop: []string{"milestone", "decisions", "non_goals", "references"},
	},
	TypeDecision: {
		toBody: []string{"decision-makers", "consulted", "informed", "evidence"},
		drop:   []string{"topic"},
	},
	TypeLearning: {
		toBody: []string{"sources"},
		drop:   []string{"parents"},
	},
	TypeThread: {
		toBody: []string{"question", "hypothesis", "resolution", "participants"},
		drop:   []string{"opened", "closed"},
	},
	TypeSweep: {
		toBody: []string{"target_repos", "prs", "metrics", "driver"},
	},
}

var designDocCuts = map[string]fieldCuts{
	"product-design": {
		toBody: []string{"target_users", "problem_statement", "success_metrics", "goals", "constraints", "appetite", "risks", "out_of_scope", "revisions"},
		drop:   []string{"milestones"},
	},
	"system-design": {
		toBody: []string{"tech_stack", "key_invariants", "risks", "boundary_diagrams", "revisions"},
		rename: map[string]string{"authorized_decisions": "authorized_by"},
	},
}

func titleCase(snake string) string {
	parts := strings.Split(snake, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func formatValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, item := range x {
			fmt.Fprintf(&b, "- %v\n", item)
		}
		return b.String()
	case map[string]any:
		var b strings.Builder
		for k, val := range x {
			fmt.Fprintf(&b, "- %s: %v\n", k, val)
		}
		return b.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func mergeOperationalIssues(v *Vault) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	base := filepath.Join(home, ".anvil", "projects")
	projects, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := filepath.Join(v.Root, TypeIssue.Dir())
	for _, p := range projects {
		issuesDir := filepath.Join(base, p.Name(), "issues")
		entries, err := os.ReadDir(issuesDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".md" {
				continue
			}
			src := filepath.Join(issuesDir, e.Name())
			tgt := filepath.Join(dst, p.Name()+"."+e.Name())
			if _, err := os.Stat(tgt); err == nil {
				fmt.Fprintf(os.Stderr, "migrate: conflict, skipping %s (target %s exists)\n", src, tgt)
				continue
			}
			b, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("read %s: %w", src, err)
			}
			if err := os.WriteFile(tgt, b, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", tgt, err)
			}
		}
	}
	return nil
}

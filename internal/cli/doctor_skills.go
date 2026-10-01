package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/installer"
)

const skillsRepairFix = "anvil install skills --force, then start a new session (Claude Code snapshots its skill registry before SessionStart hooks fire)"

// checkInstalledSkills flags skills-bundle drift that silently serves stale
// text to agents: an entry symlinked into a retired materialise dir or at a
// missing path, or a materialise dir whose content predates the running
// binary. Read-only; never repairs.
func checkInstalledSkills(skillsDir, mat, retired string) []doctorFinding {
	var findings []doctorFinding
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil // no skills dir: nothing installed to be stale
	}
	for _, e := range entries {
		p := filepath.Join(skillsDir, e.Name())
		dest, err := os.Readlink(p)
		if err != nil {
			continue // not a symlink: a copy or foreign entry, not this shape
		}
		if !filepath.IsAbs(dest) {
			dest = filepath.Join(skillsDir, dest)
		}
		var why string
		switch {
		case retired != "" && retired != mat && strings.HasPrefix(dest, retired+string(filepath.Separator)):
			why = "targets retired materialise dir " + retired
		default:
			if _, err := os.Stat(dest); err != nil {
				why = "targets missing path " + dest
			}
		}
		if why == "" {
			continue
		}
		findings = append(findings, doctorFinding{
			Kind:     "stale-skills-entry",
			ID:       e.Name(),
			Evidence: fmt.Sprintf("installed skill %s %s", p, why),
			Fix:      skillsRepairFix,
		})
	}
	if _, err := os.Stat(mat); err == nil {
		if fresh, err := installer.SkillsAreFresh(skills.FS, mat); err == nil && !fresh {
			findings = append(findings, doctorFinding{
				Kind:     "stale-skills-bundle",
				ID:       filepath.Base(mat),
				Evidence: fmt.Sprintf("skills bundle at %s predates the running binary", mat),
				Fix:      skillsRepairFix,
			})
		}
	}
	return findings
}

// checkInstalledSkillsDefault resolves the claude target's dirs. Unresolvable
// config dirs skip the check, matching doctor's best-effort stance.
func checkInstalledSkillsDefault() []doctorFinding {
	skillsDir, err := resolveAnvilSkillsTarget("claude")
	if err != nil {
		return nil
	}
	mat, err := resolveSkillsMaterialiseDir("claude")
	if err != nil {
		return nil
	}
	retired := ""
	if home, err := os.UserHomeDir(); err == nil {
		retired = filepath.Join(home, ".anvil", "skills")
	}
	return checkInstalledSkills(skillsDir, mat, retired)
}

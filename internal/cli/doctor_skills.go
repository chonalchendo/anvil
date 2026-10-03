package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/chonalchendo/anvil/anvil/skills"
	"github.com/chonalchendo/anvil/internal/installer"
)

const skillsNewSession = ", then start a new session (Claude Code snapshots its skill registry before SessionStart hooks fire)"

// skillsRepairFix names the global binary because a worktree binary would
// reinstall from its own embed — the clobbering the single materialise dir
// exists to prevent.
func skillsRepairFix(target string) string {
	return fmt.Sprintf("run `anvil install skills --force --target %s` with the globally installed binary%s", target, skillsNewSession)
}

func bundleSkillNames() map[string]bool {
	names := map[string]bool{}
	entries, err := fs.ReadDir(skills.FS, ".")
	if err != nil {
		return names
	}
	for _, e := range entries {
		if e.IsDir() {
			names[e.Name()] = true
		}
	}
	return names
}

func under(path, dir string) bool {
	return dir != "" && strings.HasPrefix(path, filepath.Clean(dir)+string(filepath.Separator))
}

// checkInstalledSkills flags skills-bundle drift that silently serves stale
// text to agents: an anvil-owned entry symlinked into a retired materialise
// dir or at a missing path, or a materialise dir whose hash differs from the
// running binary's embedded bundle. A symlink is judged only when it points
// into the materialise or retired dir, or shares a bundled skill name;
// foreign links are the user's. Read-only; never repairs.
func checkInstalledSkills(target, skillsDir, mat, retired string) []doctorFinding {
	var findings []doctorFinding
	bundle := bundleSkillNames()
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
		dest = filepath.Clean(dest)
		inMat, inRetired := under(dest, mat), retired != mat && under(dest, retired)
		inBundle := bundle[e.Name()]
		if !inMat && !inRetired && !inBundle {
			continue
		}
		var why string
		if inRetired {
			why = "targets retired materialise dir " + retired
		} else if _, err := os.Stat(dest); err != nil {
			why = "targets missing path " + dest
		}
		if why == "" {
			continue
		}
		fix := skillsRepairFix(target)
		if inRetired && !inBundle {
			// install --force prunes only links into the live materialise dir.
			fix = "rm " + p
		}
		findings = append(findings, doctorFinding{
			Kind:     "stale-skills-entry",
			ID:       e.Name(),
			Evidence: fmt.Sprintf("installed skill %s %s", p, why),
			Fix:      fix,
		})
	}
	if _, err := os.Stat(mat); err == nil {
		if fresh, err := installer.SkillsAreFresh(skills.FS, mat); err == nil && !fresh {
			findings = append(findings, doctorFinding{
				Kind:     "stale-skills-bundle",
				ID:       target + ":" + filepath.Base(mat),
				Evidence: fmt.Sprintf("skills bundle at %s differs from the running binary's embedded bundle", mat),
				Fix:      skillsRepairFix(target),
			})
		}
	}
	return findings
}

// checkInstalledSkillsDefault checks every installable target whose dirs
// resolve and exist. Unresolvable config dirs skip, matching doctor's
// best-effort stance. ANVIL_SKILLS_DIR collapses all targets onto one
// materialise dir, so it is checked once.
func checkInstalledSkillsDefault() []doctorFinding {
	retired := ""
	if home, err := os.UserHomeDir(); err == nil {
		retired = filepath.Join(home, ".anvil", "skills")
	}
	var findings []doctorFinding
	seen := map[string]bool{}
	for _, t := range []string{"claude", "codex", "pi"} {
		skillsDir, err := resolveAnvilSkillsTarget(t)
		if err != nil {
			continue
		}
		mat, err := resolveSkillsMaterialiseDir(t)
		if err != nil || seen[mat] {
			continue
		}
		seen[mat] = true
		findings = append(findings, checkInstalledSkills(t, skillsDir, mat, retired)...)
	}
	return findings
}

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain owns the process-global state this package's commands reach for, so
// no test inherits the developer's machine (convention.testing § Hermeticity).
// Two concerns:
//
//   - Agent-CLI config roots are redirected into a throwaway HOME, so any test
//     that runs `anvil install skills` writes into a temp dir rather than the
//     developer's real ~/.claude/skills.
//   - gh is stubbed to report no PR, so every `transition issue ... resolved`
//     path is deterministic regardless of the host's gh auth state. Tests that
//     exercise the refusal path re-stub via stubGhPRList.
//
// Vault and project selectors are cleared here so no test inherits an ambient
// ANVIL_VAULT/ANVIL_PROJECT from the developer's shell; the per-test leak (the
// root command exports its --vault/--project flags into the environment) is
// neutralised by isolateRootEnv.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "anvil-cli-test-home")
	if err != nil {
		panic(err)
	}
	for k, v := range map[string]string{
		"HOME":                home,
		"CLAUDE_CONFIG_DIR":   filepath.Join(home, ".claude"),
		"CODEX_HOME":          filepath.Join(home, ".codex"),
		"PI_CODING_AGENT_DIR": filepath.Join(home, ".pi", "agent"),
		"ANVIL_SKILLS_DIR":    filepath.Join(home, ".anvil", "skills"),
		"ANVIL_VAULT":         "",
		"ANVIL_PROJECT":       "",
	} {
		_ = os.Setenv(k, v)
	}

	ghPRListFn = func(_ string) (string, error) { return "", nil }

	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

// isolateRootEnv pins ANVIL_VAULT and ANVIL_PROJECT to their current values for
// the rest of the test, so the root command's PersistentPreRunE — which exports
// --vault/--project into the process environment — cannot leak a selector into
// whichever test runs next. Called from every helper that executes a command;
// t.Setenv restores the pre-call value at test cleanup.
func isolateRootEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ANVIL_VAULT", os.Getenv("ANVIL_VAULT"))
	t.Setenv("ANVIL_PROJECT", os.Getenv("ANVIL_PROJECT"))
}

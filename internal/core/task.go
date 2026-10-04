package core

// Task is one unit of work the build engine schedules.
type Task struct {
	ID           string
	SkillsToLoad []string
	Body         string
	// Cwd is the per-task working directory the adapter spawns into. `anvil
	// build` sets it to the issue's cut worktree so the spawned worker lands its
	// PR on the deterministic branch the driver already holds; empty falls back
	// to the engine's global Cwd.
	Cwd string
	// Branch is the deterministic <project>/<slug> branch the driver cut for this
	// task. The engine's advance-gate confirms a PR opened on it before recording
	// success (anvil.0112); empty when no worktree was cut (dry-run, tests).
	Branch string
	// DisallowedTools is the per-phase tool wall the driver assigns; routed
	// verbatim into the spawn's RunRequest.DisallowedTools.
	DisallowedTools []string
}

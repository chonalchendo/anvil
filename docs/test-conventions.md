# Test Conventions

How to structure and isolate tests for the Anvil orchestrator.

- Framework: stdlib `testing` + `google/go-cmp` for diffs. `gotestsum` as the runner. No testify.
- Test files live alongside source: `internal/core/manifest.go` ↔ `internal/core/manifest_test.go`.
- Use `t.TempDir()` for isolated file operations — **never touch real `~/.claude/` or real vaults**. In `internal/cli` that redirection is process-wide: `TestMain` points `HOME`, `CLAUDE_CONFIG_DIR`, `CODEX_HOME` and `ANVIL_SKILLS_DIR` at a throwaway dir and clears `ANVIL_VAULT`/`ANVIL_PROJECT`.
- **Any test executing a root command with `--vault`/`--project` must pin the selector.** The root command's `PersistentPreRunE` exports those flags into the process environment, so an unpinned test leaks its vault into whichever test runs next — a failure that only reproduces under `-shuffle`. Call `isolateRootEnv(t)` (or route through `setupVault`/`runCmd`/`runArgs`/`runArgsJSON`/`execCmd`/`execCmdJSON`/`createIssueGetPath`, which all call it) so `t.Setenv` restores the value at cleanup. The `CLI test isolation` step in `.github/workflows/ci.yml` greps the test tree and fails the build on a func that misses this.
- Mock subprocess calls at the `os/exec.Cmd` boundary via a small interface in the adapter package; real-CLI tests live in `internal/adapters/integration_test.go` gated behind `// +build integration` (run via `just test-integration`).
- `testing/synctest` (Go 1.24+) is reserved for v0.2 wave-graph executor tests; not used in v0.1.

# Test Conventions

How to structure and isolate tests for the Anvil orchestrator.

- Framework: stdlib `testing` + `google/go-cmp` for diffs. `gotestsum` as the runner. No testify.
- Test files live alongside source: `internal/core/manifest.go` ↔ `internal/core/manifest_test.go`.
- Use `t.TempDir()` for isolated file operations — **never touch real `~/.claude/` or real vaults**.
- Mock subprocess calls at the `os/exec.Cmd` boundary via a small interface in the adapter package; real-CLI tests live in `internal/adapters/integration_test.go` gated behind `// +build integration` (run via `just test-integration`).
- `testing/synctest` (Go 1.24+) is reserved for v0.2 wave-graph executor tests; not used in v0.1.

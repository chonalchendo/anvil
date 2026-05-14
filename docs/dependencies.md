# Go ecosystem decisions (baked-in)

These choices are decided. Don't re-litigate without an ADR.

| Concern | Choice | Notes |
|---|---|---|
| CLI framework | `spf13/cobra` + `charmbracelet/fang` | Fang wraps the root command. Add `charmbracelet/huh` only when prompts are needed. |
| Layout | `cmd/anvil/main.go` + `internal/...` | Embeds colocated with consuming package. No top-level `assets/`. |
| Tests | stdlib `testing` + `google/go-cmp` + `gotestsum` | Skip testify. `testing/synctest` reserved for v0.2. |
| JSON Schema | `santhosh-tekuri/jsonschema/v6` | Deferred until schemas land. `xeipuuv/gojsonschema` is dead. |
| Templating | stdlib `text/template` + `//go:embed` | `go-sprout/sprout` only if Sprig-style helpers needed. Sprig itself is unmaintained. |
| Subprocess | stdlib `os/exec` + `bufio.Scanner` (8 MiB buffer) + `errgroup` + `cmd.Cancel`/`cmd.WaitDelay` | The 8 MiB buffer is non-negotiable. |
| SQLite | `modernc.org/sqlite` (pure Go) | Cross-compile + `go install` cleanliness; rejects cgo. |
| Logging | `log/slog` + `lmittmann/tint` (terminal) + JSON handler (debug file) via `samber/slog-multi` | Pretty in terminal, structured on disk. |
| Lint | `golangci-lint v2` with `linters.default: standard` + `errorlint`, `gocritic`, `revive`, `misspell`, `bodyclose`, `nilerr`, `contextcheck` | Configured in `.golangci.yml`; runs in CI and via `prek` (see below). Use `golangci-lint fmt` instead of separate gofumpt. `gosec` and `errcheck` deferred — re-enable per backlog item. |
| Release | `goreleaser` v2 + Cosign v3 (`--bundle`) + SLSA via `slsa-github-generator` + Syft SBOMs | `releasing.md` rewrite deferred. |
| Errors | stdlib only — `errors.New`, `fmt.Errorf("…: %w", err)`, `errors.Is/As/Join` | No `pkg/errors`, no `cockroachdb/errors`. |
| Tools | go.mod `tool` directive (Go 1.24+) in isolated `tool.go.mod` | Replaces `tools.go` blank-import pattern. |
| Config | `knadh/koanf` v2 (when config code lands) | Viper rejected. |
| Vuln | `govulncheck` as a tool dep | CI integration deferred. |

## Local commit gate (prek)

`prek` (Rust pre-commit drop-in) runs hygiene + Go-toolchain hooks at `git commit` time. Install once per machine:

    brew install j178/tap/prek
    just install-hooks   # writes .git/hooks/pre-commit

Hooks defined in `.pre-commit-config.yaml`. Toolchain pins (golangci-lint version) live in `tool.go.mod` so the local gate mirrors `.github/workflows/ci.yml`. `just check` runs the same fmt/lint/vet/test sequence without going through git hooks.

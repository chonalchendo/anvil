# Go Conventions

Idioms and rules for Go code in the Anvil orchestrator.

## Imports

- Standard import grouping: stdlib first, blank line, third-party, blank line, internal (`github.com/chonalchendo/anvil/internal/...`). `goimports` (run via `golangci-lint fmt`) enforces this.
- No dot imports (`import . "pkg"`). No blank imports outside `tools.go`-style files (and we don't have those — see `tool.go.mod`).
- Internal packages live under `internal/`. External consumers cannot import them by language rule.

## Type & API design

- Exported identifiers get a doc comment starting with the identifier name. One sentence is fine for trivial cases.
- Prefer small interfaces defined where consumed, not where implemented (Go idiom: "accept interfaces, return structs").
- Use `context.Context` as the first parameter on any function that does I/O, spawns goroutines, or might block. Never store contexts in structs.
- Use `time.Duration` for durations, `time.Time` for instants — never `int64` seconds.
- No `interface{}` / `any` in public APIs unless genuinely heterogeneous (and then comment why).

## Error handling

- Wrap with `%w`: `fmt.Errorf("loading config: %w", err)`. Never `fmt.Errorf("%v", err)` — destroys the chain.
- Check with `errors.Is` / `errors.As`. Never compare `err.Error()` strings.
- Define sentinel errors as package-level vars (`var ErrNotFound = errors.New("not found")`). Define typed errors as structs implementing `error`.
- Use `errors.Join` for multi-error aggregation; do not invent custom multi-error types.
- `golangci-lint`'s `errorlint` linter is enabled — it catches `err == ErrFoo` (wrong; use `errors.Is`) and unwrapped `%v` formats.

## Functions

- Max 5 parameters. Beyond that, group into a `Config` / `Options` struct.
- Use functional options (`func WithTimeout(d time.Duration) Option`) only when ≥3 optional params accumulate; otherwise a struct literal is fine.
- Boolean parameters get keyword-style call sites via struct fields, not positional booleans.

## Logging & output

- Structured logging via `log/slog`. `lmittmann/tint` for terminal output; JSON handler for debug-file output. Combine via `samber/slog-multi`.
- CLI output (user-facing) via cobra's `cmd.Println` / `cmd.PrintErrln`. Never `fmt.Println` directly in command code — it bypasses cobra's output redirection and breaks tests.
- No `log.Println` / `log.Fatal` in non-`main` packages. The standard `log` package writes to a global; `slog` is the contract.

## Concurrency

- Use `errgroup.Group` for goroutine fan-out with error propagation.
- Use `context.Context` for cancellation; never naked channels for cancel signals in new code.
- For subprocess management: `cmd.Cancel` (Go 1.20+) + `cmd.WaitDelay` for graceful-then-forceful shutdown.
- No `sync.Once` outside package init — if you need lazy init across goroutines, use a constructor.

## Subprocess gotchas (load-bearing)

- `bufio.Scanner`'s default `MaxScanTokenSize` is 64 KiB. Agent CLI tool-result lines exceed this. **Always** set `scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)` for an 8 MiB max line, or switch to `bufio.Reader.ReadBytes('\n')`. This is a documented invariant in `system-design.md`.
- Per-spawn `CLAUDE_CONFIG_DIR` and `CODEX_HOME` isolation is mandatory. Adapters set these; never rely on inherited env.

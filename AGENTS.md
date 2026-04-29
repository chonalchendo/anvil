# Anvil — Agent Conventions

Anvil is a methodology for AI-assisted development packaged as auto-loading SKILL.md files with a thin Go orchestrator. The architectural design lives in `docs/system-design.md`. Read it before making structural decisions.

This file captures *how to write code* for Anvil. The design doc captures *what Anvil is*.

These rules apply universally. Task-specific guidance lives in `docs/`.

## Behavioral Guardrails

### Think Before Coding

Don't assume. Don't hide confusion. Surface tradeoffs.

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

**YOU MUST stop and ask the user, not guess, when:**

- Adding a new dependency. Even a small one.
- Choosing between two genuinely different architectural approaches.
- Refactoring code outside the scope of the current task.
- Adding a new artifact type, skill, schema, or top-level directory.
- The design doc is silent or ambiguous on a structural question.
- Implementation is taking longer than expected and you're considering shortcuts.

Don't stop and ask for: trivial naming choices, where to put a clearly-bounded helper, formatting decisions covered by `golangci-lint` config.

### Surgical Changes

Touch only what you must. Clean up only your own mess.

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

**The test: every changed line should trace directly to the user's request.**

### Goal-Driven Execution

Define success criteria. Loop until verified.

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

    1. [Step] → verify: [check]
    2. [Step] → verify: [check]
    3. [Step] → verify: [check]

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## Code Design

### Core Principles

| Principle                  | Rule                                                                |
| -------------------------- | ------------------------------------------------------------------- |
| Deep Modules               | Interface small relative to functionality; resist "classitis"       |
| Information Hiding         | Each module encapsulates design decisions; no knowledge duplication |
| Pull Complexity Down       | Absorb complexity into implementation, not callers                  |
| Define Errors Out          | Redefine semantics so problematic conditions aren't errors          |
| General-Purpose Interfaces | Design for the general case; handle edge conditions internally      |
| Design It Twice            | Consider two radically different approaches before committing       |

### Red Flags

| Red Flag                | Signal                                              |
| ----------------------- | --------------------------------------------------- |
| Shallow Module          | Interface complexity ≈ implementation complexity    |
| Information Leakage     | Same design decision in multiple modules            |
| Repetition              | Near-identical code in multiple places              |
| Special-General Mixture | General mechanism contains use-case-specific code   |
| Vague Name              | Name too broad to convey specific meaning           |
| Hard to Describe        | Can't write a simple comment → revisit the design   |
| Nonobvious Code         | Behaviour requires significant effort to trace      |

### Common Rationalizations

| Rationalization                | Why It's Wrong                                            | What to Do Instead                                                |
| ------------------------------ | --------------------------------------------------------- | ----------------------------------------------------------------- |
| "More classes = better design" | More interfaces to learn, not simpler code                | Apply the complexity test — does splitting reduce cognitive load? |
| "Keep methods under N lines"   | Over-extraction creates pass-throughs and conjoined logic | Extract only when the piece has a meaningful name and concept     |
| "Add a config parameter"       | Pushes complexity to every caller                         | Compute sensible defaults inside the module                       |
| "Structure by execution order" | Causes information leakage across all steps               | Structure by information boundaries instead                       |
| "We might need this later"     | Speculative generality adds interface surface now         | "Somewhat general-purpose" — cover plausible uses only            |

## Hard Rules

- **No helper without a second use.** Don't extract until duplication exists.
- **No abstraction without need.** Premature abstraction creates surface that costs more than it saves.
- **No defensive code for unreachable states.** If a precondition is invariant, document it; don't check it at runtime.
- **No comments explaining *what*.** Comments explain *why* the code is shaped this way, never restate what the code does.
- **No `fmt.Println` for control flow output.** CLI output goes through cobra's `cmd.Println` / `cmd.PrintErrln` (which respect output redirection); structured logging goes through `log/slog`.
- **No new top-level dependencies without explicit user approval.**

If you write 200 lines and it could be 50, rewrite it. Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## Go Conventions

### Imports

- Standard import grouping: stdlib first, blank line, third-party, blank line, internal (`github.com/chonalchendo/anvil/internal/...`). `goimports` (run via `golangci-lint fmt`) enforces this.
- No dot imports (`import . "pkg"`). No blank imports outside `tools.go`-style files (and we don't have those — see `tool.go.mod`).
- Internal packages live under `internal/`. External consumers cannot import them by language rule.

### Type & API design

- Exported identifiers get a doc comment starting with the identifier name. One sentence is fine for trivial cases.
- Prefer small interfaces defined where consumed, not where implemented (Go idiom: "accept interfaces, return structs").
- Use `context.Context` as the first parameter on any function that does I/O, spawns goroutines, or might block. Never store contexts in structs.
- Use `time.Duration` for durations, `time.Time` for instants — never `int64` seconds.
- No `interface{}` / `any` in public APIs unless genuinely heterogeneous (and then comment why).

### Error handling

- Wrap with `%w`: `fmt.Errorf("loading config: %w", err)`. Never `fmt.Errorf("%v", err)` — destroys the chain.
- Check with `errors.Is` / `errors.As`. Never compare `err.Error()` strings.
- Define sentinel errors as package-level vars (`var ErrNotFound = errors.New("not found")`). Define typed errors as structs implementing `error`.
- Use `errors.Join` for multi-error aggregation; do not invent custom multi-error types.
- `golangci-lint`'s `errorlint` linter is enabled — it catches `err == ErrFoo` (wrong; use `errors.Is`) and unwrapped `%v` formats.

### Functions

- Max 5 parameters. Beyond that, group into a `Config` / `Options` struct.
- Use functional options (`func WithTimeout(d time.Duration) Option`) only when ≥3 optional params accumulate; otherwise a struct literal is fine.
- Boolean parameters get keyword-style call sites via struct fields, not positional booleans.

### Logging & output

- Structured logging via `log/slog`. `lmittmann/tint` for terminal output; JSON handler for debug-file output. Combine via `samber/slog-multi`.
- CLI output (user-facing) via cobra's `cmd.Println` / `cmd.PrintErrln`. Never `fmt.Println` directly in command code — it bypasses cobra's output redirection and breaks tests.
- No `log.Println` / `log.Fatal` in non-`main` packages. The standard `log` package writes to a global; `slog` is the contract.

### Concurrency

- Use `errgroup.Group` for goroutine fan-out with error propagation.
- Use `context.Context` for cancellation; never naked channels for cancel signals in new code.
- For subprocess management: `cmd.Cancel` (Go 1.20+) + `cmd.WaitDelay` for graceful-then-forceful shutdown.
- No `sync.Once` outside package init — if you need lazy init across goroutines, use a constructor.

### Subprocess gotchas (load-bearing)

- `bufio.Scanner`'s default `MaxScanTokenSize` is 64 KiB. Agent CLI tool-result lines exceed this. **Always** set `scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)` for an 8 MiB max line, or switch to `bufio.Reader.ReadBytes('\n')`. This is a documented invariant in `system-design.md`.
- Per-spawn `CLAUDE_CONFIG_DIR` and `CODEX_HOME` isolation is mandatory. Adapters set these; never rely on inherited env.

## Test Conventions

- Framework: stdlib `testing` + `google/go-cmp` for diffs. `gotestsum` as the runner. No testify.
- Test files live alongside source: `internal/core/manifest.go` ↔ `internal/core/manifest_test.go`.
- Use `t.TempDir()` for isolated file operations — **never touch real `~/.claude/` or real vaults**.
- Mock subprocess calls at the `os/exec.Cmd` boundary via a small interface in the adapter package; real-CLI tests live in `internal/adapters/integration_test.go` gated behind `// +build integration` (run via `just test-integration`).
- `testing/synctest` (Go 1.24+) is reserved for v0.2 wave-graph executor tests; not used in v0.1.

## What Never to Commit

- Credentials of any kind (real `auth.json`, API keys, tokens).
- Real API keys in tests — use sentinel values like `sk-test-fake`.
- Personal vault content (your `~/anvil-vault/`).
- Anything from `~/.anvil/` or `~/.claude/projects/`.
- `.env` files. Use `.env.example` as a template.
- Output artifacts (`dist/`, `build/`, `*.egg-info`).

## Commit Messages

Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `refactor:`, `release:`.

## Reference Documents

### Releasing — `@docs/releasing.md`

**Read when:** cutting a new version. Covers `uv version` bump, README/CHANGELOG updates, tag-and-push, and the `publish.yml` workflow.

> *Stale: rewrite pending Go release pipeline spec. Current content describes `uv version` + `publish.yml`; the Go pipeline (`goreleaser` v2 + Cosign + SLSA + Syft) lands in a future spec when the first release is cut.*

## Go ecosystem decisions (baked-in)

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
| Lint | `golangci-lint v2` with `linters.default: standard` + `errorlint`, `gocritic`, `revive`, `misspell`, `gosec`, `bodyclose`, `nilerr`, `contextcheck` | Use `golangci-lint fmt` instead of separate gofumpt. CI deferred. |
| Release | `goreleaser` v2 + Cosign v3 (`--bundle`) + SLSA via `slsa-github-generator` + Syft SBOMs | `releasing.md` rewrite deferred. |
| Errors | stdlib only — `errors.New`, `fmt.Errorf("…: %w", err)`, `errors.Is/As/Join` | No `pkg/errors`, no `cockroachdb/errors`. |
| Tools | go.mod `tool` directive (Go 1.24+) in isolated `tool.go.mod` | Replaces `tools.go` blank-import pattern. |
| Config | `knadh/koanf` v2 (when config code lands) | Viper rejected. |
| Vuln | `govulncheck` as a tool dep | CI integration deferred. |

---

These conventions are working if: fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
# Anvil — Agent Conventions

Anvil is a methodology for AI-assisted development packaged as auto-loading SKILL.md files with a thin Go orchestrator. Product vision: `@docs/product-design.md`; system design: `@docs/system-design.md`.

This file is an index — per-turn rules below; everything else loads on demand from `docs/`.

## Context Is Scarce

Schemas, skill bodies, AGENTS.md, vault docs all compete for runtime context budget. A field, section, or rule earns its place only if **load-bearing for an agent decision or a CLI/index query**. If it could live in body prose, it doesn't belong in the always-on layer.

## Hard Rules

- **No helper without a second use.** Don't extract until duplication exists.
- **No abstraction without need.** Premature abstraction creates surface that costs more than it saves.
- **No defensive code for unreachable states.** If a precondition is invariant, document it; don't check it at runtime.
- **No comments explaining *what*.** Comments explain *why* the code is shaped this way.
- **No `fmt.Println` for control flow output.** CLI output goes through cobra's `cmd.Println` / `cmd.PrintErrln`; structured logging through `log/slog`.
- **No new top-level dependencies without explicit user approval.**
- **No whole-file `Read` of files >150 lines without grepping first.** Grep for the symbol you need, then `Read` with `offset`/`limit`. See [Reading Discipline](docs/guardrails.md#reading-discipline).

Ask: "Would a senior engineer call this overcomplicated?" If yes, simplify.

## Worktrees and PRs (non-negotiable)

Every task runs in a worktree and lands via PR. Never commit directly on `master`. See `@docs/worktree-workflow.md` for the cut command, merge cleanup, the smoke-test gate required before every PR, and the wait-on-human-gated-PR rule.

## Working through issues

- Pick from `anvil list issue --ready --json`, not arbitrary `anvil list issue`. Ready issues have no unresolved blockers.
- Claim atomically: `anvil transition issue <id> in-progress --owner <your-name>`. Owner flag is required.
- Resolve via `anvil transition issue <id> resolved`. Use `anvil set ... status` only as a force-edit escape hatch.
- Search before creating: `anvil list <type>` and `anvil link --to <id>` before `anvil create`.
- Don't promote inbox items already covered by an issue: check `anvil link --to <issue-id>` first.

## Skills before CLI

For any activity with a corresponding skill — `capturing-inbox`, `writing-issue`, `writing-plan`, `writing-product-design`, `distilling-learning`, `opening-thread`, `handing-off-session`, `resuming-session` — fire the skill, not the raw CLI.

Mechanical verbs — `anvil reindex`, `anvil link --to`, `anvil where`, `anvil list`, `anvil show`, `anvil validate`, `anvil tags list` — fine to call directly.

If reaching for `anvil create <type>` and the type has a skill, stop and fire the skill instead.

## Dogfooding

Anvil is its own primary user. Friction surfaced while working on this repo goes straight to `anvil create issue` (reproducible) or `anvil create inbox` (unshaped). No side logs, no external trackers.

**The CLI is the highest-priority friction surface.** Measure every verb, flag, and error against `@docs/agent-cli-principles.md`. A violation is friction by design — log it even when it didn't block you.

Route by shape, not domain — if you can name an acceptance criterion in one breath, it's an issue:

- Raw / fuzzy thought → `anvil create inbox --title "<one line>" --suggested-type issue`.
- Shaped (problem + AC) → `anvil create issue --project anvil ...` linked to the active milestone. Quote the failing invocation verbatim with observed-vs-expected delta.
- **No structural PR without a vault antecedent.** Structural change = touches `AGENTS.md`, `docs/`, `.claude/`, `internal/schema/`, or adds a new top-level dir. The PR must reference an issue or inbox id.

Friction must square against `@docs/product-design.md`, `@docs/system-design.md`, `@docs/system-design/roadmap.md`. Monitor first-principles contracts — traceability, context budget, iron-law substance, no-scaffolding pitch — a break is vault-issue-worthy at severity ≥ high.

See `@docs/guardrails.md` for vault hygiene (Obsidian stub cleanup) and end-of-session token reflection.

## Reference Documents

`docs/go-conventions.md`, `docs/code-design.md`, and `docs/agent-cli-principles.md` are auto-injected into context on the first `Edit`/`Write` of a `*.go` file per session, via `.claude/hooks/inject-go-conventions.sh` (Claude Code only). The "Read when..." pointers below remain authoritative for other harnesses and for proactive consultation.

- `@docs/guardrails.md` — **MUST READ before any code or design change.** Think Before Coding, Surgical Changes, Goal-Driven Execution, Vault Hygiene.
- `@docs/worktree-workflow.md` — worktree cut/cleanup, smoke-test gate.
- `@docs/code-design.md` — designing a module, API, or refactoring. **(hook-loaded on .go edits)**
- `@docs/agent-cli-principles.md` — writing/reviewing/designing an `anvil` verb. **(hook-loaded on .go edits)**
- `@docs/go-conventions.md` — Go code. Imports, error handling, logging, subprocess gotchas. **(hook-loaded on .go edits)**
- `@docs/test-conventions.md` — tests. Stdlib `testing` + `go-cmp`, `t.TempDir()`, subprocess mocking boundary, integration build tag.
- `@docs/git-conventions.md` — commits. Conventional-commits prefixes and never-commit list.
- `@docs/dependencies.md` — new libraries. Baked-in Go ecosystem decisions; don't re-litigate without an ADR.
- `@docs/releasing.md` — cutting a version. *Stale: rewrite pending Go release pipeline spec.*
- `@docs/system-design/roadmap.md` — v0.1 scope, in/out-of-scope checks.
- `@docs/skill-authoring.md` — writing/editing a SKILL.md. Trigger contract, body shape, workflow-vs-knowledge split.
- `@docs/vault-schemas.md` — frontmatter. Universal fields, per-type reference, validation rules.

# Anvil — Agent Conventions

Anvil is a methodology for AI-assisted development packaged as auto-loading SKILL.md files with a thin Go orchestrator. The product vision lives in `@docs/product-design.md`; the architectural design lives in `@docs/system-design.md`. Read both before making structural decisions.

This file is an index. Per-turn rules live below; everything else is in `docs/` and loads on demand.

## Context Is Scarce

Anvil's outputs — schemas, skill bodies, AGENTS.md content, vault docs — all compete for the agent's context budget at runtime. Tokens spent on incidental prose are tokens unavailable for the actual work. Design lean.

The default is to cut. A field, section, or skill paragraph earns its place only if it is **load-bearing for an agent decision or a CLI/index query**. If a reader could derive it from neighbouring content, infer it from a wikilink, or read it later as body prose, it doesn't belong in the always-on layer.

Apply the test before adding anything: *is this load-bearing for an agent decision or a CLI query, or could it live in body prose?* If the answer is "could live in prose," put it there. The principle was crystallised in `docs/superpowers/specs/2026-05-01-vault-schemas-redesign-design.md` (rule 3, frontmatter); the same logic applies to this file.

## Hard Rules

- **No helper without a second use.** Don't extract until duplication exists.
- **No abstraction without need.** Premature abstraction creates surface that costs more than it saves.
- **No defensive code for unreachable states.** If a precondition is invariant, document it; don't check it at runtime.
- **No comments explaining *what*.** Comments explain *why* the code is shaped this way, never restate what the code does.
- **No `fmt.Println` for control flow output.** CLI output goes through cobra's `cmd.Println` / `cmd.PrintErrln` (which respect output redirection); structured logging goes through `log/slog`.
- **No new top-level dependencies without explicit user approval.**

If you write 200 lines and it could be 50, rewrite it. Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## Reference Documents

### Behavioral Guardrails — `@docs/guardrails.md`

**MUST READ before any code or design change.** Think Before Coding, Surgical Changes, Goal-Driven Execution. Defines when to stop and ask, the scope of allowed edits, and how to convert tasks into verifiable goals. Not optional — extracted for token budget, not for relevance.

### Code Design — `@docs/code-design.md`

**Read when:** designing a module, API, or refactoring. Core Principles, Red Flags, and Common Rationalizations tables for shaping deep modules and resisting over-engineering.

### Agent-Friendly CLI — `@docs/agent-cli-principles.md`

**Read when:** writing, reviewing, or designing an `anvil` verb. Seven rules
for CLIs that agents consume: non-interactive paths, structured output,
layered help, actionable errors, safe retries, composability, bounded
responses.

### Go Conventions — `@docs/go-conventions.md`

**Read when:** writing or editing Go code. Imports, type & API design, error handling, functions, logging, concurrency, and load-bearing subprocess gotchas (the 8 MiB scanner buffer; per-spawn `CLAUDE_CONFIG_DIR` / `CODEX_HOME` isolation).

### Test Conventions — `@docs/test-conventions.md`

**Read when:** writing or modifying tests. Stdlib `testing` + `go-cmp`, `t.TempDir()` isolation rule, subprocess mocking boundary, integration-test build tag.

### Git Conventions — `@docs/git-conventions.md`

**Read when:** committing changes. Conventional-commits prefixes and the never-commit list (credentials, vault content, `.env`, build artifacts).

### Dependencies — `@docs/dependencies.md`

**Read when:** considering a new library or questioning an existing choice. Baked-in Go ecosystem decisions (cobra/fang, slog, modernc sqlite, goreleaser, etc.) — don't re-litigate without an ADR.

### Releasing — `@docs/releasing.md`

**Read when:** cutting a new version. Covers `uv version` bump, README/CHANGELOG updates, tag-and-push, and the `publish.yml` workflow.

> *Stale: rewrite pending Go release pipeline spec. Current content describes `uv version` + `publish.yml`; the Go pipeline (`goreleaser` v2 + Cosign + SLSA + Syft) lands in a future spec when the first release is cut.*

### v0.1 Roadmap — `@docs/system-design/roadmap.md`

**Read when:** planning v0.1 scope, picking the next spec to write, or checking whether a piece of work is in/out of scope. Punch list of 20 items grouped by concern, with Phase A → B → C spec order.

### Skill Authoring — `@docs/skill-authoring.md`

**Read when:** writing or editing a SKILL.md. Trigger contract, body shape, workflow-vs-knowledge split.

### Vault Schemas — `@docs/vault-schemas.md`

**Read when:** authoring or modifying a vault artifact's frontmatter. Universal fields, per-type reference, schema-validation rules.

---

These conventions are working if: fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

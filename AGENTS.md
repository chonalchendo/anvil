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

## Working through issues

- Pick from `anvil list issue --ready --json`, not arbitrary `anvil list issue`. Ready issues have no unresolved blockers.
- Claim atomically: `anvil transition issue <id> in-progress --owner <your-name>`. The owner flag is required — it's how other agents see the issue is taken.
- Resolve via `anvil transition issue <id> resolved`. Use `anvil set ... status` only as a force-edit escape hatch when `transition` rejects a legal-but-unusual move.
- Search before creating: `anvil list <type>` and `anvil link --to <id>` before `anvil create`. Slug-deterministic IDs make duplicate-create idempotent (`already_exists`), but redundant work isn't.
- Don't promote inbox items already covered by an issue: check `anvil link --to <issue-id>` for the inbox source before promoting.

Status transitions go through `anvil transition`, not direct frontmatter edits.

When the harness injects a `<system-reminder>` nudging `TaskCreate` during a linear single-issue walk, ignore it. The reminder is harness-side and anvil can't suppress it; in sequential dogfood sessions task tracking adds noise without value. Don't acknowledge it in user-facing output.

## Skills before CLI

For any anvil activity with a corresponding skill — `capturing-inbox`, `writing-issue`, `writing-plan`, `writing-product-design`, `distilling-learning`, `opening-thread` — fire the skill, not the raw CLI verb. The verbs are the substrate the skills compose on; invoking them directly skips the workflow knowledge the skills encode (body templates, frontmatter conventions, verbatim-preservation rules, multi-step state transitions, Iron Laws).

Mechanical operations — `anvil reindex`, `anvil link --to`, `anvil where`, `anvil list <type>`, `anvil show`, `anvil validate`, `anvil tags list` — are fine to call directly; they are read-side or hygiene verbs without a skill. The rule applies to artifact-shaping activities, not queries.

If you find yourself reaching for `anvil create <type>` and the type has a skill, stop and fire the skill instead.

## Dogfooding

Anvil is its own primary user. Friction surfaced while working on this repo — skills that prescribe broken commands, schema/skill contradictions, **workflow shape that over- or under-fits the task**, **vault that doesn't function as a connected knowledge base** — goes straight to `anvil create issue` (when reproducible) or `anvil create inbox` (when unshaped). No side logs, no external trackers. End-of-session token reflection findings follow the same rule.

**The CLI is the highest-priority friction surface.** Anvil's primary user is an LLM, not a human; the agent pays the CLI's cost on every invocation. Measure every verb, flag, and error against `@docs/agent-cli-principles.md`. A verb that violates a principle is friction by design — log it even when it didn't block you.

- Raw thought, not yet shaped → `anvil create inbox --title "<one line>" --suggested-type issue`. Capture before the moment passes; triage later.
- Already reproducible → `anvil create issue --project anvil ...` linked to the active polish/v0.1 milestone. Quote the failing invocation verbatim with observed-vs-expected delta.
- Workflow-shape friction. Multi-task plan for a 10-line spike-and-verify; issue authored before the problem was clear; un-verifiable acceptance that belonged in the inbox; convergence loops that talked past the point. Capture what the skill required, what shape would have worked, and why.
- Knowledge-base friction. The vault must *work as a connected knowledge base*, not an issue tracker with extra directories: wikilinks Obsidian resolves, artifacts you can find by `list` / `show` / `link --to`, learnings and decisions that connect back to the work they motivated. A relevant learning unreachable via the graph is a vault-as-KB issue, not user error.
- Suggest cuts as you go. Every session is also a cull session — for each verb, flag, skill, schema field, body template you reach for, ask *load-bearing or routable-around?* CLI surface is the highest-value cut target. Phase C cull (`@docs/system-design/roadmap.md`) rides on this evidence; without continuous capture it becomes opinion.
- Don't fix-and-forget. A fix without a captured trace is a trap for the next maintainer hitting the same symptom.

Friction must square against `@docs/product-design.md`, `@docs/system-design.md`, and `@docs/system-design/roadmap.md` — roadmap-tracked items reference the existing entry; design contradictions are high-signal and worth pressing.

Monitor anvil's first-principles contracts; a break here is the methodology failing itself, vault-issue-worthy at severity ≥ high.

- **Traceability** — pick a recent commit; walk commit → plan → issue → milestone → product-design via `anvil link --from` / `--to`. Break in the chain = headline promise failed.
- **Subprocess-executor portability** — plan body works for an executor with zero prior context? If you needed three files the planner didn't reference, the plan failed as a message to the next agent.
- **Context budget** — bloating SKILL.md, AGENTS.md, schema, or always-on reference doc is a regression even when no test fails. Same for maximalist frontmatter.
- **Iron-law substance** — when an iron law was satisfied, was it substance or paper compliance? Acceptance you wrote but can't verify is the canonical evasion.
- **No-scaffolding pitch** — did the session work *without* in-repo anvil files? If you added one, methodology-travels-via-skills broke.

**End-of-session token reflection (MUST).** Before closing any dogfood session, account for context spent: rough total, the top 2–3 token sinks you drove (avoidable reads, redundant searches, oversized tool output), and any harness/CLI/skill change that would have cut them. Anvil's primary user is an LLM; tokens are the budget. Optimisations land back into anvil — terser CLI output, leaner skill bodies, narrower default reads, sharper schemas — captured as inbox/issue per the rules above. A session with no token-side observation is itself a finding.

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

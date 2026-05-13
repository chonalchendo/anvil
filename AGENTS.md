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

Every task runs in a worktree and lands via PR. Never `git checkout -b` or commit directly on `master` — parallel sessions collide, CodeRabbit gets no review pass, work accumulates unreviewed.

```bash
git -C ~/Development/anvil worktree add ~/Development/anvil-worktrees/<slug> -b anvil/<slug>
cd ~/Development/anvil-worktrees/<slug>
```

After merge: `git -C ~/Development/anvil worktree remove ~/Development/anvil-worktrees/<slug>`.

Workflow: cut worktree → implement + commit → pass smoke-test gate → `gh pr create` → wait for CodeRabbit + user approval → remove worktree after merge. CodeRabbit catches what unit tests miss (broken commands in error hints, schema-inconsistent JSON, surprising empty fields) — part of the verification budget, not optional. No exceptions for "small" changes.

## Smoke-Test Before Resolved (non-negotiable)

Before opening a PR or claiming a feature/fix done, drive it through the installed `anvil` binary against a real vault. Every feature, every fix.

1. `go install ./cmd/anvil`.
2. Confirm freshness: `which anvil` must resolve to `$(go env GOPATH)/bin/anvil` (or a symlink to it); else invoke `$(go env GOPATH)/bin/anvil` by absolute path. Cross-check `anvil --version` ends in the short sha of `git rev-parse --short HEAD` (Go embeds VCS info only when building from the main checkout, not a worktree — `dev` with no sha means run from the main checkout to confirm).
3. Invoke the new verb, re-trigger the changed error, or read the new skill phase end-to-end.
4. Compare output against acceptance criteria.
5. Any failure (broken commands in error hints, schema-inconsistent JSON, oversized output, blank fields) is a regression — fix before resolving.

Unit tests assert *some* string appears in output; they don't assert it's runnable, schema-consistent, or usable on 40 KB real-vault artifacts. Only live invocation catches that.

## Working through issues

- Pick from `anvil list issue --ready --json`, not arbitrary `anvil list issue`. Ready issues have no unresolved blockers.
- Claim atomically: `anvil transition issue <id> in-progress --owner <your-name>`. Owner flag is required — it's how others see the issue is taken.
- Resolve via `anvil transition issue <id> resolved`. Use `anvil set ... status` only as a force-edit escape hatch.
- Search before creating: `anvil list <type>` and `anvil link --to <id>` before `anvil create`. Slug-deterministic IDs make duplicate-create idempotent (`already_exists`), but redundant work isn't.
- Don't promote inbox items already covered by an issue: check `anvil link --to <issue-id>` for the inbox source first.

When the harness injects a `<system-reminder>` nudging `TaskCreate` during a linear single-issue walk, ignore it — anvil can't suppress it, and task tracking adds noise in sequential dogfood sessions.

## Skills before CLI

For any activity with a corresponding skill — `capturing-inbox`, `writing-issue`, `writing-plan`, `writing-product-design`, `distilling-learning`, `opening-thread`, `handing-off-session`, `resuming-session` — fire the skill, not the raw CLI. The verbs skip the workflow knowledge skills encode (body templates, frontmatter conventions, verbatim-preservation, multi-step state transitions, Iron Laws).

Starting a fresh terminal mid-task? Fire `anvil:resuming-session` to load the prior session's handoff. Ending one? Fire `anvil:handing-off-session`.

Mechanical verbs — `anvil reindex`, `anvil link --to`, `anvil where`, `anvil list`, `anvil show`, `anvil validate`, `anvil tags list` — fine to call directly; they're read-side or hygiene verbs without a skill.

If reaching for `anvil create <type>` and the type has a skill, stop and fire the skill instead.

## Dogfooding

Anvil is its own primary user. Friction surfaced while working on this repo goes straight to `anvil create issue` (reproducible) or `anvil create inbox` (unshaped). No side logs, no external trackers.

**The CLI is the highest-priority friction surface.** Anvil's primary user is an LLM; the agent pays the CLI's cost on every invocation. Measure every verb, flag, and error against `@docs/agent-cli-principles.md`. A violation is friction by design — log it even when it didn't block you.

- Raw thought → `anvil create inbox --title "<one line>" --suggested-type issue`.
- Reproducible → `anvil create issue --project anvil ...` linked to the active milestone. Quote the failing invocation verbatim with observed-vs-expected delta.
- Workflow-shape friction (multi-task plan for a spike; issue authored before the problem was clear; un-verifiable acceptance) — capture what the skill required, what shape would've worked, why.
- Knowledge-base friction. The vault must work as a connected knowledge base, not an issue tracker with extra directories. A relevant learning unreachable via the graph is a vault-as-KB issue.
- Suggest cuts as you go — for each verb, flag, schema field, body template, ask *load-bearing or routable-around?* Phase C cull rides on this evidence.
- Don't fix-and-forget. A fix without a captured trace is a trap for the next maintainer.
- **No structural PR without a vault antecedent.** Structural change = touches `AGENTS.md`, `docs/`, `.claude/`, `internal/schema/`, or adds a new top-level dir. The PR description must reference an issue or inbox id; author the issue mid-session before opening the PR if it doesn't exist. Bug fixes, dep bumps, and typo-only docs PRs exempted.

Friction must square against `@docs/product-design.md`, `@docs/system-design.md`, `@docs/system-design/roadmap.md` — roadmap-tracked items reference the existing entry.

Monitor first-principles contracts; a break is the methodology failing itself, vault-issue-worthy at severity ≥ high. **Traceability** (commit → plan → issue → milestone → product-design via `anvil link`); **subprocess-executor portability** (plan body works for an executor with zero prior context); **context budget** (bloating SKILL.md/AGENTS.md/schema is a regression even without a test failure); **iron-law substance** (acceptance you wrote but can't verify is paper compliance); **no-scaffolding pitch** (session worked *without* in-repo anvil files).

**Obsidian wikilink stubs.** Clicking an unresolved `[[issue.foo.bar]]` link in Obsidian stamps a 0-byte `issue.foo.bar.md` at the vault root. `anvil reindex` flags these on stderr (`WARN: 0-byte stub at vault root: ...`); `anvil reindex --prune-stubs` deletes the 0-byte ones. Non-zero files at the root with type-prefixed names are reported but never auto-deleted — move them into the canonical `<NN>-<type>/` dir or remove by hand.

**End-of-session token reflection (MUST).** Before closing a dogfood session: rough total, top 2–3 token sinks (avoidable reads, redundant searches, oversized tool output), and any harness/CLI/skill change that would've cut them. A session with no token-side observation is itself a finding.

## Reference Documents

`docs/go-conventions.md`, `docs/code-design.md`, and `docs/agent-cli-principles.md` are auto-injected into context on the first `Edit`/`Write` of a `*.go` file per session, via `.claude/hooks/inject-go-conventions.sh` (Claude Code only). The "Read when..." pointers below remain authoritative for other harnesses and for proactive consultation.

- `@docs/guardrails.md` — **MUST READ before any code or design change.** Think Before Coding, Surgical Changes, Goal-Driven Execution.
- `@docs/code-design.md` — designing a module, API, or refactoring. Core Principles, Red Flags, Common Rationalizations. **(hook-loaded on .go edits)**
- `@docs/agent-cli-principles.md` — writing/reviewing/designing an `anvil` verb. Seven rules for agent-consumed CLIs. **(hook-loaded on .go edits)**
- `@docs/go-conventions.md` — Go code. Imports, error handling, logging, subprocess gotchas (8 MiB scanner buffer; per-spawn `CLAUDE_CONFIG_DIR`/`CODEX_HOME`). **(hook-loaded on .go edits)**
- `@docs/test-conventions.md` — tests. Stdlib `testing` + `go-cmp`, `t.TempDir()`, subprocess mocking boundary, integration build tag.
- `@docs/git-conventions.md` — commits. Conventional-commits prefixes and never-commit list.
- `@docs/dependencies.md` — new libraries. Baked-in Go ecosystem decisions; don't re-litigate without an ADR.
- `@docs/releasing.md` — cutting a version. *Stale: rewrite pending Go release pipeline spec.*
- `@docs/system-design/roadmap.md` — v0.1 scope, in/out-of-scope checks.
- `@docs/skill-authoring.md` — writing/editing a SKILL.md. Trigger contract, body shape, workflow-vs-knowledge split.
- `@docs/vault-schemas.md` — frontmatter. Universal fields, per-type reference, validation rules.

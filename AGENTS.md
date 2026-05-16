# Anvil — Agent Conventions

Anvil is a methodology for AI-assisted development packaged as auto-loading SKILL.md files with a thin Go orchestrator.

This file is an index — per-turn rules below; everything else loads on demand from `docs/`.

## Hard Rules

- **Context is scarce.** Schemas, skill bodies, docs all compete for runtime budget. A field, section, or rule earns its place only if load-bearing for an agent decision or a CLI/index query.
- **No helper without a second use.** Don't extract until duplication exists.
- **No abstraction without need.** Premature abstraction creates surface that costs more than it saves.
- **No defensive code for unreachable states.** Document invariants; don't check them at runtime.
- **No comments explaining *what*.** Comments explain *why* the code is shaped this way.
- **No `fmt.Println` for control flow output.** CLI output through cobra's `cmd.Println` / `cmd.PrintErrln`; structured logging through `log/slog`.
- **No new top-level dependencies without explicit user approval.**
- **No whole-file `Read` of files >150 lines without grepping first.** See [Reading Discipline](docs/guardrails.md#reading-discipline).

Ask: "Would a senior engineer call this overcomplicated?" If yes, simplify.

## Worktrees and PRs (non-negotiable)

Every task runs in a worktree and lands via PR. Never commit directly on `master`. See `@docs/worktree-workflow.md` for the cut command, merge cleanup, the smoke-test gate required before every PR, and the wait-on-human-gated-PR rule.

**Install with `just install` only.** Never `go install ./cmd/anvil` directly — bypasses the PATH-shadow inode check and silently hands you a stale binary.

## Working through issues

- Pick from `anvil list issue --ready --json`, not arbitrary `anvil list issue`. Ready issues have no unresolved blockers.
- Claim atomically: `anvil transition issue <id> in-progress --owner <your-name>`. Owner flag is required.
- Resolve via `anvil transition issue <id> resolved`. Use `anvil set ... status` only as a force-edit escape hatch.
- Search before creating: `anvil list <type>` and `anvil link --to <id>` before `anvil create`.
- Don't promote inbox items already covered by an issue: check `anvil link --to <issue-id>` first.

## Skills before CLI

For any activity with a corresponding skill, fire the skill — not the raw CLI. Mechanical verbs — `anvil reindex`, `anvil link --to`, `anvil where`, `anvil list`, `anvil show`, `anvil validate`, `anvil tags list` — fine to call directly. If reaching for `anvil create <type>` and the type has a skill, stop and fire the skill instead.

## Evaluating work

Anvil is its own primary user. Friction surfaces here — no side logs, no external trackers. **But friction is signal, not work.** Most observations should not become individual issues. Three principles govern what gets worked on:

1. **Observations decay.** An issue must embed a reproduction — failing command, output snapshot, or SHA stamp — not just an AC. A future agent must be able to re-run and see if the gap still exists. `anvil list issue --ready --json` rolls into filtering on this once `anvil verify` lands.

2. **Friction is signal, not work.** Repeated friction against the same surface (a verb, a flag, an error message) aggregates. Don't file five issues for five paper-cuts on the same edge; the *fix* is a redesign of that surface, surfaced when the signal crosses a threshold. The CLI is the highest-value surface to measure (`@docs/agent-cli-principles.md`).

3. **Milestones are scoped, not buckets.** Every milestone has closed acceptance criteria. "Worth fixing now?" reduces to "blocks a named AC on an open milestone?" — yes pulls in, no aggregates. Existing bucket milestones (e.g. `v0-1-polish-dogfood-findings`) are grandfathered until they're re-scoped or culled.

When to use which surface:

- Reproduction in hand, blocks a scoped milestone → `anvil create issue --project anvil ...`. Quote the failing invocation verbatim with observed-vs-expected delta.
- Pattern without a single fix → aggregate; surface in end-of-session reflection.
- Fleeting hunch, no reproduction yet → `anvil create inbox --title "<one line>" --suggested-type issue`.

**No structural PR without a vault antecedent.** Structural change = touches `AGENTS.md`, `docs/`, `.claude/`, `internal/schema/`, or adds a new top-level dir. The PR must reference an issue or inbox id.

See `@docs/guardrails.md` for vault hygiene (Obsidian stub cleanup) and end-of-session token reflection.

## Reference Documents

`docs/go-conventions.md`, `docs/code-design.md`, and `docs/agent-cli-principles.md` are auto-injected on the first `Edit`/`Write` of a `*.go` file per session via `.claude/hooks/inject-go-conventions.sh` (Claude Code only). Read proactively otherwise.

- `@docs/guardrails.md` — **MUST READ before any code or design change.** Think Before Coding, Surgical Changes, Vault Hygiene.
- `@docs/product-design.md`, `@docs/system-design.md`, `@docs/system-design/roadmap.md` — product/system context, v0.1 scope.
- `@docs/test-conventions.md` — tests. Stdlib `testing` + `go-cmp`, `t.TempDir()`, integration build tag.
- `@docs/git-conventions.md` — commits. Conventional-commits prefixes and never-commit list.
- `@docs/dependencies.md` — new libraries. Baked-in Go ecosystem decisions; don't re-litigate without an ADR.
- `@docs/skill-authoring.md` — writing/editing a SKILL.md. Trigger contract, body shape, workflow-vs-knowledge split.
- `@docs/vault-schemas.md` — frontmatter. Universal fields, per-type reference, validation rules.

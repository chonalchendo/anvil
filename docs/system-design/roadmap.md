---
title: "Anvil v0.1 roadmap"
tags: [domain/dev-tools, type/system-design-shard]
---

# v0.1 Roadmap

What must ship before anvil v0.1. Derived from a 2026-05-03 audit of CLI surface, `internal/` packages, docs, release pipeline, tests, and skill wiring.

## Status

- **Phase A (unblock the workflow)** — done, except `using-anvil` skill.
- **Phase A.5 (agent-CLI Blockers)** — done, except `create` slug-collision (follow-up spec).
- **Phase B (orchestrator)** — not started. The v0.1 main event. Exits on a dogfood + telemetry-tuning pass.
- **Phase B.5 (onboarding skills)** — not started. Greenfield `new-project` + brownfield `onboard-project`, one skill family.
- **Phase C (ship)** — not started. Closes with a brutal cull pass.

Next up: Phase B (`anvil build`), starting with `internal/adapters`.

---

## Remaining

### Phase A — workflow

- **`using-anvil` skill** — agent-facing entry point that teaches the CLI surface for vault interaction (create/set/promote/show, type-by-type field cheatsheet, when to use CLI vs. direct edit). Today every other skill re-explains anvil verbs inline; this centralises it. Wait on `create` slug-collision fix so it documents the final surface.

### Phase B — orchestrator (`anvil build`)

Three sequenced sub-projects:

1. **`internal/adapters`** — define `AgentAdapter` contract for spawning Claude Code and Codex subprocesses with isolated `CLAUDE_CONFIG_DIR` / `CODEX_HOME` per spawn (per `docs/go-conventions.md`). Natural emit point for telemetry.
2. **Per-task telemetry (build-only slice)** — SQLite-backed (modernc-sqlite per `dependencies.md`). For each task: model, input/output/cache-read/cache-write tokens, USD cost, wall time, agent time, success/failure, verify exit code. Build-summary table at end of run; queryable via `telemetry/`. **Out of scope:** session-wide events, ad-hoc CLI telemetry, skill-execution telemetry, dashboards.
3. **`anvil build` command** — walk a validated plan's wave graph, dispatch via the adapter, persist telemetry, fail loudly. Wave graph already computed for `--waves` rendering in `internal/cli/plan.go`.

**Phase B exit criterion** — dogfood `anvil build` end-to-end against a small example project; capture telemetry (tokens read/written per skill, time per task, verify outcomes); refine context loading until tasks complete without errors under a defined token budget. The telemetry stats are the feedback loop, not a separate workstream.

### Phase B.5 — onboarding skills

Treat as a single skill family with greenfield + brownfield variants (one spec, two entry points):

- **`new-project` skill (greenfield)** — walks idea → research → challenge → scope vision → scope system → define milestones → seed high-level issues. Sits on top of `using-anvil` and `anvil build`.
- **`onboard-project` skill (brownfield)** — collaborative session + codebase scan to derive product/system design, current objectives, milestones, then backfill issues to fulfil them.

Defer until `using-anvil` and `anvil build` substrate is stable (i.e. after Phase B exit).

### Phase C — ship

- **README rewrite** — currently describes a Python orchestrator. Public-facing; blocks any external user. Resolve the `anvil compile` contradiction (referenced here and in `product-design.md`, absent from `cli-substrate.md` v0.1 verb set) at the same time. **Bundle D.**
- **Release pipeline** — `.goreleaser.yml` and `.github/workflows/release.yml` neither exist; Cosign/SLSA/Syft promised in `dependencies.md` are unwired. Rewrite stale `docs/releasing.md` (still mentions `uv version` / PyPI) to match. Add v0.1 entry to empty `CHANGELOG.md`. **Bundle C.**
- **CI gap closure** — `validate-vault.yml` runs `go build` + `go test ./...` only. Add `golangci-lint`, `-race`, and `//go:build integration` invocation.
- **Doc cleanup** — Move/delete `docs/design.md` (legacy Python frontmatter); move untracked `docs/IDEAS.md` / `docs/first_principles_anvil.md` / `docs/implementation_plan.md` into the vault or out of the source tree; add `skill-authoring.md` and `vault-schemas.md` references to `CLAUDE.md` index. **Bundle B.**
- **Brutal cull pass** — final entry before tagging v0.1. Cull skills, docs, and CLI surface guided by progressive disclosure and simple-but-deep interfaces; cut anything whose purpose isn't immediately obvious, anything that bloats the always-on context, anything that makes it harder for an agent to find what it needs. Ordered *after* Phase B dogfooding so telemetry tells us what's actually load-bearing.

### Agent-CLI follow-ups

**Blocker (carved out of Bundle E):**

- `create` slug-collision: re-running with same `--title` allocates a new ID (`slug-2`…) instead of returning the existing one. Detect by slug+project; return existing ID exit 0; require `--update` for content drift.

**Bundle F — Friction sweep** (mechanical; lands alongside Phase C):

- `set` — enum/kind error formatting; print `set <type> <id> <field>=<value>` on success (`--json {id, path, field, value}`); short-circuit identical re-apply with `unchanged`.
- `link` — `already linked` vs `linked`; `--json {source, target, status, path}`.
- `where` — `project: <none>` to stdout (or JSON `"project": null`); switch `fmt.Fprintln` → `cmd.Println` / `cmd.PrintErrln`.
- `create` — `--json` branch still uses `fmt.Fprintln(cmd.OutOrStdout(), …)` directly (codebase-wide convention now); enum errors need "valid values / corrected" pattern from principle 4.
- `project current` — error includes `anvil project adopt|switch <slug>` next step.
- `project switch` / `project adopt` — print success line; `--json` mirror.
- `init` — skip write when target exists and content matches; `--force` to overwrite.
- `install hooks` — keep `--uninstall` flag; document in help.
- **fang multi-line-error squashing** — `formatEnumError` (and any `\n`-separated body) is collapsed onto one line by fang's pretty-printer, defeating principle 4. Affects every cobra error. Decide between (a) bypass fang for structured errors, (b) configure fang to preserve newlines, (c) `cmd.PrintErrln` before returning a sentinel.

**Optimization** (deferred to v0.2 unless cheap):

- Cobra `Example` blocks on `where`, `create`, `show`, `list`, `link`, `set`, `promote`, `project *`, `link`, `project current`, `install`.
- `list` — `(N items)` footer to stderr; consider header line behind `--header`.
- `list` — deprecate `--tag`, recommend `--tags`.
- `validate` — `--paths` / `--type` filter to scope re-validation.
- `migrate` — `--dry-run`; print N-files-changed on completion.

---

## Done

### Phase A — workflow

- **Merge `brainstorming` into `writing-issue`** (spec `2026-05-03-writing-issue-merge-design`) — generative-mode primary; output is the issue body.
- **`researching` skill** (2026-05-05, spec `2026-05-05-researching-skill-design`) — `skills/researching/` with light/adversarial/heavy modes; synthesis returns to caller or persists as 0+ learnings.
- **Rewrite `extract-skill-from-session` phases 5–6** (spec `2026-05-05-extract-skill-phase-6-rewrite-design`) — phase 6 is mechanical agent-side checks; no `anvil skill` verb.

### Phase A — CLI write surface

(Spec `2026-05-03-cli-write-surface-gaps`, `2026-05-04-inbox-promote-idempotent-design`, `2026-05-04-type-template-completeness`, `2026-05-06-cli-consolidation-design`.)

- `--body` / stdin on `create` and `inbox add`.
- `anvil set` array fields via `schema.FieldKind` dispatch (`--add` / `--remove`).
- `anvil inbox promote <id> --as <type>` — single-step, idempotent.
- `anvil show <type> --validate` parity for issue + milestone (schema re-validation + `core.ResolveLinks`; `ErrUnresolvedLinks`).
- `product-design` / `system-design` as CLI types — singletons at `05-projects/<project>/<type>.md`.
- `sweep.tmpl` — `--scope` + explicit `--breaking`.
- `milestone.tmpl` — `acceptance: []` seeded.
- `anvil tags list` (2026-05-05) — `--type`, `--prefix`, `--json`.
- **CLI consolidation pass 1** (2026-05-06) — 16 → 14 top-level verbs; `glossary`/`inbox`/`session` subtrees folded into `tags`, generics, `promote`, `create session`.

### Phase A.5 — agent-CLI Blockers

(Spec `2026-05-05-bounded-structured-reads-design`, 2026-05-05.)

- `list` / `inbox list` — `--limit` (default 10), recency-desc sort, `--since`/`--until`, JSON envelope `{items, total, returned, truncated}`, stderr truncation hint.
- `list` — per-item `id`/`type`/`title`/`description`/`status`/`created`/`project`/`tags`/`path`.
- `show` — frontmatter-only default; `--full` body up to 500 lines with stderr clip hint; `--json` nests under `"frontmatter"`.
- `project list` — `--json` flat array.
- `validate` — structured `{code, path, field, got, expected?, fix?}` via `internal/cli/errfmt`; codes `enum_violation`/`missing_required`/`type_mismatch`/`constraint_violation`/`unresolved_link`.
- Root `--vault` / `--project` flags — precedence flag > env (`ANVIL_VAULT`/`ANVIL_PROJECT`) > cwd.
- `show --validate` — text view to stdout; diagnostics via `cmd.PrintErrln`.
- `inbox promote` — `formatEnumError` with `corrected:` line; idempotent re-runs return `already_promoted` / `already_discarded`.
- `cmd.Println` → `fmt.Fprintln(cmd.OutOrStdout())` migration for show/list/project/validate (cobra's `cmd.Print*` defaults to **stderr**, `command.go:1436`).

### Vault schemas

- **Faceted-tag enforcement** (2026-05-06, spec `2026-05-06-faceted-tag-enforcement-design`, plan `2026-05-06-faceted-tag-enforcement.md`) — per-type rules require `domain/<x>` (operational) or `domain/<x>` + `activity/<x>` (knowledge). CLI gate on `create` / `set tags` / `promote` rejects vault-novel values unless `--allow-new-facet=<facet>`; Levenshtein + containment suggestions. `type/<x>` convention dropped (covered by `type` discriminator). Unblocks v0.2 `anvil index`.

---

## Deferred to v0.2+

- `inline-fix` skill, `inbox → plan` shortcut — discovered organically via `extract-skill-from-session`.
- `verify-implementation` skill — verification already lives per-task in plan frontmatter.
- Read-side CLI gaps beyond Bundle E — AI reads files directly.
- Codex adapter installer — only Claude Code hooks installer ships in v0.1.
- Session-wide telemetry, dashboards, skill-execution events — only the build slice ships.
- **`anvil index` verb** — surfaces facet co-occurrence across issue/plan/decision/learning/thread to feed `extract-skill-from-session`. Read-only, no LLM. Spec: `docs/superpowers/specs/2026-05-06-vault-synthesis-design.md`. Faceted-tag prerequisite landed. Natural home for cross-project link discoverability ("show everything that links to X" / reverse-link queries) — wikilinks already resolve across projects (vault-global, since project is in the ID), but there's no verb to surface the wiring.
- **Sweep type review** — `sweep` may be cut entirely; thin schema, unclear use case. Brainstorm pending.
- Optimization-tagged agent-CLI items above (cobra `Example` blocks, `--paths` filters, `--dry-run` on `migrate`).

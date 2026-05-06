---
title: "Anvil v0.1 roadmap"
tags: [domain/dev-tools, type/system-design-shard]
---

# v0.1 Roadmap

Punch list of what must ship before anvil v0.1. Derived from a 2026-05-03 audit of CLI surface, `internal/` package state, doc completeness, release pipeline, tests, and skill wiring.

Items are grouped by concern, not by execution order. Spec order is at the bottom.

## Workflow / skill design

1. ~~**Merge `brainstorming` into `writing-issue`**~~ — **done** (spec `2026-05-03-writing-issue-merge-design`). Generative-mode primary, validation preserved. No separate `brainstorming` skill; output is the issue body.
2. ~~**New `researching` skill**~~ — **done** (2026-05-05, spec `2026-05-05-researching-skill-design`). Workflow skill at `skills/researching/` with three mode references (light / adversarial / heavy). No new vault type — synthesis returns to caller (sub-skill mode) or persists as 0+ learnings (standalone mode).
3. **New `using-anvil` skill** — agent-facing entry point that teaches the CLI surface for vault interaction (create/set/promote/show, type-by-type field cheatsheet, when to use CLI vs. direct file edit). Today every other skill re-explains anvil verbs inline; this centralises it.
4. ~~**Rewrite `extract-skill-from-session` phases 5–6**~~ — **done** (spec `2026-05-05-extract-skill-phase-6-rewrite-design`). Phase 6 downgraded to mechanical agent-side checks; no `anvil skill` verb introduced.

## CLI surface (write/update only; reads stay agent-side)

5. ~~**Body authoring on create**~~ — **done** (2026-05-04, spec `2026-05-03-cli-write-surface-gaps`). `--body` / stdin on `create` (all types) and `inbox add`; non-empty plan body replaces the T1 seed.
6. ~~**`anvil set` array-field support**~~ — **done** (2026-05-04, same spec). Schema-driven dispatch via `schema.FieldKind`; `--add` / `--remove` for arrays, scalar/array/unknown handled per spec matrix.
7. ~~**`anvil inbox promote <id> --as <type>`**~~ — **done** (spec `2026-05-04-inbox-promote-idempotent-design`). Single-step promotion via `--as` flag with enum-error helper; idempotent re-runs covered by the same spec.
8. ~~**`anvil show <type> --validate` parity**~~ — **done** (2026-05-04, same spec). Issue and milestone now run schema re-validation + `core.ResolveLinks` for dangling wikilinks; new `ErrUnresolvedLinks` sentinel.
9. ~~**`product-design` and `system-design` as CLI types**~~ — **done** (2026-05-04, spec `2026-05-04-type-template-completeness`). Added `TypeProductDesign`/`TypeSystemDesign`, `Type.AllocatesID()` + `Type.Path()` for per-project singletons at `05-projects/<project>/<type>.md`, templates, and existence-check on duplicate create.
10. ~~**`sweep.tmpl`**~~ — **done** (2026-05-04, same spec). Template ships; `anvil create sweep` requires `--scope` and an explicit `--breaking` (project-exempt). `TypeSweep` wired through `NextID`'s slug branch.
11. ~~**`milestone.tmpl` slots**~~ — **partial** (2026-05-04, same spec). `acceptance: []` now seeded; `product_design` / `system_design` wikilinks deliberately left to `set` calls.
11b. ~~**`anvil tags list`**~~ — **done** (2026-05-05, spec `2026-05-05-researching-skill-design`). Walks the vault, aggregates `tags` frontmatter into deduped (tag, count) list; `--type`, `--prefix`, `--json` filters. Used by artifact-creating skills to discover existing taxonomy before proposing new tags.

## Orchestrator

12. **`anvil build` pipeline** — three sequenced sub-projects:
    - **12a. `internal/adapters`** — currently empty placeholder. Define `AgentAdapter` contract for spawning Claude Code and Codex subprocesses with isolated `CLAUDE_CONFIG_DIR` / `CODEX_HOME` per spawn (per `docs/go-conventions.md`). Adapter is the natural emit point for telemetry.
    - **12b. Per-task telemetry (build-only slice)** — SQLite-backed (modernc-sqlite per `dependencies.md`). For each task run by `anvil build`, capture: model, input/output/cache-read/cache-write tokens, USD cost (model pricing table), wall time, agent time, success/failure, verify-command exit code. Surface as build-summary table at end of run; expose via `telemetry/` queryable store. **Out of scope:** session-wide event streams, ad-hoc CLI telemetry, skill-execution telemetry, dashboards.
    - **12c. `anvil build` command** — walk a validated plan's wave graph, dispatch tasks via the adapter, persist telemetry, fail loudly. Wave graph is already computed for `--waves` rendering in `internal/cli/plan.go`; build reuses that.

## Distribution / release

13. **`README.md` rewrite** — currently describes a Python orchestrator. Public-facing; blocks any external user.
14. **`docs/releasing.md` rewrite** — stale (`uv version`, `pyproject.toml`, PyPI trusted publishers). Replace with goreleaser v2 + Cosign + SLSA + Syft workflow.
15. **`.goreleaser.yml` + `.github/workflows/release.yml`** — neither exists. Cosign/SLSA/Syft promised in `docs/dependencies.md` are unwired.
16. **`CHANGELOG.md`** — file exists but is empty; needs the v0.1 entry.

## CI / quality gates

17. **CI gap closure** — `validate-vault.yml` runs `go build` + `go test ./...` only. Add `golangci-lint`, `-race`, and `//go:build integration` invocation (the convention is documented in `docs/test-conventions.md` but never used).

## Doc cleanup

18. **Resolve `anvil compile` contradiction** — referenced in `docs/product-design.md` and `README.md`; absent from the `cli-substrate.md` v0.1 verb set. Pick one.
19. **Move/mark `docs/design.md`** — legacy, still has Python frontmatter (`language: python`, `cli_framework: cyclopts`). Move to `docs/legacy/` or delete; `system-design.md` already says "do not edit."
20. **Untracked notes** — `docs/IDEAS.md`, `docs/first_principles_anvil.md`, `docs/implementation_plan.md` are personal notes living in `docs/`. Move into the vault or out of the source tree.
21. **`CLAUDE.md` index gaps** — add references to `skill-authoring.md` and `vault-schemas.md`; both are load-bearing and currently unindexed.

## Agent-CLI readiness

Gaps from the 2026-05-04 audit of every implemented verb against
`docs/agent-cli-principles.md`. Ordered flat by severity: Blocker → Friction
→ Optimization. Severity is a prioritisation tag only.

- [ ] `create`: re-running with same `--title` (slug) allocates a new ID (`slug-2`, `slug-3`) instead of returning the existing ID.
  Detect existing artifact by slug+project; return existing ID and exit 0; require `--update` for content drift. (Blocker — carved out of Bundle E into a follow-up spec.)
- [x] ~~`list`: no `--limit`; defaults to unbounded output, busting agent context budget on large vaults.~~ **done** (2026-05-05, spec `2026-05-05-bounded-structured-reads-design`) — `--limit` (default 10), recency-desc sort, `--since`/`--until`, JSON envelope `{items,total,returned,truncated}`, stderr truncation hint when truncated.
- [x] ~~`inbox list`: same unbounded result-set as `list` (delegates to `runList` without `--limit`).~~ **done** (2026-05-05, same spec) — same flag set threaded through `runList`; status:raw default preserved.
- [x] ~~`show`: no `--full`/summary mode; full body dumped on every call regardless of size.~~ **done** (2026-05-05, same spec) — frontmatter-only default (the curated `description` IS the summary); `--full` populates body up to 500 lines with stderr clip hint; JSON nests frontmatter under `"frontmatter"` key with `body`/`body_truncated`/`body_lines_total`.
- [x] ~~`project list`: no `--json` — read-shape verb without machine output (rule 2 mandates `--json` on read verbs).~~ **done** (2026-05-05, same spec) — flat array `[{slug, root}, …]`.
- [x] ~~`validate`: errors are flat strings — no offending field, no schema-derived valid values, no copy-pasteable correction.~~ **done** (2026-05-05, same spec) — structured `{code, path, field, got, expected?, fix?}` shape via `internal/cli/errfmt`; codes `enum_violation`/`missing_required`/`type_mismatch`/`constraint_violation`/`unresolved_link`; text mode renders blank-line-separated blocks.

- [ ] `create`: enum validation errors surface raw schema messages — no "valid values: …" / "corrected: …" pattern from principle 4.
  Wrap schema errors via a helper that pulls enum from schema and reformats with corrected invocation. (Friction)
- [ ] `set`: invalid enum / kind errors don't print valid values or corrected command.
  Shared formatter with `create`; for `KindArray`/`Scalar` mismatches print the schema-derived shape. (Friction)
- [ ] `set`: success path prints nothing — agent can't confirm what changed (id/field/new value).
  Print `set <type> <id> <field>=<value>` on success; `--json` emits `{id, path, field, value}`. (Friction)
- [ ] `set`: re-applying identical value rewrites the file (mtime churn) with no signal — not strictly idempotent in observable output.
  Short-circuit when new value equals existing; print `unchanged` and skip `Save`. (Friction)
- [ ] `link`: re-running with the same pair returns silently — agent can't distinguish no-op from initial link.
  Print `already linked` vs `linked`; add `--json {linked, source, target}`. (Friction)
- [ ] `link`: no `--json` output despite the rules asking for machine-parseable success on write verbs.
  Add `--json` emitting `{source, target, status: linked|already_linked, path}`. (Friction)
- [ ] `where`: when no project resolved, prints `project: <none>` to stderr — that semantic line should be on stdout (or in JSON).
  Emit on stdout in text mode; always include `"project": null` in JSON. (Friction)
- [ ] `where`: uses `fmt.Fprintln` instead of `cmd.Println` — bypasses cobra output redirection (rule 6).
  Switch all output to `cmd.Println` / `cmd.PrintErrln`. (Friction)
- [ ] `create`: `--json` branch (used by every artifact type, including `create session`) uses `fmt.Fprintln(cmd.OutOrStdout(), …)` — leftover from before we discovered cobra's `cmd.Print`/`Println`/`Printf` actually default to **stderr** (`OutOrStderr`, command.go:1436); test buffers masked it. The codebase-wide convention is now `fmt.Fprintln(cmd.OutOrStdout(), …)` for stdout. `show`/`list`/`project list`/`validate` migrated 2026-05-05; `create` still pending. (Friction)
- [ ] `list`: text mode produces tab-separated triples with no header and no count footer — agent can't tell if result was empty vs truncated.
  Print `(N items)` footer to stderr; consider header line in text mode behind `--header`. **partial 2026-05-05** (same spec) — truncation hint emitted to stderr when `returned < total`; no count footer when complete. (Friction)
- [x] ~~`list`: JSON returns flat array; no metadata envelope (count, truncated flag) so agent can't detect truncation programmatically.~~ **done** (2026-05-05, same spec) — envelope is `{items, total, returned, truncated}`.
- [ ] `project current`: error wrapping (`no current project: …`) doesn't include actionable next step.
  Enrich error with `run \`anvil project adopt <slug>\` or \`anvil project switch <slug>\``. (Friction)
- [ ] `project switch`: success prints nothing; agent can't confirm pointer moved.
  Print `switched to <slug>`; add `--json` for `{slug, root}`. (Friction)
- [ ] `project adopt`: success prints nothing; idempotence on re-adopt unclear from output.
  Print `adopted <slug> at <root>` or `already adopted`; add `--json` mirror. (Friction)
- [x] ~~`inbox promote`: error path lists choices but no copy-pasteable corrected command.~~ **done** (2026-05-05, spec `2026-05-04-inbox-promote-idempotent-design`) — `formatEnumError` helper emits `corrected:` line.
- [x] ~~`inbox promote`: not idempotent — re-running after success errors with `ArtifactNotFound` (inbox file removed) instead of returning existing target.~~ **done** (2026-05-05, same spec) — status flip on inbox; re-runs return `already_promoted` / `already_discarded`.
- [ ] **fang renderer squashes multi-line errors** — `formatEnumError` (and any future `\n`-separated error body) is collapsed onto one line, capitalised, and period-suffixed by fang's terminal pretty-printer. Defeats principle 4's separable shape (offending value / valid values / corrected line) for human-and-agent grep-ability. Affects every cobra error in the CLI, not just `inbox promote`.
  Decide between (a) bypassing fang for structured errors, (b) configuring fang to preserve newlines, or (c) emitting these as `cmd.PrintErrln` before returning a sentinel error. Bundle F candidate. (Friction)
- [x] ~~`show --validate`: text mode mixes `schema: ok` on stdout with link errors on stderr — interleaving makes parsing fragile.~~ **done** (2026-05-05, same spec) — text mode emits the artifact view to stdout via `emitFrontMatterText`, all `schema:`/`links:` diagnostics route through `cmd.PrintErrln`.
- [ ] `init`: writes embedded schemas with `os.WriteFile` (overwrites) every run — destructive on second run if user customised schemas.
  Skip write when target exists and content matches; `--force` to overwrite; warn on drift. (Friction)
- [ ] `install hooks`: `--uninstall` is a flag on the same verb; an explicit `install hooks remove` (or symmetric `uninstall`) is more discoverable.
  Keep `--uninstall` but add help example; consider sibling `uninstall hooks` later. (Friction)
- [x] ~~`root flags`: no global `--vault` / `--project` override flags — agent stuck with cwd resolution; no global `--json` default.~~ **done** (2026-05-05, same spec) — persistent `--vault`/`--project` on root; precedence flag > env (`ANVIL_VAULT`/`ANVIL_PROJECT`) > cwd resolution; help text documents precedence. Global `--json` deferred.

- [ ] `where`: `--help` shows flags only; no example, no link to deeper docs (principle 3).
  Add cobra `Example` block (`anvil where --json`); pointer to system-design doc. (Optimization)
- [ ] `create`: `--help` lacks per-type required-flag table and example invocations.
  Add `Example` block per common type (issue, plan, decision, sweep); reference `docs/vault-schemas.md`. (Optimization)
- [ ] `show`, `list`, `link`, `set`, `promote`, `project *`: none of these include cobra `Example` blocks in `--help` (principle 3).
  Add at least one `Example` per verb with realistic flag values. (Optimization)
- [ ] `list`: `--tag` (substring) and `--tags` (all-of) coexist — naming is confusable.
  Deprecate `--tag`, recommend `--tags`; document precedence in `--help`. (Optimization)
- [x] ~~`show`: `--json` shape mixes frontmatter keys flat with `body`/`path` — collisions possible if a frontmatter field is literally `body` or `path`.~~ **done** (2026-05-05, same spec) — frontmatter nested under `"frontmatter"` key; envelope keys are `id`/`path`/`frontmatter`/`body`/`body_truncated`/`body_lines_total`.
- [x] ~~`list`: JSON list omits per-item `created`/`project`/`tags` fields useful for agent filtering downstream.~~ **done** (2026-05-05, same spec) — items now include `id`/`type`/`title`/`description`/`status`/`created`/`project`/`tags`/`path`.
- [ ] `validate`: re-running after fix produces same scan cost; `--since` or `--paths` could narrow.
  Add `--paths` / `--type` filter to scope re-validation. (Optimization)
- [ ] `migrate`: no `--dry-run`; re-running after success is silent (`migration complete`) with no indication that 0 files changed.
  Add `--dry-run`; print N-files-changed on completion. (Optimization)
- [ ] `link`: `--help` has no `Example` showing canonical use (e.g. linking issue→milestone).
  Add `Example` block. (Optimization)
- [ ] `project current`: `--help` has no example showing `--json` shape.
  Add `Example: anvil project current --json`. (Optimization)
- [ ] `install`: only `hooks` subcommand today; `install --help` doesn't note future surface.
  Add note in long help; harmless until expansion lands. (Optimization)

## Vault schemas

22. ~~**Faceted-tag enforcement on issue, plan, decision, learning, thread**~~ — **done** (2026-05-06, spec `2026-05-06-faceted-tag-enforcement-design`, plan `2026-05-06-faceted-tag-enforcement.md`). Per-type schema rules require `domain/<x>` (operational) or `domain/<x>` + `activity/<x>` (knowledge). CLI gate on `anvil create` / `anvil set tags` / `anvil promote` rejects values novel to the vault unless `--allow-new-facet=<facet>` is passed; suggestions via Levenshtein + containment. `type/<x>` tag convention dropped (covered by the `type` discriminator field). Unblocks the v0.2 `anvil index` spec listed under "Out of scope".

## Out of scope (deferred to v0.2+)

- `inline-fix` skill, `inbox → plan` shortcut → discovered organically via `extract-skill-from-session` once #4 works.
- `verify-implementation` skill → verification already lives per-task in plan frontmatter.
- Read-side CLI gaps (`show`, `list` parity) → AI reads files directly.
- Codex adapter installer → only Claude Code hooks installer ships in v0.1.
- Session-wide telemetry, dashboards, skill-execution events → only the build slice ships.
- **`anvil index` verb** → surfaces facet co-occurrence patterns across operational + knowledge artifacts (issue, plan, decision, learning, thread) to feed `extract-skill-from-session`. Read-only, mechanical, no LLM. Spec: `docs/superpowers/specs/2026-05-06-vault-synthesis-design.md`. Faceted-tag enforcement prerequisite has landed in v0.1 (see "Vault schemas" group above).
- **Sweep type review** → `sweep` may be cut entirely from the vault; thin schema, unclear use case. Brainstorm pending.

## Spec order

**Phase A — unblock the workflow:** ~~#1, #5, #7, #8, #6, #9, #10, #11, #4~~ — **done.**

**Phase A.5 — agent-CLI Blockers (gate Phase B):**
~~Bundle E.~~ **done 2026-05-05** (spec `2026-05-05-bounded-structured-reads-design`) except the `create` slug-collision Blocker, carved out into a follow-up spec. `list`/`show`/`validate` are now bounded and machine-readable; Phase B can drive them.

**Phase B — orchestrator:**
#12a adapters → #12b build telemetry → #12c `anvil build`.

**Phase C — ship:**
#13, #14, #15, #16, #17, #18–21 (+ Bundle F friction sweep alongside).

Item #3 (`using-anvil` skill) is the only Phase A item still open and can land anywhere; #2 (researching skill) shipped 2026-05-05.

## Bundles

Items that ship together as a single spec/PR:

- **A — CLI-surface fills:** ~~#7 + inbox-promote agent-cli items + #4~~ **done 2026-05-05.**
- **B — Doc cleanup:** #19 + #20 + #21. File moves + index fixes, no code.
- **C — Release pipeline:** #14 + #15 (+ #16 v0.1 entry). Config and the docs that describe it must not drift.
- **D — Public-facing docs:** #13 + #18. README rewrite has to resolve the `anvil compile` contradiction anyway.
- **E — Agent-CLI Blockers:** ~~`list --limit`, `inbox list --limit`, `show --full`, `project list --json`, structured `validate` errors~~ **done 2026-05-05** (spec `2026-05-05-bounded-structured-reads-design`); also pulled in adjacent Friction (root `--vault`/`--project`, `show --validate` stream split, `list --json` envelope + per-item fields, `show --json` nested frontmatter, `cmd.Println` → `fmt.Fprintln(cmd.OutOrStdout())` migration for show/list/project/validate). `create` slug collision deferred to a follow-up spec.
- **F — Agent-CLI Friction sweep:** remaining Friction items — `set`/`link`/`where`/`project *` output + idempotence; `cmd.Println` consistency on `create` and `create inbox` JSON branches; `init` overwrite guard; fang multi-line-error squashing. Mechanical; lands alongside Phase C doc cleanup.

**Deferred to v0.2 unless cheap:** the Optimization-tagged items (cobra `Example` blocks, `--json` shape stability, `--paths` filters, `--dry-run` on `migrate`).

**Don't bundle:** #3 waits on #7/#4 so it documents the final surface; #2 needs a brainstorm pass; #12a/b/c is its own sequenced bundle and is the v0.1 main event.

## Done

- **CLI consolidation pass 1** — done 2026-05-06 (spec `2026-05-06-cli-consolidation-design`). 16 → 14 top-level verbs; `glossary`/`inbox`/`session` subtrees folded into `tags`, generics, `promote`, and `create session`.

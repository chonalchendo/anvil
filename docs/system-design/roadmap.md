---
title: "Anvil v0.1 roadmap"
tags: [domain/dev-tools, type/system-design-shard]
---

# v0.1 Roadmap

Punch list of what must ship before anvil v0.1. Derived from a 2026-05-03 audit of CLI surface, `internal/` package state, doc completeness, release pipeline, tests, and skill wiring.

Items are grouped by concern, not by execution order. Spec order is at the bottom.

## Workflow / skill design

1. **Merge `brainstorming` into `writing-issue`** — generative-mode primary, validation preserved. Brainstorm output is the issue body; no separate `brainstorm` artifact, no separate skill. Removes the issue-in-the-middle hop.
2. **New `researching` skill** — Claude-desktop-style research session. Open question: does it produce a vault artifact (new type + schema + template) or stay in-session and feed into brainstorm/issue/plan as cited prose?
3. **New `using-anvil` skill** — agent-facing entry point that teaches the CLI surface for vault interaction (create/set/promote/show, type-by-type field cheatsheet, when to use CLI vs. direct file edit). Today every other skill re-explains anvil verbs inline; this centralises it.
4. **Rewrite `extract-skill-from-session` phases 5–6** — currently calls `quick_validate.py` and an `anvil skill` verb that don't exist. Either add `anvil create skill` + `skill.schema.json` + a real validator, or downgrade phase 6 to mechanical agent-side checks. Cheap fix preferred — skill-authoring should not block v0.1.

## CLI surface (write/update only; reads stay agent-side)

5. ~~**Body authoring on create**~~ — **done** (2026-05-04, spec `2026-05-03-cli-write-surface-gaps`). `--body` / stdin on `create` (all types) and `inbox add`; non-empty plan body replaces the T1 seed.
6. ~~**`anvil set` array-field support**~~ — **done** (2026-05-04, same spec). Schema-driven dispatch via `schema.FieldKind`; `--add` / `--remove` for arrays, scalar/array/unknown handled per spec matrix.
7. **`anvil inbox promote <id> --as <type>`** — single-step promotion instead of the current two-step `set suggested_type` + `promote`.
8. ~~**`anvil show <type> --validate` parity**~~ — **done** (2026-05-04, same spec). Issue and milestone now run schema re-validation + `core.ResolveLinks` for dangling wikilinks; new `ErrUnresolvedLinks` sentinel.
9. ~~**`product-design` and `system-design` as CLI types**~~ — **done** (2026-05-04, spec `2026-05-04-type-template-completeness`). Added `TypeProductDesign`/`TypeSystemDesign`, `Type.AllocatesID()` + `Type.Path()` for per-project singletons at `05-projects/<project>/<type>.md`, templates, and existence-check on duplicate create.
10. ~~**`sweep.tmpl`**~~ — **done** (2026-05-04, same spec). Template ships; `anvil create sweep` requires `--scope` and an explicit `--breaking` (project-exempt). `TypeSweep` wired through `NextID`'s slug branch.
11. ~~**`milestone.tmpl` slots**~~ — **partial** (2026-05-04, same spec). `acceptance: []` now seeded; `product_design` / `system_design` wikilinks deliberately left to `set` calls.

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
  Detect existing artifact by slug+project; return existing ID and exit 0; require `--update` for content drift. (Blocker)
- [ ] `list`: no `--limit`; defaults to unbounded output, busting agent context budget on large vaults.
  Add `--limit` (default 50); print narrowing hint to stderr citing available filters when truncated. (Blocker)
- [ ] `inbox list`: same unbounded result-set as `list` (delegates to `runList` without `--limit`).
  Thread `--limit` (default 50) through `runList`; emit narrowing hint on truncation. (Blocker)
- [ ] `show`: no `--full`/summary mode; full body dumped on every call regardless of size.
  Default to header + first N lines summary; require `--full` for entire body; bound JSON `body` field too. (Blocker)
- [ ] `project list`: no `--json` — read-shape verb without machine output (rule 2 mandates `--json` on read verbs).
  Add `--json` emitting `[{slug, root}, …]`; document shape in `--help`. (Blocker)
- [ ] `validate`: errors are flat strings — no offending field, no schema-derived valid values, no copy-pasteable correction.
  Structure failures as `{path, field, got, valid_values, fix}` (with `--json`); humanise prose mode the same way. (Blocker)

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
- [ ] `create`: `--json` branch uses `fmt.Fprintln(cmd.OutOrStdout(), …)` instead of `cmd.Println` — minor consistency violation of rule 6.
  Use `cmd.Println`; same pattern in `inbox add` and `runList`/`runShow` JSON branches. (Friction)
- [ ] `inbox add`: same `fmt.Fprintln`-on-JSON pattern as `create`.
  Switch JSON emit to `cmd.Println`. (Friction)
- [ ] `list`: text mode produces tab-separated triples with no header and no count footer — agent can't tell if result was empty vs truncated.
  Print `(N items)` footer to stderr; consider header line in text mode behind `--header`. (Friction)
- [ ] `list`: JSON returns flat array; no metadata envelope (count, truncated flag) so agent can't detect truncation programmatically.
  When `--limit` truncates, emit `{items, total, truncated: true}` or set narrowing hint on stderr regardless. (Friction)
- [ ] `project current`: error wrapping (`no current project: …`) doesn't include actionable next step.
  Enrich error with `run \`anvil project adopt <slug>\` or \`anvil project switch <slug>\``. (Friction)
- [ ] `project switch`: success prints nothing; agent can't confirm pointer moved.
  Print `switched to <slug>`; add `--json` for `{slug, root}`. (Friction)
- [ ] `project adopt`: success prints nothing; idempotence on re-adopt unclear from output.
  Print `adopted <slug> at <root>` or `already adopted`; add `--json` mirror. (Friction)
- [ ] `inbox promote`: error path lists choices but no copy-pasteable corrected command.
  Include `anvil inbox promote <id> --as issue` example in error body. (Friction)
- [ ] `inbox promote`: not idempotent — re-running after success errors with `ArtifactNotFound` (inbox file removed) instead of returning existing target.
  Check whether equivalent target already exists; if so, exit 0 with target id. (Friction)
- [ ] `show --validate`: text mode mixes `schema: ok` on stdout with link errors on stderr — interleaving makes parsing fragile.
  Route diagnostic summary entirely to stderr; reserve stdout for canonical artifact view. (Friction)
- [ ] `init`: writes embedded schemas with `os.WriteFile` (overwrites) every run — destructive on second run if user customised schemas.
  Skip write when target exists and content matches; `--force` to overwrite; warn on drift. (Friction)
- [ ] `install hooks`: `--uninstall` is a flag on the same verb; an explicit `install hooks remove` (or symmetric `uninstall`) is more discoverable.
  Keep `--uninstall` but add help example; consider sibling `uninstall hooks` later. (Friction)
- [ ] `root flags`: no global `--vault` / `--project` override flags — agent stuck with cwd resolution; no global `--json` default.
  Add persistent `--vault`, `--project` flags on root; document precedence in `anvil --help`. (Friction)

- [ ] `where`: `--help` shows flags only; no example, no link to deeper docs (principle 3).
  Add cobra `Example` block (`anvil where --json`); pointer to system-design doc. (Optimization)
- [ ] `create`: `--help` lacks per-type required-flag table and example invocations.
  Add `Example` block per common type (issue, plan, decision, sweep); reference `docs/vault-schemas.md`. (Optimization)
- [ ] `show`, `list`, `link`, `set`, `inbox *`, `project *`: none of these include cobra `Example` blocks in `--help` (principle 3).
  Add at least one `Example` per verb with realistic flag values. (Optimization)
- [ ] `list`: `--tag` (substring) and `--tags` (all-of) coexist — naming is confusable.
  Deprecate `--tag`, recommend `--tags`; document precedence in `--help`. (Optimization)
- [ ] `show`: `--json` shape mixes frontmatter keys flat with `body`/`path` — collisions possible if a frontmatter field is literally `body` or `path`.
  Nest frontmatter under `"frontmatter"` key; document stable shape. (Optimization)
- [ ] `list`: JSON list omits per-item `created`/`project`/`tags` fields useful for agent filtering downstream.
  Document stable JSON shape; consider adding common spine fields (`project`, `created`). (Optimization)
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

## Out of scope (deferred to v0.2+)

- `inline-fix` skill, `inbox → plan` shortcut → discovered organically via `extract-skill-from-session` once #4 works.
- `verify-implementation` skill → verification already lives per-task in plan frontmatter.
- Read-side CLI gaps (`show`, `list` parity) → AI reads files directly.
- Codex adapter installer → only Claude Code hooks installer ships in v0.1.
- Session-wide telemetry, dashboards, skill-execution events → only the build slice ships.

## Spec order

**Phase A — unblock the workflow:**
~~#1 brainstorm-merge → #5 body authoring~~ → #7 → ~~#8, #6, #9, #10, #11~~ → #4 extract-skill fix.

**Phase B — orchestrator:**
#12a adapters → #12b build telemetry → #12c `anvil build`.

**Phase C — ship:**
#13, #14, #15, #16, #17, #18–21.

#2 (research skill) and #3 (using-anvil skill) are independent and can land anywhere in Phase A.

## Bundles

Items that ship together as a single spec/PR:

- **A — CLI-surface fills:** #7 + #4 (cheap-fix path). Both small CLI verbs; if #4 takes the "add `skill` as Type" route it reuses the product-design/system-design/sweep shape end-to-end.
- **B — Doc cleanup:** #19 + #20 + #21. File moves + index fixes, no code.
- **C — Release pipeline:** #14 + #15 (+ #16 v0.1 entry). Config and the docs that describe it must not drift.
- **D — Public-facing docs:** #13 + #18. README rewrite has to resolve the `anvil compile` contradiction anyway.

**Don't bundle:** #3 waits on #7/#4 so it documents the final surface; #2 needs a brainstorm pass; #12a/b/c is its own sequenced bundle and is the v0.1 main event.

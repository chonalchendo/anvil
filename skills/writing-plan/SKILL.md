---
name: writing-plan
description: |
  Use when an issue under 70-issues/ is open and a build-ready plan is needed
  before any production code is written. The issue must already link to a
  milestone with an approved design. Do NOT use when implementing inline in
  the current session; that path goes through anvil:implementing-plan.
---

# Writing Plan

> **Iron Law: NO PLAN SHIPS WITHOUT A FAILING TEST CONTRACT FOR EVERY TASK.**
>
> Every task in the plan must declare (a) the test that will fail before the
> task runs and (b) the verification command that proves it passes after.
> A task without a failing-test contract is not a task — it is a wish.
> If you are tempted to add a task without one, stop, scope-hammer, and split.

## When this skill runs

You enter this skill holding three things:

1. An open issue (`anvil show issue <id>` returns a valid artifact).
2. A milestone the issue links to (`anvil show milestone <m-id>` is valid).
3. A clean working tree, or a worktree dedicated to this issue.

## Announce yourself, then load context

Read, in this order:

1. `anvil show issue <id>` — the user-visible problem.
2. `anvil show milestone <m-id>` — scope alignment and design doc pointer.
3. The design doc the milestone points to (filesystem read; no typed CLI yet).
4. The repository's `AGENTS.md` / `CLAUDE.md` — project conventions.

If the design contradicts the issue, stop and ask the user. Do not infer.

## Phase 1 — Lock the decisions

Before decomposing, surface every architectural choice the design doc *implies*
but does not *state*. For each, write a Decision block:

- **Summary** in one sentence. **Status: locked.**
- **Alternatives considered**, each with one-sentence rejection reason.

Stop and ask the user when two alternatives have indistinguishable trade-offs,
the decision touches a public interface or persisted data shape, or it changes
the milestone surface area.

## Phase 2 — Decompose into tasks (vertical slices, TDD-shaped)

Each task must satisfy ALL of the following:

- **Vertical slice** — thin, end-to-end behavior. No horizontal-layer tasks.
- **15–60 minutes of agent work.**
- **≤ 3 files touched.** Hard limit; split if exceeded.
- **Test-first** — body opens with a failing test a fresh subprocess can run.
- **Single verify command** that returns 0 on success, non-zero on failure.

## Phase 3 — Declare dependencies, NOT execution order

Fill `depends_on` with task IDs whose *outputs* the task literally needs.
Do NOT add dependencies for logical sequencing — false serialization prevents
wave parallelism. Mentally topological-sort after writing all lists.

### File-isolation rule

No two tasks in the same wave may touch the same file. `anvil show plan <id>
--validate --waves` rejects with `code: same_file_in_wave` (lists the conflicting
tasks and file) when this is violated. The rule exists because executors fan out
inside a wave: two concurrent edits to the same file produce non-deterministic
output, race against each other's pre-condition checks, and break the TDD anchor.

When you hit it:

- **Split the task** along file-set lines. Each split task touches a distinct
  file; both can run in the same wave.
- **Add a depends_on edge** between the conflicting tasks. The dependent task
  drops to the next wave. Choose this when the second task genuinely consumes
  the first's edit (renamed symbol, new helper).
- **Don't merge tasks** to avoid the rule — that grows the file-set beyond the
  ≤ 3 cap and dilutes the TDD anchor.

> Open question for v0.2: a merge-aware fan-out could let multiple tasks edit
> the same file in one wave if the planner declared disjoint hunks. Out of scope
> for v0.1; tracked separately in the inbox.

## Phase 4 — Write the failing-test contract for every task

For each task, the body section MUST contain, in this order:

1. **Context the executor needs** — exactly what files, types, and
   conventions the fresh subprocess must know. Assume zero project context.
2. **Step 1 — RED:** the exact failing test, in code.
3. **Step 2 — Run, observe failure:** the verify command and the expected
   failure message.
4. **Step 3 — GREEN:** "minimal implementation to pass the test." Do NOT
   pre-write the implementation; the executor writes it.
5. **Step 4 — Verify and commit:** the verify command and the commit message
   template `<id>/<task-id>: <summary>`.

This is the *test-as-spec* posture — closer to strict TDD than test-after,
and adapted for isolated subprocess execution.

### Why this TDD posture, specifically

The candidate postures are: strict TDD (one test, one impl, refactor),
TDD-light (acceptance-shaped tests only), test-after, and test-as-spec.
For Anvil's executor model — fresh subprocess per task, no memory of prior
waves — only test-as-spec works:

- **Test-after** fails because the executor has no anchor for "is this done?"
  When the agent loses the thread mid-task, there is nothing to bring it
  back. Practitioner reports converge here (Simon Willison's "designing
  agentic loops," AI Hero's TDD skill writeup).
- **TDD-light** fails because the executor in a fresh subprocess will
  rationalize past acceptance tests under context pressure. Superpowers'
  baseline testing showed this directly — agents skip tests once the goal
  feels close.
- **Strict TDD inside one session** is what Superpowers enforces with its
  Iron Law. We adopt the same Iron Law because it survives the subprocess
  boundary: the test is *the message you send to the next agent*.
- **Test-as-spec** is just strict TDD plus the constraint that the test is
  written by the planner, not the executor. The executor's only job is to
  make the planner's test pass. This is also the explicit lesson of the
  alexop.dev "Forcing Claude Code to TDD" experiment: when the test-writer
  and implementer are different agents (here: planner vs. executor
  subprocess), the implementer cannot game the test.

If a task genuinely cannot be expressed as a failing test (e.g. "rename
package directory"), mark it `kind: mechanical` in the task block and
provide a `verify` command that proves the rename succeeded (a grep, a
`pytest --collect-only`, a build). The Iron Law is not "every task has a
unit test"; it is "every task has a verification that fails before and
passes after."

## Phase 5 — Self-review against the spec

- [ ] Spec coverage: every requirement in the design doc maps to ≥1 task.
- [ ] Non-goals: documented as a body section (## Non-goals), not as frontmatter.
- [ ] Type consistency: function names, type names, file paths match across tasks.
- [ ] No placeholders: no "add validation," no "similar to T2."
- [ ] Wave shape: ≥1 task has `depends_on: []`; longest chain ≤ 5.
- [ ] Budget realism: sum of `max_lines_changed` fits the milestone's appetite.
- [ ] `anvil show plan <id> --validate --waves` exits 0.

## Phase 5b — Per-task model/effort (optional)

Tasks default to `model: claude-sonnet-4-6` and `effort: medium` via orchestrator config. Set per-task `model: claude-opus-4-7` only on tasks that need deeper reasoning (architectural choices the planner deferred to the executor). Set `effort: high` on tasks expected to require extended work. Both fields are optional — omit when defaults fit.

## Phase 6 — Hand off

List the existing `domain/` taxonomy and reuse a value when one fits:

```bash
anvil tags list --source used --prefix domain/ --json
```

The CLI rejects an unrecognised value unless you pass `--allow-new-facet=domain`.

```bash
anvil create plan --issue <issue-id> --title "<title>" --tags domain/<x> --json
```

```bash
anvil show plan <plan-id> --validate --waves
```

If it errors, fix and re-validate.

**REQUIRED SUB-SKILL:** Use anvil:implementing-plan for single-agent inline execution today; `anvil build` once it lands for orchestrated execution.

## Plan state walk

The plan lifecycle is `draft → locked → in-progress → done` (with `→ abandoned` and a `done → in-progress` reverse audit edge). Each transition fires at a specific point in the workflow:

```bash
# Planner — after Phase 5 self-review and Phase 6 validation pass
anvil transition plan <id> locked

# Executor — at the start of implementation (locked → in-progress)
anvil transition plan <id> in-progress

# Executor — once every task is resolved and acceptance is met
anvil transition plan <id> done

# Recovery — reopen for follow-up work (requires --reason)
anvil transition plan <id> in-progress --reason "<why>"
```

The planner closes its phase at `locked`; the executor owns every transition after. Lock-before-execute is the contract: any plan in `draft` is not yet committed material.

## YAML frontmatter traps

The validator surfaces these as cryptic parse errors. Quote any scalar that hits them with `'...'` or `"..."`:

- **Backtick-prefixed scalar** — `` `cmd`: ... `` parses as a tag, not a string. Write `` '`cmd`: ...' ``.
- **Colon-space inside a value** — `summary: foo: bar` parses as a nested mapping. Write `summary: 'foo: bar'`.
- **Leading reserved indicators** (`>`, `|`, `&`, `*`, `!`, `%`, `@`, `` ` ``) — quote the value.
- **Multi-line text** — use block scalars (`>` folded, `|` literal) instead of embedded `\n`.

## Forbidden patterns

- "The executor will figure out the file layout" — no. Lock the paths.
- "Add tests for edge cases" — no. List them as tasks or enumerated assertions.
- "Refactor where it makes sense" — no. Refactoring is a task with its own RED/GREEN cycle.
- A task touching >3 files — split it. Hard cap, not a guideline.
- A task with no `depends_on` that imports from a later task — check the wave graph.

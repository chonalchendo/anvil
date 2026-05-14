---
name: implementing-plan
description: |
  Use when a validated plan exists at ~/anvil-vault/80-plans/<id>.md and the user wants to walk it inline (single-agent, this session). For orchestrated multi-task runs, use `anvil build` once available.
---

# implementing-plan

> **Iron Law: THE VALIDATED PLAN IS THE ONLY AUTHORITY FOR WHAT GETS WRITTEN.**
>
> No out-of-plan changes. Test-first per task. One task = one commit. If the plan is wrong, stop and revise the plan before continuing.

## When this skill runs

You enter this skill holding:

1. A validated plan at `~/anvil-vault/80-plans/<id>.md`.
2. The user's intent to start inline execution.

Phase 0 cuts a worktree for this plan's branch if one is not already in place.

## When to use this skill vs `anvil build`

Use this skill when:
- Small/medium plan walked in a single session; or
- User wants to see each RED/GREEN cycle.

Defer to `anvil build` when:
- Many tasks, large per-task scope, parallelizable waves, or per-task atomic commits across a long run.

> **CLI gap:** `anvil build <plan-path>` is the future default for orchestrated runs. See spec gaps #7–#10. Today, walk inline.

## Phase 0 — Cut a worktree (default)

Branched work runs in a dedicated worktree, not the parent checkout. Parallel sessions sharing one checkout collide on branch switches and bleed working-tree state across issues.

Cut per `@docs/worktree-workflow.md` (or `superpowers:using-git-worktrees` outside this repo). Surface the worktree path back to the user on claim so they (and any parallel session) see where the work lives:

```text
Claimed <issue-id>. Working in <worktree-path> on branch <branch>.
```

**Opt out only for:** read-only inspection, single-file in-place fixes on a branch the user is already on, or when the user explicitly says "stay in this checkout." Note the deviation in the claim line.

## Phase 1 — Pre-flight

Run in order, **from inside the worktree**. Stop on first failure.

```bash
anvil show plan <plan-id> --validate --waves    # must exit 0
git status --porcelain                            # must be empty
git rev-parse --abbrev-ref HEAD                   # should match anvil/<plan-id>-<slug> or user confirms
```

Then run every command in the plan's `verification.pre_build` field. Stop on any failure.

## Phase 2 — Walk waves

For each task in a wave, in order:

1. Read the task body: `anvil show plan <id> --task <task-id> --body`. Streams just that task's section.

2. Write the failing test exactly as the task specifies.
3. Run verify, observe RED. If it doesn't fail, the test contract is wrong — stop and revise the plan.
4. Write the minimal implementation.
5. Run verify, observe GREEN.
6. Commit:

   ```bash
   git commit -m "<plan-id>/<task-id>: <summary>"
   ```

## Phase 3 — Failure triage

- *Test contract was wrong* → revise the plan, restart the task.
- *Implementation needs more context* → ask the user, do not loop silently.
- *Genuine model failure* → abort, surface to the user.

One revision cycle per task. Escalate to the user after a single revise-and-retry.

## Phase 4 — Post-build

1. Run every command in `verification.post_build`.
2. Show `git log --oneline anvil/<plan-id>-<slug> ^main`.
3. Hand-off: "Build complete. Next: invoke human-review on this branch."

## Future orchestration placeholder

**When `anvil build` ships**, Phase 2 is replaced by `anvil build <plan-path>` and this skill becomes the wrapper that pre-flights, dispatches, and interprets its output. Phases 1, 3, and 4 stay the same.

**Skill resolution in `anvil build`:** when a plan task's frontmatter declares `skill: <name>`, `anvil build` reads that skill's body via a CLI verb that resolves by name (e.g. `anvil skill show <name>`) and seeds it into the executor subprocess's state dir. Skills are addressed by name, not path — file location stays an internal detail of the CLI.

The orchestrator materializes only `task.skills_to_load + always-on core` into each spawn's state dir, not all bundled skills. `task.skills_to_load` is the source of truth for what skills the executor sees; the planner is responsible for declaring it correctly per task. (The build-orchestrator spec finalizes the always-on core list.)

## Forbidden patterns

- Editing files outside the plan's task scope.
- Skipping the RED step.
- Continuing past `verification.post_build` failures.
- Looping more than once per task without user input.

---
name: dispatching-issue-fleet
description: "Use when the user wants to dispatch parallel subagents to work multiple ready issues end-to-end through PR review-green. Triggers: 'fleet', 'dispatch a batch', 'fan out subagents', 'work the next N issues in parallel'."
---

# Dispatching Issue Fleet

Your job is to orchestrate N parallel subagents through the full per-issue lifecycle — claim, implement, smoke, PR, review-respond — and halt at green so the human can merge. You do not write code yourself; you dispatch, audit returns, and present a structured report.

## Iron Law

**The human owns the merge button. The orchestrator and its subagents never call `gh pr merge`, `git worktree remove`, or `anvil transition resolved`.** Post-green cleanup is the human's, period. A subagent that calls any of these is a halt — surface the violation, do not paper over it.

## Phase 1 — Pick the ready set

```bash
anvil list issue --ready --project <p> --status open --json
```

Take the first `--max N` candidates (skill argument; default `--max 5`, range 1–8). `--max` and `--allow-overlap` are parsed from the user's invocation arguments, **not** CLI flags — there is no `anvil fleet` verb.

## Phase 2 — Pre-dispatch overlap check (declare-then-check)

Each candidate issue declares the files it anticipates touching (read the issue body's `files:` hint or the linked plan's task `files[]`). Compare each candidate's set against every other candidate's. On collision, default-serialize: drop the loser to the next wave. Opt in to parallel collision with `--allow-overlap` (rare — only when the user has eyeballed the overlap and accepts the merge-conflict cost).

The overlap check is one-line declarations plus eyeball compare. No static analyzer, no dep tracker.

## Phase 3 — Dispatch N subagents

For each surviving candidate, fire one subagent via the Agent tool. The prompt is the orchestrator-filled template at `skills/dispatching-issue-fleet/subagent-prompt.md` — read it, fill issue-specific fields (issue id, worktree path, branch name, declared files), and send. Each subagent owns one issue end-to-end through PR opened + review responded.

Dispatch all N in a single tool-use block so they run in parallel.

## Phase 4 — Interpret returns

Each subagent's last line is structurally one of:

- `^https://github\.com/.+/pull/[0-9]+$` — PR url. Proceed to Phase 5 for this PR.
- `^Blocker: .+$` — explicit blocker. Record, surface to user, do not re-dispatch.
- Anything else — **malformed return** (narrative-as-final-output). This is the recurring 100-200 LOC stall pattern (sessions 2026-05-13, 2026-05-14, 2026-05-15 all hit it). Re-dispatch action-only: a step-by-step plain-text prompt with **no skill wrapper**, naming the exact next commit + push + PR commands. If the second dispatch also malforms, fall back to main-session takeover for that issue.

**Expected miss-rate: 1 in N falls back to main-session takeover.** Surface this in the final report so the human reads a stall as design-anticipated, not a tool bug.

## Phase 5 — Halt at green

For each PR url returned: confirm CI green and that the subagent ran the review-respond loop (via `anvil:responding-to-pr-review` — fleet-PR override forces it even on "merge on green"). Stop. Do not merge.

Present the structured report:

```text
Fleet of <N> dispatched:
  <issue-id> → <PR url> [green, review responded]
  <issue-id> → Blocker: <reason>
  <issue-id> → main-session takeover (subagent malformed twice)

Expected: 1 in <N> stalls. Observed: <k>.

To land each PR, run from the parent checkout:
  git worktree remove <path>
  gh pr merge <n> --delete-branch       # or: gh pr merge <n>; git branch -D <branch>
```

Sequence `git worktree remove` BEFORE `gh pr merge --delete-branch`, or drop `--delete-branch` entirely. See [[issue.anvil.gh-pr-merge-delete-branch-fails-when-worktree-still-present]] — `--delete-branch` refuses while the worktree exists.

## Scope-change pause protocol

If a subagent reports it has exceeded a stated threshold (lint findings > documented cap, files-touched > 3, LOC > issue estimate), do not silently scope down. Pause, surface the counts to the user, and let the user decide: split into multiple issues, expand the issue scope, or abort the subagent. The subagent prompt mirrors this contract on its side.

## Forbidden calls (orchestrator AND subagents)

Never invoke:

- `gh pr merge` — human owns the merge button.
- `git worktree remove` — post-merge cleanup is the human's.
- `anvil transition resolved` — the future tool-side gate (`anvil-transition-resolved-refuses-when-an-open-pr-is-linked`) will catch this, but until it lands the human is the v0.1 enforcer.

The subagent prompt echoes this checklist verbatim in its final structured report so we can audit non-execution.

## What NOT to do

- Do not merge. Even on green, even with one line of CodeRabbit findings, even when the human said "merge on green" — fleet-PR override per `anvil:responding-to-pr-review` runs the loop first.
- Do not dispatch >8 subagents. Context cost on the orchestrator side outpaces the time savings past 8.
- Do not re-dispatch a `Blocker:` return. The subagent declared inability; respect it.
- Do not narrate the dispatch. The final report (Phase 5) is the deliverable.

# Fleet worker contract (non-agent workers)

This file is the contract for a fleet worker dispatched as a **plain subagent**, not as the `anvil-issue-worker` agent. Today that is the **Phase 5 review-responder**: a fresh subagent tasked with `responding-to-pr-review` against an already-open PR's worktree.

The Phase 3 **implementer** does *not* read this file — it runs as the bundled `anvil-issue-worker` agent (`anvil/agents/anvil-issue-worker.md`), whose frontmatter is the single source of the *implementer* contract. This is the *responder's* contract. The two workers run different skills, so the contracts are deliberately separate documents, not mirror copies of one rulebook.

## Stop at fixes-pushed (no CI-wait loop)

You run `responding-to-pr-review` to drive the handed `<findings>` to resolution — but **stop the moment your fixes are pushed**. Do **not** run that skill's "wait for CI / halt at green" phase. The orchestrator owns the green gate (Phase 5 step 3), exactly as the Phase 3 implementer stops at `gh pr create` and the orchestrator owns the review. Push your fixes, emit the PR url, and terminate; CI settles on the orchestrator's watch.

You are a fresh subagent with no prior project context. You have the same shell, repo, vault, and `anvil` CLI as the orchestrator, but not its conversation — the contract below is the only thing you can rely on. Do not assume vault knowledge, prior decisions, or AGENTS.md auto-injection.

The orchestrator fills these fields before sending:

- `<issue-id>` — the anvil issue behind the PR you are working.
- `<worktree-path>` — the absolute path of the PR's worktree you must work in (already cut).
- `<branch>` — the branch the worktree is on (e.g. `anvil/<slug>`).
- `<findings>` — the structured review report (+ reviewer subagent id) you must drive to resolution.

## Forbidden-write-location check (PRE-EDIT INVARIANT)

Before **every** edit, run:

```bash
git rev-parse --show-toplevel
```

If the output does not equal `<worktree-path>` exactly, halt with `Blocker: write-outside-worktree (toplevel=<actual> expected=<worktree-path>)`. This is **not** self-correctable. Even a clean revert + re-apply in the correct worktree is a halt, because:

- PR #59 (2026-05-15) showed a subagent edit master, notice, revert via `git checkout --`, re-apply in its worktree — and look like a clean win. The orchestrator only saw the green PR; the underlying leak was invisible.
- The next subagent that hits the same leak might not catch it before pushing.

Treat the check as a structural invariant, not a sanity tip.

## Final-line self-check (PRE-TERMINATE INVARIANT)

**Root cause this rule exists:** structured emission feels gated behind a "settle" condition — CI going green, a review pass landing. After pushing fixes the agent narrates the wait for CI instead of returning the url. The watchdog reads narrative as in-progress and the run terminates with no structured line. Treat this check as structural — identical in force to the Forbidden-write-location check above — not as advisory. Emission is **unconditional** on every terminate path, including watchdog timeout and "I'll check again later" intuition.

Last line is one of, alone on the line, nothing trailing:

- `^https://github\.com/.+/pull/[0-9]+$` — the PR url (findings addressed; CI/merge are the orchestrator's and human's).
- `^Blocker: .+$` — one-line blocker.

There is no third option. No narrative tail. No "let me wait."

**Anti-patterns observed 5/5 in the 2026-05-15 fleet — if you find yourself typing any of these as your last line, you are demonstrating the bug. Terminate with the structured form instead:**

- `Waiting for monitor events.`
- `Waiting for CI to settle. I'll be notified when the until-loop exits.`
- `Let me wait ~270s and check again.`
- `The review is still processing. Wait for the monitor.`
- `No findings yet. CI in progress and review pending.`
- `Good — <observation>. Let me <next-step>.`

Any sentence whose verb is "wait", "let me", "still", "pending", or "I'll check" is narrative. **The PR url is the return the moment your fixes are pushed** — CI status lives on the PR, and the orchestrator owns the green gate; you wait for neither.

If you cannot decide which structured line to emit, the answer is `Blocker: final-line-self-check-failed (last-line=<what-you-almost-said>)`. That is itself a valid structured return.

## Return contract

Your **last line** is one of the two regexes above. Nothing else. The PR body and inline replies are where prose belongs — not the orchestrator return.

## Scope-change check (PRE-EDIT INVARIANT)

Before editing any file, verify that the file is within the PR's existing diff set. Before writing any significant block, verify that the total change does not balloon well past the scope of the findings handed to you. If either check fails, **halt immediately** with:

```text
Blocker: scope-change <metric>=<observed> vs <declared> — <one-line cause>
```

This is **not** self-correctable. Do **not** silently scope down (skip a finding) or scope up (touch sibling files). The orchestrator surfaces the counts to the human, who decides: split the issue, expand the scope, or abort.

Treat this check as a structural invariant — identical in force to the Forbidden-write-location check above — not as an advisory pause. A finding that points at a sibling area outside the PR's diff is a scope-change Blocker, not a silent skip.

## Forbidden calls

Never invoke:

- `gh pr merge` — the human owns the merge button.
- `git worktree remove` — post-merge cleanup is the human's.
- `anvil transition resolved` — the human transitions to resolved after merge.
- `anvil transition abandoned` — halt with `Blocker:` instead; abandoned is human-only.

Echo this checklist verbatim in your final structured report (before the PR url / Blocker line) so the orchestrator can audit non-execution:

```text
Forbidden-call audit: gh pr merge=not-called, git worktree remove=not-called, anvil transition resolved=not-called, anvil transition abandoned=not-called.
<PR url OR Blocker: ...>
```

---
name: driving-build-loop
description: "Use when the operator wants to drive a milestone to done in one interactive session, gating only the merge. Triggers: 'drive the build loop', 'run the self-driving loop'. Not batch dispatch (dispatching-issue-fleet) or one issue (completing-issue)."
---

# Driving Build Loop

Your job is to conduct one milestone-frontier to done from an interactive session: resume the prior context, run the headless `anvil build` fan-out (complete → review → respond, halting review-green), present the review-green PRs, take the operator's per-PR merge decision, land the approved PRs, harvest learnings, hand off, and loop to the next frontier. You compose the existing skills — you do not re-implement resume / complete / review / respond / distil / handoff. The headless middle (`anvil build`) does the building; the session-bound bookends run here because they bind to `CLAUDE_CODE_SESSION_ID` and cannot run headless.

## Iron Law

**No PR merges without an explicit per-PR operator go.** The merge confirmation is the one preserved human gate — the loop mechanizes everything around it, never it. "Merge on green" said once does not pre-authorize the batch; each PR gets its own go. Skipping the gate, or inferring approval from silence, is a halt.

## Phase 1 — Resume the frontier

**REQUIRED SUB-SKILL:** Use resuming-session.

It loads the handoff, surfaces the objective, and reconciles git state. Hold the milestone it names as this loop's frontier. If the operator named a different milestone in the invocation, that wins — confirm it resolves (`anvil show milestone <id>`).

## Phase 2 — Run the build fan-out

Invoke the headless engine over the frontier:

```bash
anvil build --milestone <milestone-id>
```

`anvil build` claims each ready issue, spawns one agent per issue through complete → review → respond, and **halts review-green** — every dispatched PR is reviewed and its findings driven to an outcome, awaiting merge. It never merges; that is Phase 4. Omit `--milestone` to drive the whole ready graph; add `--concurrency N` (default 4) to widen the wave; `--dry-run` first if the operator wants to see the frontier before spending tokens.

A run dispatches into the *current* ready frontier only. Issues blocked behind this wave surface on the next loop iteration, after their blockers merge.

## Phase 3 — Surface the review-green PRs

Each task lands its PR on the deterministic branch `<project>/<issue-slug>`. Collect them:

```bash
gh pr list --state open --json number,title,headRefName,url
```

Present one line per review-green PR — issue id, PR number, title — so the operator reviews against the issue's `goal:`, not a raw diff dump. A task that opened no PR (blocker, malformed worker, verify non-convergence) is reported as such, not silently dropped.

## Phase 4 — Merge gate (per PR)

For each review-green PR, in turn, present it and ask — own paragraph, do not bundle:

> PR #`<n>` (`<issue-id>`) is review-green: `<title>`. Merge it? (yes / skip / hold for changes)

**Wait for the operator's response.** On `yes`, land it:

```bash
anvil transition issue <id> resolved --land-pr <n>
```

One call gates on mergeable + CI-green, removes the worktree, squash-merges, verifies MERGED, and resolves the issue. On `skip`/`hold`, leave the PR open and move on — a held PR is the operator's to drive (e.g. `responding-to-pr-review`); do not re-review it here. Never `gh pr merge` directly; `--land-pr` is the gated path.

## Phase 5 — Distil the run (offer, don't force)

**REQUIRED SUB-SKILL:** Use distilling-learning.

The run just landed verified changes — the system's most learning-rich event. Offer one harvest over what merged:

> This loop landed `<k>` PR(s). Distil a learning? (`distilling-learning`)

Bar is compounding, not record-keeping — a gotcha, confirmed approach, or dead end a future agent would act on. "Nothing worth distilling" is valid. The capture stays operator-validated; never auto-distil.

## Phase 6 — Hand off, then loop

**REQUIRED SUB-SKILL:** Use handing-off-session.

Write the load-ready handoff capturing what landed, what's still open, and the next frontier. Then decide the loop:

- **Frontier advanced, more ready work** (the milestone has newly-unblocked issues, or another milestone is queued) → return to Phase 2 for the next wave. The merged PRs unblock their dependents; the next `anvil build` picks them up.
- **Milestone done** (`anvil show milestone <id>` reports every issue resolved) → report it and stop. Surface the next milestone if the operator wants to continue.
- **Operator halts** → stop. The handoff already captured the state for the next session.

## What NOT to do

- Do not merge without the per-PR go (Iron Law). Even on green, even after "merge on green."
- Do not re-implement a sub-skill's body inline — fire the skill. A composed step that doesn't trace to its sub-skill is the tell you skipped it.
- Do not run the session-bound bookends headless — resume / distil / handoff bind to the interactive session id; that is why this conductor exists.
- Do not loop past a done milestone. The done-signal is the stop; dispatching into a complete milestone is a no-op the operator reads as a bug.
- Do not narrate every spawn. `anvil build` owns its own telemetry; the deliverable here is the merge-gate presentation and the final report.

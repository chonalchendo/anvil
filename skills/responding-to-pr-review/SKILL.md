---
name: responding-to-pr-review
description: "Use when a PR has inline review comments to address — CodeRabbit, a human reviewer, or reviewing-pr's output. Triggers: 'respond to the review', 'address coderabbit', 'reply to inline comments', 'babysit PR <n>'."
---

# Responding to PR Review

Your job is to drive a PR's review threads to "every comment has a reply" and CI to green, then surface the PR url back. You do **not** merge — `anvil:dispatching-issue-fleet`'s Iron Law (human owns the merge button) applies here too.

## Iron Law

**Every inline comment receives a reply — fix, skip-with-reason, or push-back. No silent drops.** A reply is the audit trail; a missing reply is a hidden disagreement.

## Review-source-agnostic posture

The same pipeline handles CodeRabbit, a human reviewer, and `anvil:reviewing-pr`'s fresh-subagent output. The discriminator is structural — *is this an inline thread on a hunk?* — yes → thread-reply via `gh api .../comments/<id>/replies`; no → top-level comment via `gh pr comment`.

Reviewer identity does **not** change the loop. CodeRabbit nitpicks that cite a documented repo rule (e.g. `docs/code-design.md`'s "no helper without second use") get the same treatment as a human asking for the same fix: apply, do not skip.

## Phase 1 — Fetch

```bash
gh pr view <n>                                              # status, branch, mergeability
gh api repos/<o>/<r>/pulls/<n>/comments \
  --jq '.[] | {id, path, line, user: .user.login, body, in_reply_to_id}'
gh pr checks <n>                                            # CI state
```

If there are zero inline comments AND CI is green AND the PR was not opened by a dispatched subagent (see fleet-PR override below): the review-respond loop is no-op. Surface that and return.

## Phase 2 — Evaluate

Defer evaluation discipline to `superpowers:receiving-code-review`. Per thread, decide one of:

- **Fix.** Implement, commit, push. Cite the commit SHA in the reply.
- **Skip with reason.** Reply with the reason in the thread. Examples: "out of scope for this PR — tracked in `<issue-id>`", "intentional per `docs/<...>`".
- **Push back.** Reply disagreeing, with rationale. The reviewer either updates or the human resolves it.

**Nitpick policy:** if a nitpick cites a documented repo rule, **apply** the fix. Do not skip with "nitpick". Memory entry `feedback_coderabbit_read_inline_before_merge` is the rationale — SUCCESS status is non-blocking, not no-findings.

## Phase 3 — Apply / push back per thread

For each thread that takes a fix:

1. Edit, commit with a focused message, push.
2. Capture the new commit SHA.
3. Reply on the thread: `gh api -X POST repos/<o>/<r>/pulls/<n>/comments/<thread-id>/replies -f body="Fixed in <SHA>: <one-line rationale>"`.

For skip-with-reason or push-back, reply with the rationale only — no commit needed.

After all threads have a reply, post a top-level summary:

```bash
gh pr comment <n> --body "Addressed N threads as of <SHA>: <k> fixes, <m> skips-with-reason, <p> push-backs. <one-line residual delta if any>."
```

## Phase 4 — Poll for CI and follow-up review

Respect the poll budget in `docs/worktree-workflow.md`. Default poll-every-30min eats the prompt cache; prefer event-driven re-entry. When a check is mid-flight and you must poll, use ~270s intervals (stays inside the 5-minute cache TTL).

Wait for CI to settle on the new SHA, then re-fetch comments — reviewers may add follow-ups. Loop Phase 2-3 until the PR has a stable green state with every thread replied.

## Rate-limit fallback

CodeRabbit caps per user per hour. If a batched-PR session burns the cap and a PR sits without a CodeRabbit pass beyond the budget, merge on **local-review + CI-green**. Memory entry `feedback_coderabbit_rate_limit_per_hour` records the cap-shape; 5-PR batches reliably hit it.

The fallback is local-review, not zero review. Do a one-pass diff read against the hard-rules list before declaring green.

## Fleet-PR override

When the PR was opened by an `anvil:dispatching-issue-fleet` subagent, **green CI is not sufficient for merge** — the review-respond loop runs even if the orchestrating user said "merge on green."

**Detection heuristics** (any one is enough; err on the side of running the loop when uncertain):

- Branch prefix `anvil/<slug>`.
- PR body contains the orchestrator's structured marker.
- The orchestrator's report names this PR.

**Rationale (evidence at filing time):**

- PR #37 — single-callsite helper extraction (`listBundledSkills`) shipped by a fleet subagent; CodeRabbit flagged it as a "no helper without second use" violation. Green CI hid it.
- PR #60 — same helper-extraction shape, second occurrence in the 2026-05-15 batch. Same green-CI miss.
- PR #61 — `--remove 1 --remove 1` against `["1","x"]` index-fall-through correctness bug. CodeRabbit caught it and suggested the exact patch. Green CI ran the wrong test.

The override is a workflow-shape constraint keyed off the dispatching skill, not the user's intent.

## Halt at green

After every thread has a reply AND CI is green on the latest SHA AND no new reviewer activity within the poll budget: stop. Surface the PR url. The human merges.

## What NOT to do

- Do not merge. Even on green. Even if the user said "merge on green" and this isn't a fleet PR — confirm intent first.
- Do not skip with "nitpick" when the nit cites a documented repo rule (see Phase 2 nitpick policy).
- Do not paraphrase CodeRabbit's findings in the reply. Cite the SHA; the diff speaks.
- Do not loop past the poll budget without surfacing to the user. A reviewer that never lands is a signal to fall back to local-review, not infinite poll.

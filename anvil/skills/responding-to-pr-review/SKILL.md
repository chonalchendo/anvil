---
name: responding-to-pr-review
description: "Use when a PR has inline review comments to address — CodeRabbit, a human reviewer, or reviewing-pr's output. Triggers: 'respond to the review', 'address coderabbit', 'reply to inline comments', 'babysit PR <n>'."
---

# Responding to PR Review

Your job is to drive every review finding — inline thread or thread-less report — to an outcome (fix / skip / push-back) and CI to green, then surface the PR url back. You do **not** merge — `dispatching-issue-fleet`'s Iron Law (human owns the merge button) applies here too.

## Iron Law

**Every finding gets an outcome — fix, skip-with-reason, or push-back. No silent drops.** For an inline comment the outcome is a thread reply; for a thread-less report finding it is a commit SHA or a line in the post-resolution summary. A missing outcome is a hidden disagreement.

## Review-source-agnostic posture

The same pipeline handles CodeRabbit, a human reviewer, and `reviewing-pr`'s fresh-subagent output. Findings arrive in one of two shapes:

- **Inline thread on a hunk** (CodeRabbit, human) — reply via `gh api .../comments/<id>/replies`.
- **Thread-less structured report** (a `reviewing-pr` subagent's Phase 3 findings, handed in-hand) — no GH thread exists to reply on.

The shape decides only *where the reply lands*, never *whether the finding is evaluated*. Every finding — threaded or thread-less — runs Phase 2's apply / skip-with-reason / push-back. A thread-less blocker gets implemented, not summarized. Routing thread-less findings to a top-level `gh pr comment` *instead of* Phase 2 is the silent-drop this skill forbids; the only legitimate top-level comment is the Phase 3 summary posted *after* each finding is resolved.

Reviewer identity does **not** change the loop. CodeRabbit nitpicks that cite a documented repo rule (e.g. `docs/code-design.md`'s "no helper without second use") get the same treatment as a human asking for the same fix: apply, do not skip.

## Phase 1 — Collect findings

Inline threads come from the API. A `reviewing-pr` report comes in-hand from that skill's Phase 4 handoff (the structured report + subagent id) — there is nothing to fetch for it.

```bash
gh pr view <n>                                              # status, branch, mergeability
gh api repos/<o>/<r>/pulls/<n>/comments \
  --jq '.[] | {id, path, line, user: .user.login, body, in_reply_to_id}'
gh pr checks <n>                                            # CI state
```

If there are zero inline comments AND no thread-less report was handed in AND CI is green AND the PR was not opened by a dispatched subagent (see fleet-PR override below): the review-respond loop is no-op. Surface that and return.

## Phase 2 — Evaluate

Defer evaluation discipline to `superpowers:receiving-code-review`. Per finding — inline thread or thread-less report entry alike — decide one of:

- **Fix.** Implement, commit, push. The commit SHA is the audit record (Phase 3 chooses the channel).
- **Skip with reason.** Record the reason. Examples: "out of scope for this PR — tracked in `<issue-id>`", "intentional per `docs/<...>`".
- **Push back.** State the disagreement with rationale. The reviewer either updates or the human resolves it.

**Nitpick policy:** if a nitpick cites a documented repo rule, **apply** the fix. Do not skip with "nitpick". Memory entry `feedback_coderabbit_read_inline_before_merge` is the rationale — SUCCESS status is non-blocking, not no-findings.

## Phase 3 — Record per finding

For each finding that takes a fix:

1. Edit, commit with a focused message, push.
2. Capture the new commit SHA.
3. If the finding has an inline thread, reply on it: `gh api -X POST repos/<o>/<r>/pulls/<n>/comments/<thread-id>/replies -f body="Fixed in <SHA>: <one-line rationale>"`.

For skip-with-reason or push-back on a threaded finding, reply with the rationale only — no commit needed.

**Thread-less findings have no thread to reply on.** A fix's audit record is its commit SHA; a skip or push-back is recorded in the post-resolution summary below, keyed to the `reviewing-pr` report id. Do not open a top-level comment to *carry* a thread-less finding through evaluation — that is the silent-drop. The summary records outcomes only *after* Phase 2 has decided each one.

After every finding has an outcome, post the top-level summary:

```bash
gh pr comment <n> --body "Addressed N findings as of <SHA> (report <reviewing-pr-id> + threads): <k> fixes, <m> skips-with-reason, <p> push-backs. <one-line residual delta if any>."
```

## Phase 4 — Wait for CI and follow-up review

Instead of polling in-agent (which replays full conversation context on every iteration), invoke the out-of-band poller **once** and act on its result:

```bash
bash ~/.claude/skills/completing-issue/scripts/wait-for-pr.sh --pr <n> [--repo owner/repo] [--timeout 900]
# blocks until: merged | closed | review_blocked | ci_failed | timeout
# emits one JSON: {state, merged, ci_conclusion, review_blockers_count, timed_out}
```

Branch on `state`:
- `merged` or `closed` — done; surface the PR url and return.
- `review_blocked` — re-fetch inline comments and loop Phase 2-3.
- `ci_failed` — investigate the failed check, fix, push, then re-invoke the poller.
- `timeout` — surface to the user; fall back to local-review per the rate-limit fallback below.

Default timeout (900 s / 15 min) aligns with the CodeRabbit rate-limit-fallback policy.

## Rate-limit fallback

CodeRabbit caps per user per hour. If a batched-PR session burns the cap and a PR sits without a CodeRabbit pass beyond the budget, merge on **local-review + CI-green**. Memory entry `feedback_coderabbit_rate_limit_per_hour` records the cap-shape; 5-PR batches reliably hit it.

The fallback is local-review, not zero review. Do a one-pass diff read against the hard-rules list before declaring green.

## Fleet-PR override

When the PR was opened by a `dispatching-issue-fleet` subagent, **green CI is not sufficient for merge** — the review-respond loop runs even if the orchestrating user said "merge on green."

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

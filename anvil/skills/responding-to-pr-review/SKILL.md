---
name: responding-to-pr-review
description: "Use when a PR has review findings to address — a reviewing-pr subagent report or a human reviewer's inline comments. Triggers: 'respond to the review', 'address the findings', 'reply to inline comments', 'babysit PR 42'."
---

# Responding to PR Review

Your job is to drive every review finding — inline thread or thread-less report — to an outcome (fix / skip / push-back) and CI to green, then either surface the PR for the human's merge **decision** or, on an explicit per-PR approval, run the merge **mechanics** yourself: `--land-pr`, then distil + handoff, never raw `gh pr merge`. `running-milestone`'s human-owns-the-merge-button law is preserved as that per-PR decision gate — it bans the agent from *deciding* the merge, not from *executing* one the human just approved.

## Iron Law

**Every finding gets an outcome — fix, skip-with-reason, or push-back. No silent drops.** For an inline comment the outcome is a thread reply; for a thread-less report finding it is a commit SHA or a line in the post-resolution summary. A missing outcome is a hidden disagreement.

## Review-source-agnostic posture

The same pipeline handles `reviewing-pr`'s fresh-subagent report and a human reviewer's inline comments. Findings arrive in one of two shapes:

- **Thread-less structured report** (a `reviewing-pr` subagent's Phase 3 findings, handed in-hand, or recovered from the issue on resume) — the default source; no GH thread exists to reply on.
- **Inline thread on a hunk** (a human reviewer) — reply via `gh api .../comments/<id>/replies`.

The shape decides only *where the reply lands*, never *whether the finding is evaluated*. Every finding — threaded or thread-less — runs Phase 2's apply / skip-with-reason / push-back. A thread-less blocker gets implemented, not summarized. Routing thread-less findings to a top-level `gh pr comment` *instead of* Phase 2 is the silent-drop this skill forbids; the only legitimate top-level comment is the Phase 3 summary posted *after* each finding is resolved.

Reviewer identity does **not** change the loop. A finding that cites a documented repo rule (e.g. "no helper without second use" from the project's code-design guide) gets the same treatment whether the subagent or a human raised it: apply, do not skip.

## Phase 1 — Collect findings

Inline threads come from the API. A `reviewing-pr` report comes in-hand from that skill's Phase 4 handoff (the structured report + subagent id). On resume with no report in hand, run `anvil show issue <issue-id>` and take the latest `## Review findings — PR <n>, round <k> @ <head-sha>` section (if show reports the body truncated, read the file path it names and take the last matching section) — `reviewing-pr` persists every round there. It is a recovery surface only: re-fire `reviewing-pr` for an independent re-review only when `git log <head-sha>..HEAD` holds commits that are not fixes for this round's findings.

```bash
gh pr view <n>                                              # status, branch, mergeability
gh api repos/<o>/<r>/pulls/<n>/comments \
  --jq '.[] | {id, path, line, user: .user.login, body, in_reply_to_id}'
gh pr checks <n>                                            # CI state
```

If there are zero inline comments AND no thread-less report was handed in or persisted on the issue AND CI is green: the review-respond loop is no-op. Surface that and return.

## Phase 2 — Evaluate

Evaluate each finding on technical merit for *this* codebase: verify the claim against the code before implementing, and push back with reasoning where it is wrong rather than performing agreement. Per finding — inline thread or thread-less report entry alike — decide one of:

- **Fix.** Implement, commit, push. The commit SHA is the audit record (Phase 3 chooses the channel).
- **Skip with reason.** Record the reason. Examples: "out of scope for this PR — tracked in `<issue-id>`", "intentional per `docs/<...>`".
- **Push back.** State the disagreement with rationale. The reviewer either updates or the human resolves it.

**Nitpick policy:** if a finding flagged as a nit cites a documented repo rule, **apply** the fix. Do not skip with "nitpick" — a low-severity band is not a no-finding.

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
# blocks until: merged | closed | conflicting | ci_passed | review_blocked | ci_failed | timeout
# emits one JSON: {state, merged, ci_conclusion, review_blockers_count, timed_out}
```

Branch on `state`:
- `merged` or `closed` — done; surface the PR url and return.
- `conflicting` — rebase or merge the base branch, force-push, then re-invoke the poller.
- `ci_passed` — CI green, no blockers, PR unmerged; proceed to Phase 5 (merge gate). CI green is not the verdict: Phase 5 runs `anvil verify` before it lands.
- `review_blocked` — re-fetch inline comments and loop Phase 2-3.
- `ci_failed` — investigate the failed check, fix, push, then re-invoke the poller.
- `timeout` — surface to the user; a human reviewer that never lands is their call, not an infinite poll.

The default 900 s / 15 min timeout is a poll budget, not a merge deadline.

## Phase 5 — Merge gate (per-PR decision)

After every finding has an outcome AND CI is green on the latest SHA AND no new reviewer activity within the poll budget, verify before you ask. At the pushed head, rebuild with the project's build command from CLAUDE.md. Put any worktree-local binary first on PATH. Run the verb from the worktree. Your run is the human's evidence at the prompt; the land re-runs the verification itself on the PR head, and a pass here does not skip that.

```bash
cd <worktree-path> && anvil verify <issue-id> --json | jq -r .verdict
```

`pass` goes to the prompt. `fail` returns to Phase 2. A `verification_changed` refusal goes to the human; never run `--accept-change`. Never ask the human to merge on red.

Then present the PR and ask — its own paragraph, do not bundle:

> PR #`<n>` (`<issue-id>`) is verdict-pass, review-green and CI-green: `<title>`. Merge it? (yes / skip / hold for changes)

**Wait for the response.** This is the one preserved human gate — the merge *decision*. "Merge on green" said earlier does not pre-authorize it; each PR gets its own explicit go, and silence is never approval.

- **`skip` / `hold`** → leave the PR open, surface the url, return. The human drives it later.
- **`yes`** → run these steps in order, then stop. Never raw `gh pr merge`.

1. **Land** — the gated verb that merges and resolves in one call:

   ```bash
   anvil transition issue <issue-id> resolved --land-pr <n>
   ```

   It runs the issue's verification on a fresh checkout of the PR head and merges only on its pass, on an intact lock, for the issue's own branch, mergeable and CI-green; a red run refuses `land_pr_verification_failed` and the PR is untouched; it then removes the worktree, squash-merges, confirms MERGED, and resolves the issue. `<issue-id>` is the issue whose completion opened this PR — branch `<project>/<issue-slug>`, url stamped via `anvil link issue <issue-id> --external`. On a branch-only setup the verb refuses (`land_pr_worktree_missing`); the human merges instead.
2. **Distil** — fire `distilling-learning` (REQUIRED SUB-SKILL) in its attended autonomous mode: **auto-distil compounding learnings**, no offer or prompt. Hold its **high-value bar**: distil only when you can name the specific future failure it prevents; most runs clear it for nothing, and a marginal learning pollutes retrieval, so default to silence.
3. **Hand off** — fire `handing-off-session` (REQUIRED SUB-SKILL): write the load-ready handoff of what landed and what's still open.
4. Surface the merged PR url.

## Autonomous / dispatched mode never reaches the gate

The land-on-approval steps run only on an explicit per-PR approval typed by a human. A fleet- or `anvil-issue-worker`-dispatched completion stops at PR-opened and never fires this skill, so the gate is unreachable without an interactive human; an autonomous orchestrator owns its own post-merge. No standing or blanket pre-approval authorizes a later PR — every merge decision is per-PR.

## What NOT to do

- Do not merge unprompted, and never via raw `gh pr merge` — land only through `--land-pr` after an explicit per-PR approval. "Merge on green" said once does not pre-authorize the batch; each PR needs its own go.
- Do not skip with "nitpick" when the nit cites a documented repo rule (see Phase 2 nitpick policy).
- Do not paraphrase the reviewer's findings in the reply. Cite the SHA; the diff speaks.
- Do not loop past the poll budget without surfacing to the user. A human reviewer that never lands is a signal to surface to the user, not infinite poll.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.

---
name: running-milestone
description: "Use when the human asks to run, work or continue an in-progress milestone to its finish line. Triggers: 'run the milestone', 'continue the milestone', 'work milestone X'. Not for one issue (completing-issue)."
---

# Running Milestone

Your job is to run one in-progress milestone to its finish line. You derive a few issues, dispatch each, review each PR, land on the human's approval, and read the next state from the vault. You write no code. Every exit returns to the human.

## Iron Law

**The human owns the merge button and the milestone gates. Never call `gh pr merge` or `git worktree remove`. Never call `anvil transition issue <id> abandoned`. Run `resolved` only after the human approves that PR.** This binds your subagents too. A subagent that breaks it is a halt. Surface it.

## Phase 0 — Read the state

Read the repo's `CLAUDE.md` or `AGENTS.md` for build, install and verification commands. Then read the vault, never session memory:

```bash
anvil show milestone <milestone-id> --body
anvil milestone status <milestone-id> --json
anvil fleet status --json
anvil list issue --ready --milestone <project>.<slug> --json
```

`milestone status` runs the acceptance predicates in the project's main checkout with your privileges. Run it only on the base branch tip.

The milestone must be `in-progress`. If it is `planned`, halt: the human approves the milestone gate first.

Read the end goal, the finish line, the design change and the issue limit in the milestone body. A fresh session continues from this read alone.

## Phase 1 — Derive the next wave

Derive a few issues, never the whole milestone. Each issue traces to one acceptance criterion that has no resolved issue.

1. Count the milestone's issues. If the next wave would pass the limit, take Exit 3.
2. Read the current code under the milestone's components. Prior issues have landed, so the code differs from the design.
3. Fire `writing-issue` for each next issue. It writes the verification and the ready gate checks it.

Skip this phase when ready issues already cover the open criteria.

## Phase 2 — Pick the work set

Take the ready issues from `anvil list issue --ready --milestone <project>.<slug> --json`. Respect `depends_on`: dispatch an issue only after its blockers resolve.

**Overlap check.** Read each candidate's declared files. On a collision, serialize: the loser waits for the next wave. Dispatch in parallel only when the file sets are disjoint.

**Never stack.** Never base a dependent issue on its predecessor's branch. `--land-pr` deletes that branch and closes the dependent PR for good. Land the predecessor, then cut the dependent from the base branch tip.

## Phase 2b — Retrieve learnings once

A worker cannot dispatch a sub-subagent, so retrieve once, before fan-out. Dispatch `anvil-learnings-researcher` with a `<work-context>` that names the milestone in `artifacts:` and the union of the candidates' `domain/` tags. Reduce the return to one line. Put it in each worker's prompt.

## Phase 3 — Dispatch the workers

Claim and cut each worktree before dispatch, one call per issue:

```bash
anvil transition issue <id> in-progress --owner <name> --cut-worktree
```

Dispatch each issue with `subagent_type: anvil-issue-worker`. The agent file holds the worker contract. Fill only these values:

> Complete anvil issue `<issue-id>`. Worktree: `<worktree-path>` on branch `<branch>`. Declared files (estimate, grep to confirm): `<declared-files>`. Prior learnings (gist): `<one line or "none">`.

Dispatch at most eight workers in one wave. Dispatch parallel issues in one tool-use block. A new or edited agent file is not dispatchable until the session restarts.

After dispatch, end the turn. The completion notification resumes you. Do not use `Monitor`. Nudge a worker once with `SendMessage` only when a sibling finished and it has no PR url or verdict file.

## Phase 4 — Read each return

The last line of a worker's return is one of three shapes.

- A PR url. Go to the verdict gate below.
- `Blocker: <reason>`. Take Exit 1. Do not re-dispatch.
- Anything else. The worker died or returned prose. Read `git log --stat <branch>` for its `wip:` commits. Re-dispatch once with an action-only prompt that builds on them. After a second failure, take Exit 1.

**Verdict gate.** The worker writes the runner's output to `/tmp/verdict.<issue-id>.json`, where `<issue-id>` is the full id (for example `issue.acme.0042.fix-login`). Run `jq -r .verdict` on it. A value of `pass` goes to review. Any other value, a missing file, or a prose excuse means re-measure:

```bash
cd <worktree-path> && anvil show issue <issue-id> | bash <run-verification.sh> | jq -r .verdict
```

`<run-verification.sh>` is the `completing-issue` skill's `scripts/run-verification.sh`. Rebuild with the project's build command from CLAUDE.md. Put any worktree-local binary first on PATH. Green on re-measure: proceed and note it in the report. Red on re-measure is a blocker: take Exit 1. Never accept a prose account of a red check in place of the verdict.

## Phase 5 — Review each PR

1. Fire `reviewing-pr` on the PR. Do not let it fire `responding-to-pr-review` in your session. The fixes live in a worktree you are not in.
2. Route the findings. Findings at low or below with CI green: the PR is ready. Any blocker, high or actionable medium finding: dispatch `anvil-pr-responder` into the PR's worktree. Hand it the issue id, worktree path, branch and findings. End the turn. On the responder's return: a `Blocker:` line takes Exit 1. A PR url re-runs the Phase 4 re-measure at the new head, then returns to step 1 of this phase.
3. Count the responder's resolution summaries on the PR (`gh pr view <n> --comments`). A third round with findings left is the round limit: take Exit 1.
4. Confirm CI green. Wire any rail edge that the PR's `## Context box` names in a `swept` row. Do not merge.

## Phase 6 — Land on approval

Present each ready PR to the human with its verdict, review result and CI state. Wait for the human's approval of that PR. Then land it from the parent checkout:

```bash
anvil transition issue <id> resolved --land-pr <n>
```

The verb checks the gates, merges, confirms the merge, removes the worktree and resolves the issue. Never run `gh pr merge` yourself.

## Phase 7 — Continue

After each landing, fast-forward the parent checkout to the base branch tip (`git pull --ff-only`). `milestone status` measures that checkout. Then re-read `fleet status --json` and `milestone status --json` from the vault. Never use session memory.

- Open criteria remain: return to Phase 1.
- The finish line is green: harvest learnings (Phase 8), then stop at the acceptance gate.

Do not run `anvil transition milestone <id> done`. Report the green finish line and let the human accept.

## Exits

Each exit is a halt that returns to the human with a reason. There is one escalation path: the milestone gate.

1. **Issue escalated.** A worker blocker or the round limit. Run `anvil transition issue <id> escalated --reason "<reason>"` if the issue is still `in-progress`. Offer: amend, cut scope or abandon.
2. **Milestone wrong.** The issues show the end goal, finish line or design change is wrong. Propose `anvil transition milestone <id> planned` and the amendment. Do not edit the milestone yourself.
3. **Issue limit reached.** The milestone body names the limit. If the next wave would pass it, propose `anvil transition milestone <id> planned` with a re-scope, then stop.

## Phase 8 — Harvest learnings

Run this after the human's landings. Collect gotchas, confirmed approaches and dead ends from the landed PRs only. Flag any cross-PR breakage on the base branch. Fire `distilling-learning` in attended autonomous mode. Distil only when you can name the future failure the learning prevents. Most milestones yield one or none.

An unattended orchestrator does not distil. It lists `Harvest candidates:` in the report and stops.

## Report

```text
Milestone <id>: <N> issues resolved of <M>
  <issue-id> → <PR url> [ready | landed | escalated: <reason>]
Finish line: <green | red: <failing check>>
Exit: <none | 1 | 2 | 3> <reason>
To land each ready PR:
  anvil transition issue <id> resolved --land-pr <n>
```

## What NOT to do

- Do not land a PR without the human's approval of that PR. A blanket "merge on green" is not approval.
- Do not derive the whole milestone at once.
- Do not re-dispatch a `Blocker:` return.
- Do not write code or edit the milestone body.
- Do not narrate the dispatch. The report is the deliverable.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.

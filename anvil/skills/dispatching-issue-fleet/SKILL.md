---
name: dispatching-issue-fleet
description: "Use when the user wants to dispatch parallel subagents to work multiple ready issues end-to-end through PR review-green. Triggers: 'fleet', 'dispatch a batch', 'fan out subagents', 'work the next N issues in parallel'."
---

# Dispatching Issue Fleet

Your job is to orchestrate N parallel subagents through claim → implement → smoke → PR (each subagent's half), then fire the independent review on every returned PR and drive its findings to resolution (your half — Phase 5), halt at green so the human can merge, and — once the batch lands — offer a post-merge distillation harvest (Phase 6, the write-side learning crossbar). You do not write code yourself; you dispatch, audit returns, and present a structured report.

## Iron Law

**The human owns the merge button. The orchestrator and its subagents never call `gh pr merge`, `git worktree remove`, or `anvil transition resolved`.** Post-green cleanup is the human's, period. A subagent that calls any of these is a halt — surface the violation, do not paper over it.

## Phase 1 — Pick the work set

Two modes, resolved from the invocation arguments:

**Curated id list (preferred when the caller has already triaged):** if the invocation supplies an explicit id list — e.g. `--ids issue-a issue-b issue-c` or an inline JSON array — use those ids directly as the candidate set, skipping the `anvil list` query. Verify each id resolves (`anvil show issue <id>`) and is not already `resolved` or `in-progress` by another owner; drop any that don't. This is the path a triaged subset (e.g. from `self-testing`'s Phase 5 gate) should use so the curated work is not discarded and re-picked.

**Default ready set (fallback when no id list is given):**

```bash
anvil list issue --ready --project <p> --status open --json
```

Take the first `--max N` candidates.

In both modes, `--max` (default `5`, range 1–8) and `--allow-overlap` are parsed from the invocation arguments, **not** CLI flags — there is no `anvil fleet` verb. A curated `--ids` list is exempt from `--max` (the caller already triaged), so every supplied id dispatches.

## Phase 2 — Pre-dispatch overlap check (declare-then-check)

Each candidate issue declares the files it anticipates touching (read the issue body's `files:` hint or the linked plan's task `files[]`). Compare each candidate's set against every other candidate's. On collision, default-serialize: drop the loser to the next wave. Opt in to parallel collision with `--allow-overlap` (rare — only when the user has eyeballed the overlap and accepts the merge-conflict cost).

The overlap check is one-line declarations plus eyeball compare — pre-dispatch only. Per-worker post-edit enforcement is handled by `anvil fleet scope-audit` inside each worker before its PR opens (see Scope-change pause protocol).

## Phase 2b — Retrieve prior learnings once (before fan-out)

The fleet worker is a subagent and cannot dispatch a sub-subagent, so per-worker retrieval is impossible by topology. Retrieve **once in the orchestrator**, before fan-out, and inject the gist into every worker's dispatch prompt — correct by topology and cheaper (one retrieval, N workers). Dispatch `anvil-learnings-researcher` via `subagent_type` with a `<work-context>` built from the batch's shared milestone and the union of the candidates' domains:

```text
<work-context>
work: fleet of <N> issues under [[milestone.<project>.<slug>]]
domain: <union of candidate domain/ tags>
activity: activity/issue
artifacts: [[milestone.<project>.<slug>]], <candidate issue ids>
</work-context>
Return the findings that genuinely bear on this batch, highest-precision first.
```

The milestone belongs in `artifacts:`, not just the `work:` prose — that is what the agent's link-graph pass queries edges against. Distil the return (or `Findings: none`) to one line and inject it into each worker's dispatch prompt below.

## Phase 3 — Dispatch N subagents

For each surviving candidate, fire one subagent via the Agent tool with `subagent_type: anvil-issue-worker` — the bundled, cost-tuned worker (`anvil/agents/anvil-issue-worker.md`: runs on a cheaper model than the orchestrator, `completing-issue` preloaded). The agent file **is** the worker contract — implement → smoke → `gh pr create`, stop-at-PR with no review loop, pre-edit worktree invariant, scope-change Blocker, forbidden-call audit, structured return line — so you do not re-state it per call. **Claim and cut each candidate's worktree before dispatching**, one atomic call per candidate:

```bash
anvil transition issue <id> in-progress --owner <name> --cut-worktree
```

This claims the issue `in-progress` (stamping an owner) *and* emits the worktree path — so the issue never sits `open` through the run. The worker arrives pre-claimed and skips its own Phase 0 claim: it is anonymous (no owner to claim under) and a bare `--cut-worktree` would re-cut a duplicate. The agent works in `<worktree-path>` and halts if it is absent (its pre-edit invariant refuses to cut its own). Fill only the per-call values into the dispatch prompt body:

> Complete anvil issue `<issue-id>`. Worktree: `<worktree-path>` on branch `<branch>`. Declared files (estimate, grep to confirm): `<declared-files>`. Prior learnings (gist): `<one-line distillation from Phase 2b, or "none">`.

A worker stops at PR opened — it cannot dispatch the reviewer sub-subagent, so review is the orchestrator's job (Phase 5).

Dispatch all N in a single tool-use block so they run in parallel. **Restart caveat:** the Agent tool enumerates `subagent_type` values at session start, so a freshly installed or edited `anvil-issue-worker` (via `just install` && `anvil install agents`) is not dispatchable until the next restart. If dispatch errors with "Agent type not found", restart the session once, then retry.

## Phase 4 — Interpret returns

Each subagent's last line is structurally one of:

- `^https://github\.com/.+/pull/[0-9]+$` — PR url. Proceed to Phase 5 for this PR.
- `^Blocker: .+$` — explicit blocker. Record, surface to user, do not re-dispatch.
- Anything else — **malformed return** (narrative-as-final-output). This is the recurring 100-200 LOC stall pattern (sessions 2026-05-13, 2026-05-14, 2026-05-15 all hit it). Re-dispatch action-only: a step-by-step plain-text prompt with **no skill wrapper**, naming the exact next commit + push + PR commands. If the second dispatch also malforms, fall back to main-session takeover for that issue.

**Expected miss-rate: 1 in N falls back to main-session takeover.** Surface this in the final report so the human reads a stall as design-anticipated, not a tool bug.

## Phase 5 — Review each PR, then halt at green

For each PR url returned, in turn:

1. **Fire the independent review.** Run `reviewing-pr` against the PR. It dispatches a fresh reviewer subagent (one level down from you — the same topology as the single-PR path) and returns structured findings. This is the only independent review source post-CodeRabbit; the fleet worker can't fire it (a subagent can't dispatch a sub-subagent), which is why it runs here.
2. **Route findings — fleet override.** `reviewing-pr` Phase 4 would fire `responding-to-pr-review` in-session; on the fleet path you do **not** — the fixes live in a worktree you are not in, and you don't write code. Take its findings and route them yourself:
   - All findings ≤low + CI green → the PR is ready for the human's merge decision.
   - Any blocker/high/actionable-medium → **dispatch a fresh worker into the PR's worktree**, tasked with `responding-to-pr-review` against the handed findings (the structured report + reviewer subagent id). This worker is a plain subagent (not the `anvil-issue-worker` agent — wrong skill); its contract is `subagent-prompt.md` (worktree invariant, return contract, forbidden-call audit). Interpret its return exactly as in Phase 4.
3. **Halt.** Confirm CI green. Do not merge.

Present the structured report:

```text
Fleet of <N> dispatched:
  <issue-id> → <PR url> [green, reviewed — no actionable findings]
  <issue-id> → <PR url> [green, reviewed — findings addressed]
  <issue-id> → Blocker: <reason>
  <issue-id> → main-session takeover (subagent malformed twice)

Expected: 1 in <N> stalls. Observed: <k>.

To land each PR, run from the parent checkout (one line per issue):
  anvil transition issue <id> resolved --land-pr <n>
```

The verb gates on mergeable + CI-green, removes the worktree, squash-merges, verifies MERGED, and resolves the issue atomically (see `completing-issue` Phase 5) — no manual `git worktree remove` / `gh pr merge` sequencing. Once the human has landed the batch, run Phase 6.

## Phase 6 — Post-merge distillation harvest (offer, don't force)

Phase 2b wired the read-side crossbar (retrieve learnings before fan-out); this closes the **write-side** for dispatched work. A worker stops at PR-opened, so it never reaches `completing-issue`'s Phase 6 distillation offer (an explicit no-op in dispatched mode), and per-worker distillation is impossible by topology (a subagent can't dispatch a sub-subagent). The learnings a worker surfaces are therefore lost unless harvested here — at the orchestrator level, once the batch lands. Throughput already scales learning *consumption*; this is what scales their *production*.

Fire this **after** the human has run the `anvil transition … resolved --land-pr` calls from Phase 5 — not at green. Learnings from PRs that never land have lower value, so harvest only what merged. Mirror the read-side: **one** orchestrator-level pass over the whole batch, not per-worker.

1. **Collect candidate material — landed PRs only.** For each issue that produced a PR url in Phase 4 *and* has since merged, gather what a future agent would act on differently: a gotcha hit, a confirmed approach, a dead end avoided. Sources are the merged PR (`gh pr view <n>`), the resolved issue, and your own Phase 5 review observations. The guard from Phase 4 carries forward — a `Blocker:` or malformed return emitted no PR, so it contributes no material; confirm the merged PR before collecting. Also flag any **cross-PR breakage** you observed — sequential merges of individually-green PRs can land a broken `master` — as a first-class harvest candidate.
2. **Offer — do not force — a single handoff to `distilling-learning`** over the collected candidates:

   > This fleet landed `<k>` PRs. Distill learnings from the batch? (`distilling-learning`)

   The bar is **compounding, not record-keeping**: distill only what a future agent would be retrieved into and act on. "Nothing worth distilling" is a valid answer.
3. **The human gate is non-negotiable** (`distilling-learning`'s Iron Law). The harvest *offers* and stages candidate material for the human-validated capture step; it never auto-distills, even unattended.

**Autonomous orchestrator (unattended runs):** make no blocking offer — stage the candidate list into the final report under a `Harvest candidates:` block and stop, consistent with the dispatched-completion-stops-at-PR-opened convention. A human (or a later session) fires `distilling-learning` over it.

## Scope-change pause protocol

A worker runs `anvil fleet scope-audit` against its branch before opening a PR (see `anvil-issue-worker.md` — Scope-change check). Out-of-scope files surface as a `Blocker:` return; the worker never opens the PR. If a subagent returns such a Blocker, do not re-dispatch: surface the named files to the user and let them decide — split the issue, expand the declared set, or abort. The worker's per-file gate is the deterministic enforcement point; this protocol handles the orchestrator's response to it.

## Forbidden calls (orchestrator AND subagents)

Never invoke:

- `gh pr merge` — human owns the merge button.
- `git worktree remove` — post-merge cleanup is the human's.
- `anvil transition resolved` — the future tool-side gate (`anvil-transition-resolved-refuses-when-an-open-pr-is-linked`) will catch this, but until it lands the human is the v0.1 enforcer.

The subagent prompt echoes this checklist verbatim in its final structured report so we can audit non-execution.

## What NOT to do

- Do not merge. Even on green, even with one line of review findings, even when the human said "merge on green" — the Phase 5 review pass runs first.
- Do not dispatch >8 subagents. Context cost on the orchestrator side outpaces the time savings past 8.
- Do not re-dispatch a `Blocker:` return. The subagent declared inability; respect it.
- Do not narrate the dispatch. The final report (Phase 5) is the deliverable.

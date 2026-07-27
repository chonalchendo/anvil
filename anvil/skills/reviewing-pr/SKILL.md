---
name: reviewing-pr
description: "Use to gate every PR before merge with an independent review. Triggers: 'review this PR', 'review PR 42', 'self-review', or a freshly opened PR."
---

# Reviewing PR

Your job is to dispatch a **fresh `anvil-pr-reviewer` subagent** that reviews one PR against the repo's standards, and to surface its findings so `responding-to-pr-review` can drive them to resolution. You do not review the PR yourself — independent context is half the value.

The review recipe is **not** here — the `anvil-pr-reviewer` contract owns it end to end: the rubric gate, the judgment axes, the findings format, and the bar they measure against. This skill owns what is genuinely orchestrator-side: when to fire, the size check, the dispatch fill-ins, and routing the findings that come back.

## Iron Law

**Review in a fresh subagent, not in this session.** The author's reasoning chain biases the review. If you find yourself reading the diff and forming an opinion before dispatch, stop — that's the failure mode this skill exists to prevent.

## When to fire

This is the default independent-review gate: fire on **every PR** before merge, right after `completing-issue` opens it. Explicit triggers ("review PR <n>", "self-review") fire it directly.

Deterministic checks (CI lint/format/tests, prek) cover data-integrity findings; this subagent targets the maintainability / code-design dimensions that need judgment. CI green is necessary but not sufficient — the merge decision waits on this review.

## Phase 1 — Fetch PR shape

```bash
gh pr view <n> --json number,title,headRefName,baseRefName,files,additions,deletions
gh pr diff <n>
```

If the diff is >800 LOC or touches >10 files, surface the size to the user before dispatch — large PRs warrant a split conversation, not a bigger review.

## Phase 2 — Dispatch fresh subagent

Fire one Agent-tool call with `subagent_type=anvil-pr-reviewer` — the bundled `anvil-pr-reviewer` agent definition. Its frontmatter pins the review **model** so the review never silently inherits this session's, and its body owns the review itself — it loads the context box with `anvil hydrate`, sweeps for governing artifacts nobody linked, and carries the judgment axes, findings format, and return contract. Restating any of it here is how the two copies drift; the dispatch prompt carries **fill-ins only**:

- PR number and repo
- the linked issue id (from the PR body's reference or the branch slug) — or state that none resolves
- the PR's worktree path
- any PR-specific review dimension this diff warrants beyond the standing axes

It does **not** get this session's conversation.

Fallback: if the agent type is unavailable (freshly installed agents need a session restart), dispatch `subagent_type=general-purpose` with a `model` override matching the reviewer definition and that contract's body pasted verbatim as the prompt — never a hand-assembled substitute.

## Phase 3 — Findings contract

The report shape, the severity bands (**blocker** / **high** / **medium** / **low**), and the rule that an uncited finding drops one band all belong to the reviewer contract — it applies them, this skill only interprets them below. Its return closes with three accounting lines, `Context loaded:` / `Verification:` / `Findings:`, which are what Phase 4 routes on.

## Phase 4 — Interpret findings

Read the subagent's report and route:

- **All findings ≤low and CI green** — surface "no actionable findings" to the user; the PR is ready for the human's merge decision.
- **Any blocker/high, or actionable medium** — fire `responding-to-pr-review`, handing it **the structured report (Phase 3 findings) and the subagent id**. These findings are thread-less, so its loop drives each through apply / skip-with-reason / push-back exactly as it does a human reviewer's inline threads — a blocker gets implemented, not summarized. The subagent id keys the post-resolution summary so the audit trail survives the handoff.
- **`Context loaded:` reports a gap that plainly resolves** — hydrate failed on a dangling spine edge, or the sweep skipped a contract/convention that governs the touched files. The review judged the diff with part of its context shut; re-dispatch rather than accept it. A gap the line *justifies* ("no contract governs these files") is a recorded fact, not a defect — and a missing rail the reviewer reports is a link to repair, not a re-dispatch.
- **Subagent malformed return** (not the reviewer contract's findings format) — re-dispatch once with a tightened prompt naming the format verbatim. If the second dispatch also malforms, stop and surface a handoff-required failure to the user; log the malformation via `anvil create inbox` and wait for manual review or a later retry. Do **not** fall back to main-session review — that defeats the Iron Law.

Do **not** silently drop findings the subagent surfaced. A finding you judge wrong or out-of-scope goes in an explicit **Dismissed** bucket in the report you surface — the finding plus a one-line reason — kept visible so the human can override; disagreement is recorded, not erased. Findings you act on route through the responding-to-pr-review loop. The audit trail matters more than the disagreement.

## What NOT to do

- Do not review the PR in this session. Dispatch.
- Do not skip the review because CI is green. CI is necessary, not sufficient; the merge decision waits on this review pass.
- Do not restate the rubric, standards content, or doc paths in the dispatch prompt — the reviewer contract owns all of it and follows `CLAUDE.md` to this project's standards itself. Fill-ins only; a hand-assembled rubric is the divergence this split exists to end.
- Do not merge. `dispatching-issue-fleet`'s Iron Law applies — human owns the merge button.
- Do not skip findings with "nitpick" when the finding cites a documented repo rule. Same nitpick policy as `responding-to-pr-review`.

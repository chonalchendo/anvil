---
name: reviewing-pr
description: "Use when an Anvil PR needs an independent code review and CodeRabbit is unavailable (rate-limited/paused) or the user invokes a self-review. Triggers: 'review this PR', 'self-review PR <n>', 'coderabbit fallback review'. NOT auto-fired."
---

# Reviewing PR

Your job is to dispatch a **fresh general-purpose subagent** that reviews one PR against the repo's standards, and to surface its findings so `anvil:responding-to-pr-review` can drive them to resolution. You do not review the PR yourself — independent context is half the value.

## Iron Law

**Review in a fresh subagent, not in this session.** The author's reasoning chain biases the review. If you find yourself reading the diff and forming an opinion before dispatch, stop — that's the failure mode this skill exists to prevent.

## When to fire

- Explicit: user says "review PR <n>", "self-review", "coderabbit fallback".
- Implicit fallback: CodeRabbit rate-limit hit (see `feedback_coderabbit_rate_limit_per_hour`), or the PR has sat past the local-review budget in `docs/worktree-workflow.md` with no CodeRabbit pass.

Do **not** fire on every PR. Deterministic checks (CI lint/format/tests, prek) cover most of CodeRabbit's data-integrity findings; this skill targets the maintainability / code-design dimensions that need judgment.

## Phase 1 — Fetch PR shape

```bash
gh pr view <n> --json number,title,headRefName,baseRefName,files,additions,deletions
gh pr diff <n>
```

If the diff is >800 LOC or touches >10 files, surface the size to the user before dispatch — large PRs warrant a split conversation, not a bigger review.

## Phase 2 — Dispatch fresh subagent

Fire one Agent-tool call with `subagent_type=general-purpose`. The subagent gets the PR number, branch, and the rubric below. It does **not** get this session's conversation.

The subagent prompt names the standards by path and instructs the subagent to read them as needed:

- `CLAUDE.md` — hard rules section (no helper without second use, no abstraction without need, no defensive code, no whole-file Read >150 lines without grepping).
- `docs/code-design.md` — module/API shape, refactor discipline.
- `docs/go-conventions.md` — imports, error handling, `log/slog`, `cmd.Println`/`cmd.PrintErrln`.
- `docs/agent-cli-principles.md` — only when the PR touches an `anvil` verb.
- `docs/skill-authoring.md` — only when the PR touches `skills/*/SKILL.md`.
- `docs/test-conventions.md` — only when the PR touches `*_test.go`.

Do not restate any of these in the dispatch prompt. The subagent reads them directly. Restating burns context and drifts from source.

## Phase 3 — Findings contract

The subagent returns a structured report with one entry per finding:

```text
[<severity>] <path>:<line> — <one-line claim>
  Cite: <doc path or CLAUDE.md rule>
  Suggest: <concrete patch or "surface to author">
```

Severity bands (the subagent applies these; this skill interprets them downstream):

- **blocker** — correctness bug, security issue, or hard-rule violation that would land a regression. Always fix before merge.
- **high** — design smell with a named doc citation (e.g. "helper extracted for one callsite" → `code-design.md`). Default: fix.
- **medium** — quality nit with a citation. Default: fix if cheap, surface if it requires judgment.
- **low** — style/taste, no doc citation. Default: surface, do not fix.

A finding without a doc citation drops one severity band. Unsourced opinions are low at best.

## Phase 4 — Interpret findings

Read the subagent's report and route:

- **All findings ≤low and CI green** — surface "no actionable findings" to the user; the PR is ready for the human's merge decision.
- **Any blocker/high, or actionable medium** — fire `anvil:responding-to-pr-review`. Its loop treats these the same as CodeRabbit threads (apply, skip-with-reason, or push back). Cite the subagent's report id in the top-level comment so the audit trail survives.
- **Subagent malformed return** (not the structured format above) — re-dispatch once with a tightened prompt naming the format verbatim. If the second dispatch also malforms, stop and surface a handoff-required failure to the user; log the malformation via `anvil create inbox` and wait for manual review or a later retry. Do **not** fall back to main-session review — that defeats the Iron Law.

Do **not** silently drop findings the subagent surfaced. If you disagree, push back in the responding-to-pr-review loop — the audit trail matters more than the disagreement.

## What NOT to do

- Do not review the PR in this session. Dispatch.
- Do not auto-fire on every PR. CodeRabbit + CI is the default review pipeline; this is a fallback.
- Do not restate the standards docs in the dispatch prompt — name the paths, the subagent reads them.
- Do not merge. `anvil:dispatching-issue-fleet`'s Iron Law applies — human owns the merge button.
- Do not skip findings with "nitpick" when the finding cites a documented repo rule. Same nitpick policy as `anvil:responding-to-pr-review`.

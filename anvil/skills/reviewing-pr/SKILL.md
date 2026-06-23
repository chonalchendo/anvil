---
name: reviewing-pr
description: "Use to gate every PR before merge with an independent review. Triggers: 'review this PR', 'review PR 42', 'self-review', or a freshly opened PR."
---

# Reviewing PR

Your job is to dispatch a **fresh general-purpose subagent** that reviews one PR against the repo's standards, and to surface its findings so `responding-to-pr-review` can drive them to resolution. You do not review the PR yourself — independent context is half the value.

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

Fire one Agent-tool call with `subagent_type=general-purpose`. The subagent gets the PR number, branch, and the rubric below. It does **not** get this session's conversation.

The subagent reads **this project's own** standards — never a hardcoded doc path. The same skill ships into a Go repo, a Python repo, or any other; the entry point is always `CLAUDE.md`:

- Instruct the subagent to read `CLAUDE.md` (and `AGENTS.md` if present) first — its hard-rules section, then the convention docs it indexes (code design, language conventions, CLI/API principles, test conventions, skill authoring — whichever this project defines).
- Scope the read to the diff: follow CLAUDE.md's index to the docs governing the files the PR touches (the test-convention doc when it touches tests, the skill-authoring doc when it touches skill files), not the whole tree.

Do not name `docs/<x>.md` paths or restate their content in the dispatch prompt — hardcoding one repo's layout dangles in every other, and restating burns context and drifts from source. Name the entry point (`CLAUDE.md`); the subagent follows its index.

### Contract rubric

Before dispatching, load the contracts linked to the PR's issue (via the routing link `writing-issue` establishes) — resolve them from the issue's own routing links, not the vault-wide list:

```bash
anvil show issue <issue-id> --json \
  | jq -r '.related[]? | select(startswith("[[contract.")) | ltrimstr("[[contract.") | rtrimstr("]]")'
anvil show contract <id> --body   # for each id printed
```

Instruct the subagent to treat each contract's `does not` constraints as a **blocker-severity rubric**: any diff line that crosses a `does not` boundary is a blocker finding, cited against the contract id and the specific constraint text. If no contract links resolve (issue has no routing link, or the issue cannot be found), skip this step — the rubric is empty, not an error.

### Goal validation

Beyond reading the standards docs, the dispatch prompt carries one explicit task — the standards are necessary but not sufficient, and a clean diff can still fail to deliver the issue it closes. Instruct the subagent to resolve the PR's linked issue — from the PR body's issue reference or the branch slug — and run `anvil show issue <id> --json` to read its `goal:`, the one-sentence terminal predicate. It judges whether the diff plainly achieves that goal and reports a shortfall as a Phase 3 finding — **blocker** when the goal is plainly unmet. When the issue also carries `acceptance[]` (an optional prose aid post-`goal:`), check each criterion too. Name the lookup; do not paste the goal or ACs into the dispatch prompt (the subagent fetches them — same context discipline as the standards docs).

`## Verification` is the binary gate `completing-issue` already ran; this step adds the judgment `goal:` needs and a binary check cannot give.

If no linked issue resolves, the subagent records that it could not and skips goal-validation rather than inventing a target.

### Structural simplification

The standards docs catch rule violations; they miss working-but-needlessly-complex code that breaks no documented rule — a diff can be correct, CI-green, and still a tangle. Instruct the subagent to also ask, per meaningful change: is there a behavior-preserving reframing that deletes whole branches, helpers, or layers? Does an added abstraction earn its keep, or is it a pass-through? Did a cohesive module get more coupled or stateful? A simplification finding that cites a Hard Rule (`no abstraction without need`, `no helper without a second use`, `context is scarce`) is a cited finding — **high**, not a taste nit. Scope the suggestion to naming the simpler shape; a reviewer flags it, it does not authorize a refactor beyond the PR's goal.

The underlying bar is: code a human or agent can reason about — atomic, composable, simple. Atomic means one concern in one place; composable means parts snap together without hidden coupling; simple means the least machinery that solves the problem. Instruct the subagent to measure the diff against this bar.

Before the subagent reads the diff, instruct it to establish the design principles **already on display** in the codebase: read 1–2 sibling implementations of the same component type the PR touches (e.g. a sibling command, handler, task plugin, skill body), derive the house shape from those siblings, then judge the diff for conformance. Documented conventions lag the code; live siblings are the freshest spec. A deviation from sibling shape is a cited finding — **high** when it adds coupling or layers the siblings avoid, **medium** when it is a style inconsistency with no coupling cost (cite the sibling `file:line` whose shape the diff deviates from).

### Documentation staleness

A diff can be correct and still leave the project's docs lying. Instruct the subagent to check, per change that alters an observable contract — a CLI flag or command name, a path, an output shape, a config key, a documented default or behaviour — whether the matching documentation moved with it: `README`, `CLAUDE.md`/`AGENTS.md`, the project's `docs/`, and any skill body that describes the changed surface. Documentation that now contradicts shipped behaviour is a cited finding — **high** (cite the stale `file:line` against the diff). A doc that needs updating but does not yet contradict behaviour is **medium**. Scope this to docs whose subject the diff actually touches — it is not a request to audit the whole doc tree.

### Regression provenance

A correctness or behavioural defect the subagent finds needs provenance before severity — a defect the diff *introduces* is the author's to fix here; one it merely *surfaces* (a latent bug the change exposed) or *carries forward* (pre-existing, in code the diff did not touch) is real but usually outside this PR's scope. Instruct the subagent to establish this with `git blame` on the offending lines and `git log -S<symbol>`/`-G<regex>` over the touched paths, classifying each defect as **introduced by**, **made visible by**, or **carried forward by** the diff. It attaches a confidence — `clear`, `likely`, or `unknown` — and reports `unknown` when blame is ambiguous rather than inventing a cause. A defect the diff introduces is a **blocker**; a made-visible or carried-forward one is surfaced with its provenance at a severity matching its scope, not silently folded into the author's burden.

## Phase 3 — Findings contract

The subagent returns a structured report with one entry per finding:

```text
[<severity>] <path>:<line> — <one-line claim>
  Cite: <doc path, CLAUDE.md rule, or the issue's goal/acceptance criterion>
  Provenance: <introduced | made-visible | carried-forward, confidence clear|likely|unknown — correctness/regression findings only>
  Suggest: <concrete patch or "surface to author">
```

**Keep findings terse.** Instruct the subagent to write each claim and `Suggest:` as one tight sentence — the defect and the fix, no preamble, no restating the diff. A human and the `responding-to-pr-review` loop read every finding, so an over-long comment costs reader context on every PR; a finding that needs more than a sentence is two findings — split it.

Severity bands (the subagent applies these; this skill interprets them downstream):

- **blocker** — correctness bug, security issue, hard-rule violation that would land a regression, or the issue's goal the diff plainly fails to achieve. Always fix before merge.
- **high** — design smell or stale-doc finding with a named citation (e.g. "helper extracted for one callsite" → the project's code-design rule). Default: fix.
- **medium** — quality nit with a citation. Default: fix if cheap, surface if it requires judgment.
- **low** — style/taste, no doc citation. Default: surface, do not fix.

A finding without a citation — doc, rule, or the issue's goal/AC — drops one severity band. Unsourced opinions are low at best.

## Phase 4 — Interpret findings

Read the subagent's report and route:

- **All findings ≤low and CI green** — surface "no actionable findings" to the user; the PR is ready for the human's merge decision.
- **Any blocker/high, or actionable medium** — fire `responding-to-pr-review`, handing it **the structured report (Phase 3 findings) and the subagent id**. These findings are thread-less, so its loop drives each through apply / skip-with-reason / push-back exactly as it does a human reviewer's inline threads — a blocker gets implemented, not summarized. The subagent id keys the post-resolution summary so the audit trail survives the handoff.
- **Subagent malformed return** (not the structured format above) — re-dispatch once with a tightened prompt naming the format verbatim. If the second dispatch also malforms, stop and surface a handoff-required failure to the user; log the malformation via `anvil create inbox` and wait for manual review or a later retry. Do **not** fall back to main-session review — that defeats the Iron Law.

Do **not** silently drop findings the subagent surfaced. A finding you judge wrong or out-of-scope goes in an explicit **Dismissed** bucket in the report you surface — the finding plus a one-line reason — kept visible so the human can override; disagreement is recorded, not erased. Findings you act on route through the responding-to-pr-review loop. The audit trail matters more than the disagreement.

## What NOT to do

- Do not review the PR in this session. Dispatch.
- Do not skip the review because CI is green. CI is necessary, not sufficient; the merge decision waits on this review pass.
- Do not restate or hardcode doc paths in the dispatch prompt — name the entry point (`CLAUDE.md`), the subagent follows its index to this project's standards.
- Do not merge. `dispatching-issue-fleet`'s Iron Law applies — human owns the merge button.
- Do not skip findings with "nitpick" when the finding cites a documented repo rule. Same nitpick policy as `responding-to-pr-review`.

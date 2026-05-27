---
name: self-testing
description: "Use to self-test a project end-to-end, discovering its flow from CLAUDE.md/AGENTS.md and filing each bug as an issue tagged activity/self-test. Triggers: 'self-test this project', 'shake out bugs'. Not for a known issue (use completing-issue)."
---

# Self-Testing

Exercise a project end-to-end the way a real user or operator would — not to ship a feature, but to find where it breaks or grates. You are a **user first, reporter second**: drive the actual flow, and only when something concretely breaks or stumps you do you stop and file it. The project's own conventions tell you what to exercise and how — you discover that, you do not assume it.

## Iron Law

**NO ISSUE WITHOUT A REPRODUCTION.** Every filed issue embeds the exact failing command or step and the observed-vs-expected delta. Friction you cannot reduce to a re-runnable reproduction goes in the report as an observation, not the tracker. Speculative risks, "might-be-nice," and one-off paper-cuts are aggregated and reported — not filed.

## Autonomous mode (unattended runs)

When the invocation declares an unattended run — a `/loop`-driven overnight self-test, or a caller stating no human is watching — drive every phase to its end and **defer all human judgment to the morning PR review**. Never pause for a confirmation: make the bounded decision and move on.

Quality gates do **not** relax — the Iron Law, each issue's `## Verification`, and the independent PR review all still run. Autonomy auto-answers *confirmations*, never *gates*, and **ends at PR-opened — never auto-merge.** What changes:

- **Phase 4:** propagate the mode — tell `writing-issue` the run is unattended so its severity, convergence, and milestone confirms auto-resolve (see its *Autonomous mode* section). A finding with no nameable goal or no fitting milestone becomes an `inbox` item, not a forced issue.
- **Phase 5:** skip "confirm scope before dispatching" — fire a `completing-issue` worker per auto-fixable id directly; the morning PR review is the scope gate. A worker hitting a scope-change files a blocker and skips (see its *Autonomous mode* section), leaving that issue `in-progress` for the morning review.

## Phase 0 — Orient (discover, do not assume)

Read the project's entry-point docs — `CLAUDE.md`, `AGENTS.md`, `README`, the task runner (`justfile`/`Makefile`), and whatever `docs/` they index. Extract, in the project's own terms:

- **Core loop** — the primary end-to-end flow a user/operator runs (a CLI pipeline, a request path, an ingest→transform→serve chain, a methodology loop). This is your main test target.
- **How to exercise it for real** — the commands/entrypoints, and the project's bar for a *real* end-to-end check versus a fixture/mock that proves nothing. Honor that bar.
- **Sandbox** — how to run against throwaway state (a tmp catalog/db, a scratch dir, a throwaway vault) so the self-test never corrupts real data. If the docs name no isolation mechanism, that gap is itself a finding.
- **Issue workflow** — how this project files issues (which tracker or issue-authoring skill, which project slug), so findings land where the maintainer will see them.

If the docs don't answer one of these, note it — a project a new user can't orient in from its own docs has a real onboarding gap.

## Phase 1 — Isolate

Stand up the sandbox you identified in Phase 0 so the run works against throwaway state, never real data. Confirm you are actually pointed at it (the catalog path, vault, or working dir the project's own tooling reports) before exercising anything that writes or mutates.

## Phase 2 — Walk the core loop

Drive the project's primary end-to-end flow, in order, as a real user/operator — using its *real* invocation, not a fixture shortcut (Phase 0's live-smoke bar is the standard). At each step watch for friction: broken commands, confusing output, schema surprises, silently wrong results, dead ends, missing affordances. Keep a running log of each: the exact command/step, what you expected, what happened.

A green unit suite is not the target — you are testing whether the system *works*, which is exactly what unit tests miss.

## Phase 3 — Probe the surface

Map the project's command/entry surface from the tool, not from memory — gaps in that self-description are themselves findings. Enumerate the primary CLI/entrypoints from their own `--help`/usage (and any sub-command or skill list the project ships), then walk each, including the obvious wrong inputs (missing flags, bad ids, malformed state). At each step judge whether the help text and the error messages *teach* or *stump*, and log frictions the same way.

## Phase 4 — File findings (Iron Law applies)

File findings into the **real** tracker, not the sandbox. If your sandbox redirected state via an env var or flag, drop it and confirm you are pointed back at the real project *before* filing — a forgotten override would file findings into the throwaway state you are about to discard.

For each logged friction that reduces to a re-runnable reproduction, file it via the project's issue workflow (discovered in Phase 0), labelled with this skill's own provenance tag `activity/self-test` — applied through whatever tracker the project uses — so the run's findings are retrievable as one batch. On an anvil tracker the tag is facet-novel on first use: register it once with `anvil tags add activity/self-test --desc "..."` so filings validate (passing only `--allow-new-facet=activity` lets `create` through but leaves later `validate` failing). Before filing, **dedup**: scan existing issues and the current batch — fold a repeat into the existing issue, and aggregate repeated paper-cuts against one surface into a single issue, not five. Honor the project's rule for *which* tracker or project a finding belongs to (a methodology-tool gripe may belong to the tool's repo, not the product's).

**REQUIRED SUB-SKILL:** Use writing-issue for each filed finding (or the project's own named issue-authoring skill if it differs).

Then tear down the sandbox.

## Phase 5 — Resolve the clear wins

Filing is not the finish line — an unworked backlog is its own friction. From the batch, pick the **auto-fixable subset** and dispatch a worker per finding; leave the rest filed.

- **Auto-fixable** — a `bug` with a small, unambiguous fix, **in this repo**, already carrying a goal + verification. It resolves to a review-green PR with no human design call.
- **Leave filed** — design/refactor findings and aggregated paper-cuts (an auto-PR would pre-commit a direction the maintainer should decide first), and anything belonging to *another* repo (a methodology-tool gripe filed against the tool is unfixable from here).

```bash
anvil list issue --tag activity/self-test --status open --json   # triage; pick the auto-fixable bug ids
```

Fire the worker on each auto-fixable id: it claims → fixes → live-smokes → opens one review-green PR (and can run on a cheaper subagent). A worker that cannot converge leaves its issue filed — so the hard ones correctly stay as issues. The human owns the merge button. For a larger batch the fleet parallelises workers, but note it selects the project's `--ready` set rather than a curated id list, so confirm scope before dispatching.

**REQUIRED SUB-SKILL:** Use completing-issue per auto-fixable bug (main session only — a subagent cannot fan out its own workers); dispatching-issue-fleet is the batch alternative when its ready-set matches the chosen subset.

## Phase 6 — Report

Output a concise closeout in chat (not a tracked artifact):

- **Exercised** — which flows/commands you drove, against which sandbox.
- **Dispatched** — bug issues handed to the fleet: id + the PR each opened, for your review.
- **Filed, left for you** — design/cross-repo/aggregated findings filed but not auto-fixed, each with why.
- **Observed, not filed** — friction without a clean reproduction, plus paper-cuts you aggregated — each with a one-line reason (a conscious-rejection list, not silence).
- **Clean** — surfaces that worked end-to-end with no friction.

Stop. Do not re-run the walk solely to produce a tidier report.

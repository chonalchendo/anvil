---
name: self-testing-anvil
description: "Use to dogfood anvil's core loop end-to-end in a throwaway vault, filing each concrete friction as an issue tagged activity/self-test. Triggers: 'self-test anvil', 'dogfood the core loop'. Not for working a known issue (use completing-issue)."
---

# Self-Testing Anvil

Drive anvil's core loop the way a brand-new user would — empty vault, greenfield project, product design, system design, milestone, issue, completion — surfacing friction that steady-state work never re-exercises. You are a **user first, reporter second**: use the tools for real, and only when something concretely breaks or grates do you stop and file it. Findings land as issues in the *current* project's vault, tagged `activity/self-test`; the dogfood itself runs in a throwaway vault so it never pollutes real work.

## Iron Law

**NO ISSUE WITHOUT A REPRODUCTION.** Every filed issue embeds the exact failing command and the observed-vs-expected delta. Friction you cannot reduce to a re-runnable command goes in the report as an observation, not the vault. Speculative risks, "might-be-nice," and one-off paper-cuts are aggregated and reported — not filed.

## Phase 0 — Orient

Read the host project's `CLAUDE.md` / `AGENTS.md` to learn *this* anvil's conventions: where the real vault lives, the project slug, how issues are tagged, any provenance-facet rules. Do not hardcode a layout — discover it. Confirm the binary under test reflects the build you mean to exercise: `anvil --version`.

## Phase 1 — Isolate

Stand up a throwaway vault so the dogfood never touches real artifacts:

```bash
export ANVIL_VAULT="$(mktemp -d)/self-test-vault"
anvil init "$ANVIL_VAULT"
```

Everything in Phases 2–3 runs against `$ANVIL_VAULT`. Findings (Phase 4) are filed against the *real* vault — unset or override `ANVIL_VAULT` for those calls. Never create a throwaway project in the real vault.

## Phase 2 — Walk the cold-start path

Exercise the greenfield loop in order, as a real user. Where a skill exists for a step, fire the skill — not the raw CLI. At each step watch for friction: confusing output, broken hint commands, missing affordances, schema surprises, oversized output, dead ends.

1. **Product design** — `writing-product-design` for a small, plausible throwaway project.
2. **System design** — `writing-system-design`.
3. **Milestone** — `writing-milestone` with a closed acceptance criterion.
4. **Issue** — `writing-issue`: author one issue end-to-end (goal, severity, verification, milestone link).
5. **Completion** — `completing-issue` far enough to exercise the claim → verify path. A throwaway project has no repo to PR against, so stop at the verify gate.

Keep a running log of each friction point: the command run, what you expected, what happened.

## Phase 3 — Probe the CLI surface

Map the surface from the tool, not from memory — and treat gaps in that self-description as findings. Enumerate the full verb set with `anvil --help`, then walk each verb via `anvil <verb> --help` and read each skill body via `anvil show skill <name>`; that live list is your coverage map, so it stays current as the CLI grows. For the verbs a new user leans on, try the obvious wrong inputs too (missing flags, bad ids). At each step judge whether the `--help` text and the error message *teach* or *stump*, and log frictions the same way.

## Phase 4 — File findings (Iron Law applies)

Findings go to the **real** vault, not the throwaway. Drop the override and confirm *before* filing anything — a forgotten env var would file findings into the throwaway you are about to delete in this same phase, silently losing them:

```bash
unset ANVIL_VAULT
anvil where   # confirm this points at the real vault, not the self-test sandbox
```

Then, for each logged friction that reduces to a re-runnable reproduction, file it via `writing-issue`, tagged `activity/self-test` so the run's findings are retrievable as one batch:

```bash
anvil list issue --tags activity/self-test --json   # the batch this run contributes to
```

The first use of the tag trips the novelty gate; pass `--allow-new-facet=activity` once. Before filing, **dedup**: scan the batch above and existing issues — fold a repeat into the existing issue rather than file a near-duplicate, and aggregate repeated paper-cuts against one surface into a single issue, not five.

**REQUIRED SUB-SKILL:** Use writing-issue for each filed finding.

Then tear down the throwaway vault: `rm -rf "$ANVIL_VAULT"`.

## Phase 5 — Report

Output a concise closeout in chat (not a vault artifact):

- **Exercised** — which skills/verbs you drove, against which throwaway project.
- **Filed** — each issue id + one-line title + severity, all tagged `activity/self-test`.
- **Observed, not filed** — friction without a clean reproduction, plus paper-cuts you aggregated — each with a one-line reason (a conscious-rejection list, not silence).
- **Clean** — surfaces that worked end-to-end with no friction.

Stop. Do not re-run the walk solely to produce a tidier report.

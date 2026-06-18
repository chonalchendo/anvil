# Researching — Adversarial mode (default)

Adversarial is the default. The point is to surface opposing views before synthesising, so the caller doesn't get a one-sided pitch dressed as a finding.

NO SYNTHESIS WITHOUT AN OPPOSING VIEW CONSIDERED.

## Phase: Gather

Use `WebSearch` and `WebFetch`. Capture URLs alongside claims as you go — don't defer. Aim for ~5–8 sources, deliberately including ones likely to disagree (compare-vs queries, "X considered harmful", "why we moved off X", issue trackers, HN/Lobsters threads).

If every source agrees, that's a signal you haven't looked hard enough — not a finding.

## Phase: Challenge

Before synthesising, surface at least one of:

- a named production failure (post-mortem, incident write-up, public outage),
- a knowledgeable critic by name (maintainer of an alternative, cited expert, well-argued blog post),
- a non-trivial limitation tied to a specific scenario (workload shape, scale point, ops model where it breaks).

If an honest search turns up none, record "no public criticism found" as the finding. That discharges the iron law — silent omission does not.

Keep this section short: one to three bullets, each with a URL and the gist.

## Phase: Synthesise

Only after Challenge. The synthesis must explicitly reflect the opposing view, not bury it. Shape:

> X is the consensus pick for <case>. Y argues it fails when <scenario>, which applies to / does not apply to our context because <reason>.

Name sources inline (`per docs.example.com/...`). Mark gaps explicitly — "no info found on Y" beats silent omission.

## Phase: Multi-voter (optional, high-stakes claims only)

When a decision rides on one or more load-bearing claims and you want stronger than a single-agent Challenge pass, apply independent multi-voter adjudication. Engage this phase explicitly ("verify these claims with multiple skeptics") — do not run it by default, as the token cost is high.

**Mechanism:**

1. Identify the load-bearing claims from the Synthesise output (typically 2–5; skip obvious background facts).
2. For each claim, run K independent skeptic passes (K = 3 is the default). Each pass argues against the claim using only sources not already cited in favour of it. Passes must be independent — each starts from the claim text only, not from the previous skeptic's output.
3. Tally: if ≥ ⌈K×2/3⌉ passes refute the claim (assert it is false, unsupported, or overstated in the decision context), **drop the claim** from the synthesis and note it as unverified. If fewer than that threshold refute it, the claim survives.
4. Revise the synthesis to reflect dropped claims. Where a claim is dropped, record "claim dropped: <gist>, refuted by <n>/<K> independent skeptics."

**Parallel vs. sequential harnesses:**

Where the agent harness can run K passes concurrently (e.g. multi-agent fan-out), do so. Where it cannot, run the K passes sequentially in a single context — each pass must not be influenced by the previous one's conclusion; summarise the claim but do not summarise how the prior pass evaluated it. Sequential and parallel produce the same tally; sequential is slower but cross-harness portable.

**Why not always:** Multi-voter triples (or more) the Challenge cost. Reserve it for claims whose error would cause a genuinely bad decision — architecture choices, security properties, performance claims that size a migration.

## Hand-back

- **Sub-skill:** return synthesis (including the reflected critique, and any dropped claims) to the caller. Done.
- **Standalone:** return to `SKILL.md` Phase 3 — Capture.

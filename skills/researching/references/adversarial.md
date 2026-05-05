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

## Hand-back

- **Sub-skill:** return synthesis (including the reflected critique) to the caller. Done.
- **Standalone:** return to `SKILL.md` Phase 3 — Capture.

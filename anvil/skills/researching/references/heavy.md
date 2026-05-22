# Researching — Heavy mode

Heavy is for decision-grade research: a real call rides on the answer. The cost of a confident-sounding but unsourced claim is high, so every assertion in the synthesis ties back to a graded source.

EVERY CLAIM CITES ITS SOURCE.

## Phase: Source-map

Before reading anything in depth, list candidate sources and grade each:

- **primary** — project docs, original paper, maintainer post, source code, official changelog.
- **secondary** — well-known synthesis (Wikipedia, established textbook, recognised survey).
- **blogspam** — unsourced posts, content-farm SEO pages, AI-generated-looking listicles, tutorials that cite nothing.

Drop blogspam outright. Don't read it, don't quote it, don't "balance" with it. If a claim only shows up in blogspam, it isn't a fact yet — note the gap instead.

Aim for a mix: at least one primary per major claim area, secondaries to triangulate. Record the map (URL + grade) before moving on.

## Phase: Gather

For each kept source, record three things:

- **claim** — what the source asserts that's relevant to the question.
- **evidence quote** — verbatim, ≤3 lines. No paraphrase at this stage; paraphrase loses fidelity and the verbatim is what you'll cite.
- **grade** — primary / secondary, carried over from the source-map.

If a quote runs longer than 3 lines, you're probably gathering too broadly — narrow the claim. If you can't find a verbatim quote that supports the claim, the source doesn't support the claim.

Use `WebSearch` and `WebFetch`. Capture URLs alongside claims as you go.

## Phase: Synthesise

Write the answer with a per-claim citation on every assertion. Inline `[per <url>]` or footnote-style — pick one and stick to it within the document. No bare claims, no "it is generally said that" hand-waves.

Shape per claim:

> <assertion> [per docs.example.com/page#anchor]

Where primaries and secondaries disagree, surface the disagreement with both citations rather than picking a winner silently. Where only secondaries cover a point, say so — "secondary sources only, no primary located".

Mark gaps explicitly: "no primary found on Y" beats silent omission. Decision-grade callers need to see the holes.

## Hand-back

- **Sub-skill:** return the cited synthesis to the caller. Done.
- **Standalone:** return to `SKILL.md` Phase 3 — Capture. Heavy mode usually produces multiple learnings — one per coherent finding, not one mega-artifact.

# Researching — Light mode

Light is for low-stakes curiosity. No iron law. Output: short synthesis returned to caller, or 0–1 learning if standalone.

## Phase: Gather

Use `WebSearch` and `WebFetch`. Capture URLs alongside claims as you go — don't defer. Cap at ~5 sources unless something obvious is missing (a primary doc, a counter-claim worth one more fetch).

Stop when the question is answered well enough for the framed stakes. No completeness theatre.

## Phase: Synthesise

Short prose answer to the framed question. Name sources inline — `per docs.example.com/...`, `per github.com/org/repo#readme`. One paragraph is usually enough; bullets only if the answer is genuinely a list.

Mark gaps explicitly: "no info found on Y" beats silent omission. The caller needs to know what you didn't find.

## Hand-back

- **Sub-skill:** return synthesis to the caller. Done.
- **Standalone:** return to `SKILL.md` Phase 3 — Capture. Expect 0–1 learning; light-mode curiosity rarely warrants more.

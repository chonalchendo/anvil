# `## Problem` for a cold reader

The reader is a human triaging the queue weeks later, not the agent that found the gap, and they get one pass. Order the section so the first sentence alone says what is wrong, and lead plus evidence together let them judge severity:

1. **Lead sentence** — what is broken or missing and what it stops. Plain words, ≤25 words. No cause, evidence, history, or identifiers.
2. **Evidence** — what proves the gap today: a measurement, a reproduction, or the thing a user cannot do. Counts, ids, paths, and timestamps go in a short list, never inline in prose.
3. **Cause** — its own paragraph when known. Omit when the kind has none.
4. **Direction** — the fix in one short paragraph, closing with the files or modules it touches.
5. **Sequencing** — dependencies and ordering, one line at the end. Every issue it names becomes a typed edge in Phase 4b. Omit when none.

One idea per sentence, about 20 words, no chained clauses.

**Cold-reader self-test.** Cover everything after the first sentence: can a reader say what is wrong? Cover everything after the evidence: could they pick a severity? Either fails, rewrite.

# `## Problem` for a cold reader

The reader is a human triaging the queue weeks later in Obsidian's reading view, not the agent that found the gap. They scan before they read: the scan lands on bold labels, list heads and table cells, and only the lead sentence is read in full. Shape the section so the scan alone says what is wrong, what proves it, and what will be built.

Five parts, in this order. Every part after the first opens with a bold run-in label (`**Evidence.**`), never a `###`: H3s inside an issue belong to `## Verification`, and a `### Direction` heading prefix-matches the `### Direct` scan.

1. **Lead sentence** — its own paragraph, first. What is broken or missing and what it stops. Plain words, ≤25 words. No cause, evidence, history, or identifiers.
2. **Evidence** — what proves the gap today: a measurement, a reproduction, or the thing a user cannot do. A list, one fact per item. Counts, ids, paths and timestamps live in list items, never inline in prose.
3. **Cause** — its own paragraph when known. Omit when the kind has none.
4. **Direction** — one short paragraph naming the approach, then a list of everything the fix names (model, table, column set, verb, flag), one item per thing with its name at the head. Three or more things sharing the same attributes (say model, grain, kind) go in a table instead. Close with a `Files:` list.
5. **Sequencing** — dependencies and ordering, one line at the end. Every issue it names becomes a typed edge in Phase 4b. Omit when none.

Shape rules for every part:

- One idea per sentence, about 20 words, no chained clauses.
- A paragraph holds one idea and stays under about 80 words. A paragraph that would enumerate — more than two named things, or a "then / plus / also" chain — is a list.
- Identifiers go in code spans. CommonMark only: no callouts or other Obsidian-only syntax, so every agent CLI reads the body the same.

**Cold-reader self-test.** Cover everything after the first sentence: can a reader say what is wrong? Cover everything after the evidence: could they pick a severity? Either fails, rewrite.

**Glance test.** Read only the bold labels, list heads and table cells. Do they alone say what is wrong, what proves it, and what will be built? A paragraph over eight rendered lines, or a list item over two, fails it: split.

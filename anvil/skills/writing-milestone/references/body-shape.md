# Body shape (cold reader)

The readers are `running-milestone` at Phase 0, the human choosing the next work in Obsidian's reading view, and the worker one hop up the spine. All scan before they read: bold labels, list heads and table cells carry the section; prose carries only the lead sentence and rationale. Four sections, in this order:

- `## Objective`
  - Lead sentence, its own paragraph: what ships and what it changes for whom. ≤25 words, no history or mechanism.
  - **Why now** — bold run-in label, then the gap's measurement as a list, one fact per item.
  - **Design change** — bold run-in label, then one bullet per design artifact the milestone touches, component designs included; acceptance updates each one.
  - **Components changed:** — bold run-in label, then the components touched.
  - **Limit:** — bold label with colon, one line: the appetite, counted in issues (for example `**Limit:** 6 issues`). `running-milestone` reads it at Phase 0 and Exit 3.
  - **Risks:** — optional bold label, one line naming what could stall the milestone.
- `## Non-goals` — bulleted scope fence.
- `## Links` — sibling milestones and reader-facing references, each with a few words after the link saying why it is here. The governing design travels in the typed slots (Phase 4); component designs reach a worker via `writing-issue` Phase 4b, not from here.
- `## Status` — one dated block, rewritten in place. It opens with a line starting `Measured: YYYY-MM-DD` (line-start, no bold; prose may follow the date). Anvil flags the milestone `measurement_stale` once that date is over 14 days old while `in-progress`; re-measure and bump it. Then the acceptance ledger as a table, one row per `acceptance:` entry (AC, met / not met, measured value). The AC cell holds the `acceptance:` index plus a short label, never a truncated command. Acceptance predicates run from the project's main checkout. Then dated prose for what moved and what blocks. The issue map is `anvil list issue --milestone <id>`; do not copy it here.

No `## Success criteria` section. `acceptance:` is the single source; refine it with `anvil set milestone <id> acceptance --add/--remove`, never by appending an "AC refinement" section.

One idea per sentence, about 20 words, no chained clauses. A paragraph stays under 80 words; one that would enumerate is a list. CommonMark only: no callouts or other Obsidian-only syntax.

**Cold-reader test.** Cover everything after the Objective's first sentence. Can a reader say what ships?

**Glance test.** Read the Objective's lead sentence plus only the labels and the Status table. Can the reader say what ships, what the limit is, and what has landed?

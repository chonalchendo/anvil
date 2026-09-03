# Body shape (cold reader)

The reader is a human deciding what to work next in Obsidian's reading view, or an agent at issue-start reading one hop up the spine. Both scan before they read: bold labels, list heads and table cells carry the section; prose carries only the lead sentence and rationale. Four sections, in this order:

- `## Objective`
  - Lead sentence, its own paragraph: what ships and what it changes for whom. ≤25 words, no history or mechanism.
  - **Why now** — bold run-in label, then the gap's measurement as a list, one fact per item.
  - **Waves** — bold run-in label, then a numbered list:
    - A wave is one line: bold name, then one sentence of intent.
    - Under it, one nested bullet per issue: the `[[wikilink]]`, then what lands in ≤15 words.
    - Add ids as issues are written.
    - Wave order becomes typed edges on the issues (`writing-issue` Phase 4b), never prose alone.
- `## Non-goals` — bulleted scope fence.
- `## Links` — sibling milestones and reader-facing references, each with a few words after the link saying why it is here. The governing design travels in the typed slots (Phase 4); contracts reach a worker via `writing-issue` Phase 4b, not from here.
- `## Status` — one dated block, rewritten in place. For `kind: scoped` it opens with the acceptance ledger as a table, one row per `acceptance:` entry (AC, met / not met, measured value), then dated prose for what moved and what blocks. The issue map is `anvil list issue --milestone <id>`; do not copy it here.

No `## Success criteria` section. `acceptance:` is the single source; refine it with `anvil set milestone <id> acceptance --add/--remove`, never by appending an "AC refinement" section.

One idea per sentence, about 20 words, no chained clauses. A paragraph stays under 80 words; one that would enumerate is a list. CommonMark only: no callouts or other Obsidian-only syntax.

**Cold-reader test.** Cover everything after the Objective's first sentence. Can a reader say what ships?

**Glance test.** Read the Objective's lead sentence plus only the labels, wave names, nested issue links and the Status table. Can the reader say what ships, what has landed, and what to work next? A wave line holding more than one issue link fails it.

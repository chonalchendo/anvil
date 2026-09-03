# Body shape (cold reader)

The reader is a human deciding what to work next, or an agent at issue-start reading one hop up the spine. Four sections, in this order:

- `## Objective`
  - Lead sentence: what ships and what it changes for whom. ≤25 words, no history or mechanism.
  - Why now: the gap, with its measurement in a short list.
  - **Waves**: an ordered list, one line each, naming what lands. Add issue ids as they are written. Wave order becomes typed edges on the issues (`writing-issue` Phase 4b), never prose alone.
- `## Non-goals` — bulleted scope fence.
- `## Links` — sibling milestones and reader-facing references. The governing design travels in the typed slots (Phase 4); contracts reach a worker via `writing-issue` Phase 4b, not from here.
- `## Status` — one dated block, rewritten in place: each AC with met / not met and the measured value. The issue map is `anvil list issue --milestone <id>`; do not copy it here.

No `## Success criteria` section. `acceptance:` is the single source; refine it with `anvil set milestone <id> acceptance --add/--remove`, never by appending an "AC refinement" section.

One idea per sentence, about 20 words, no chained clauses. Test before create: cover everything after the Objective's first sentence. Can a reader say what ships?

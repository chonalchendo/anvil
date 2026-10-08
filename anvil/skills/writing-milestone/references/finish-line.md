# Finish-line gate

Each acceptance bullet is a runnable predicate — a command that exits 0/1, or an observation a reader can re-check without ambiguity — never prose that merely looks testable. Each bullet names the command or query that measures it and what green reads as (`exits 0`, `= 0 rows`); "SQL predicate: zero X" with no SQL is prose.

A milestone needs a **witnessable finish line** — a point a future agent can run and see is reached. Two silent ways it lacks one; refuse both here rather than carry them into Phase 3:

- **State-phrased goal.** The `goal` names an ongoing condition ("docs *stay* accurate", "the CLI *remains* fast") instead of an event. A persisting state never closes, so the milestone never ends. Refuse it: rewrite the goal as a terminal predicate ("docs match the shipped flags as of <sha>"). If the work is open-ended collection, it is not a milestone: route it to inbox or aggregate it.
- **Silent empty acceptance.** `acceptance` is left `[]`. A milestone with no closeable AC never ends. Refuse it — do not proceed with empty acceptance.

The author resolves a refusal by writing the finish line: at least one runnable-predicate acceptance criterion and an event-phrased goal.

# Finish-line gate (scoped only)

A scoped milestone needs a **witnessable finish line** — a point a future agent can run and see is reached. Two silent ways it lacks one; refuse both here rather than carry them into Phase 3:

- **State-phrased goal.** The `goal` names an ongoing condition ("docs *stay* accurate", "the CLI *remains* fast") instead of an event. A persisting state never closes, so the milestone never ends. Refuse it: rewrite the goal as a terminal predicate ("docs match the shipped flags as of <sha>"), or — if the work genuinely is open-ended collection — flip to a bucket.
- **Silent empty acceptance.** `acceptance` is left `[]` on a `kind: scoped` milestone. That is the bucket anti-pattern smuggled into the wrong kind: a scoped milestone with no closeable AC. Refuse it — do not proceed with empty acceptance on a scoped milestone.

The author resolves a refusal one of two ways, deliberately:

1. **Write the finish line** — supply at least one runnable-predicate acceptance criterion (and an event-phrased goal). This is the default.
2. **Explicit bucket affirmation** — the work really is a rolling-findings tracker with no end. Affirm it out loud, then flip `kind` to `bucket` in Phase 3; `acceptance: []` is then legal *because the choice was made, not defaulted into*.

# Refactor issue — body guidance

**Goal:** a predicate naming the new internal shape with behaviour held constant (e.g. "claim logic is centralised in one transition path, externally unchanged").

No `reproduction_anchor` — nothing is broken.

Lead `## Problem` with:

- **The forcing function** — the concrete upcoming change this refactor unblocks. Without one it is in-passing cleanup that rots into a "someday clean this up" never claimed; file it only when a named milestone AC needs the new shape now. The milestone link is necessary but not sufficient.
- **The held invariant** — the external behaviour or contract that must not change. A refactor that changes behaviour is a different issue.
- **The target shape** — a one-line before→after sketch (signatures, call-paths — not full bodies) concrete enough that "better" isn't left to the implementer's guess.
- **The regression risk** the change carries.

Point `## Non-goals` at the scope fence: name what you will *not* touch or improve while in there. Scope creep is the dominant refactor failure mode.

`## Verification` asserts behaviour is *unchanged*: pin current behaviour with a characterization test **before** the change (if coverage is thin, writing it is part of this issue), then show the same observed output before and after for one real path. Verifying new behaviour is a category error here.

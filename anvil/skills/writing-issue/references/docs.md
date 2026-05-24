# Docs issue — body guidance

A docs issue closes a documentation gap. No `reproduction_anchor`.

## Body shape

- **## Problem** — the named audience and the specific gap: who needs the doc, what they cannot currently find or understand, and where it should live.
- **## Non-goals** — adjacent docs you are not writing here.
- **## Verification** — Direct asserts the doc exists and carries the load-bearing content (e.g. `grep -q` the rendered/installed doc); Indirect confirms it is reachable through the published surface (see `docs/issue-spec.md`).

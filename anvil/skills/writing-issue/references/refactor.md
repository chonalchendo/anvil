# Refactor issue — body guidance

A refactor changes internal shape while holding external behaviour constant. No `reproduction_anchor` — nothing is broken.

## Body shape

- **## Problem** — the invariant that must hold across the change (the external behaviour or contract), the shape change you intend, and the regression risk it carries.
- **## Non-goals** — behaviour changes that are explicitly out of scope (a refactor that changes behaviour is a different issue).
- **## Verification** — Direct asserts the held invariant via the existing test suite (it must stay green); Indirect confirms the refactored path behaves identically on the built artifact (see `docs/issue-spec.md`).

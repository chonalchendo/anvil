# Autonomous mode (unattended runs)

When the caller declares an unattended run (e.g. an overnight self-test loop), resolve every human-confirm in this skill from the rubric instead of asking — the morning PR review is the gate. The Iron Law still holds: no issue lands without a milestone link.

- **Severity (Phase 4):** pick from the rubric; do not confirm.
- **Convergence (Phase 1):** skip — an unattended self-test finding already carries a problem, a goal, and a reproduction, so there is nothing fuzzy to converge. If a finding is genuinely fuzzy (no nameable goal), do not converge solo: capture it as an `inbox` item and stop.
- **Milestone-fit (Phase 2):** auto file-or-skip. A fitting milestone → file. No fit → do not prompt and do not invent one: capture as `inbox` and stop.

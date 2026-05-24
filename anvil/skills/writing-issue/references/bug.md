# Bug issue — body guidance

**Goal:** a predicate naming the fixed state — the broken behaviour no longer occurs (e.g. "inbox no longer drops items on concurrent writes"). Not "fix the inbox bug" (a task).

Lead `## Problem` with observed-vs-expected: what the path does now versus what it should do, plus the repro environment.

## Reproduction anchor

Author `reproduction_anchor` for a bug — the claim-time gate that proves the bug still exists when a future agent picks it up.

Shape: `command` (shell-runnable invocation that reproduces the gap), `expected` (literal output or `sha:<hex>` digest). When an agent later runs `anvil transition issue <id> in-progress`, anvil re-runs the command and refuses the claim if the output no longer matches. Two escape hatches if the gate misfires:

- `--force` — bypass the check and claim anyway (use when the anchor itself is broken but the issue is real).
- `--no-longer-reproduces` — confirm the mismatch and close the issue directly as `resolved` with the diff captured in the audit trail.

The anchor stays optional — a bug without one transitions normally (grandfather rule) — but author it whenever a command can capture the failure, so the issue can't silently rot.

---
name: writing-milestone
description: "Use when scoping a shippable bundle of work into a milestone (product/system design must already exist). Triggers: 'scope a milestone', 'what's the next milestone', 'M1', 'M2', 'define M3'."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-milestone
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-04-30
  updated: 2026-09-03
  tags: [type/skill, activity/milestone]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Milestone

Workflow for creating a milestone artifact via the `anvil` CLI. Milestones sit one level below the design docs in Anvil's hierarchy: product-design → **milestones** → issues.

## When this skill runs

- A product-design or system-design exists for the project.
- The user wants to carve the next shippable increment (M1, M2, etc.).
- Before any issues are written for that increment.

## When not to use

- No design doc exists yet → `writing-product-design` or `writing-system-design` first.
- Work item level (a task, bug, feature) → `writing-issue`.
- Editing existing milestone frontmatter only (date bump, status flip) → a direct `anvil set` call, not this workflow.

## Phase 1 — Read the design doc

```bash
anvil project current
anvil list product-design --project <project>
anvil list system-design --project <project>
```

Read the returned artifact(s) directly (`anvil show product-design <id> --body` / `anvil show system-design <id> --body`). If both lookups return empty, say so and stop: `writing-product-design` or `writing-system-design` runs first.

**Gate:** user confirms which design doc drives scope.

## Phase 2 — Shape the milestone body

Draft before calling the CLI:
- **title** — verb-noun, one line.
- **goal** — one sentence, ≤120 chars, terminal predicate; required by schema.
- **kind** — `scoped` default, or `bucket` for rolling-findings trackers only.
- **acceptance** — runnable predicates (substance: `references/finish-line.md`); required for `kind: scoped`.

**REQUIRED REFERENCE:** Use skills/writing-milestone/references/finish-line.md — refuse a state-phrased goal or silent empty acceptance before proceeding.
**REQUIRED REFERENCE:** Use skills/writing-milestone/references/body-shape.md — the four-section body a cold reader scans: labelled parts including the **Limit:** line, the Status acceptance table.

Read the routed inbox before the gate. List the raw inbox items. Read each one, and keep those whose `## Route` names this milestone:

```bash
anvil list inbox --status raw --limit 1000 --json --fields id,title
```

The human promotes or drops each routed item. The `## Links` section of the milestone names each item it absorbed. `anvil transition milestone <id> in-progress` refuses with `inbox_unread` while a raw inbox item links the milestone. After Phase 3, run `anvil link --to milestone.<id>` to list the exact set the gate refuses on.

**Gate:** user confirms title, goal, kind, and acceptance — and, for scoped, that the goal is event-phrased and acceptance carries a runnable predicate; for bucket, that the open-ended kind was explicitly affirmed.

## Phase 3 — Create

```bash
anvil create milestone --title "<title>" --description "<one-line preview>" --goal "<terminal predicate>" --acceptance "<criterion>" --json
```

`--acceptance` repeats, one per Phase 2 criterion. A bucket passes `--kind bucket` and no `--acceptance`. Capture `id` and `path` from the JSON output. From here on, `<id>` is the JSON `id` without its `milestone.` prefix, that is `<project>.<slug>`.

If the JSON `warnings[]` carries a `kind: validation` entry, the milestone was written but its body needs revising — fix it per the entry's `code`.

Then direct-edit the body sections (shaped in Phase 2) into the file at `path`.

## Phase 4 — Link to design docs

```bash
anvil set milestone <id> product_design "[[product-design.<project>]]"
anvil set milestone <id> system_design "[[system-design.<project>]]"
```

`system_design` is the governing spine edge — issues scoped under it inherit that design as box grounding. Make an absent link an **explicit decision**, not a silent omission. Either attach the governing design, or affirm to the user that none governs this slice, before leaving the slot empty.

When the Design change names a text change to a design, link that design through `related`. `anvil doctor` flags a design that a done milestone links through `related` and whose `updated` is older than the milestone's `done`; the slots never flag.

```bash
anvil link milestone <id> system-design <project>
anvil link milestone <id> product-design <project>
```

When the milestone came from a product-design candidate, rewrite that candidate's top-level bullet under `## Milestones`. Use the shaped form `- [[milestone.<id>]] <title>`. Keep the nested "Why now" and "Components" lines. Then add the reverse edge:

```bash
anvil set product-design <project> related --add "[[milestone.<id>]]"
```

The human removes the bullet at acceptance; `anvil doctor` reports `candidate-milestone-done` until then.

## Phase 4b — Component-design coverage

**REQUIRED REFERENCE:** Use skills/writing-milestone/references/component-design-coverage.md

## Phase 5 — Validate

```bash
anvil show milestone <id> --validate
```

Fix any schema errors reported. Re-run until clean. Validate now also enforces body shape: the four required headings in order, no `## Success criteria` section, and (for `kind: scoped`) non-empty `acceptance`.

## Approval

Approval is `anvil transition milestone <id> in-progress`. For a scoped milestone, the body must carry the **Design change** and **Components changed** parts. The verb refuses with a `milestone_gate_` code when a part is absent. On success it stamps `approved:` in the frontmatter. To amend an approved milestone, run `anvil transition milestone <id> planned`, edit it, and approve again.

## Hand-off

**REQUIRED SUB-SKILL:** Use `writing-issue` for the first issue under this milestone.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.

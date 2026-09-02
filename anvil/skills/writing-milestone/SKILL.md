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

Workflow for creating a milestone artifact via the `anvil` CLI. Milestones sit one level below the design docs in Anvil's hierarchy: product-design → **milestones** → plans → issues.

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
**REQUIRED REFERENCE:** Use skills/writing-milestone/references/body-shape.md — the four-section body a cold reader needs.

**Gate:** user confirms title, goal, kind, and acceptance — and, for scoped, that the goal is event-phrased and acceptance carries a runnable predicate; for bucket, that the open-ended kind was explicitly affirmed.

## Phase 3 — Create

```bash
anvil create milestone --title "<title>" --description "<one-line preview>" --goal "<terminal predicate>" --json
```

Capture `id` and `path` from the JSON output. It ships `kind: scoped` by default; flip if bucket:

```bash
anvil set milestone <id> kind bucket
```

Then direct-edit the body sections (shaped in Phase 2) into the file at `path`.

## Phase 4 — Link to design docs

```bash
anvil set milestone <id> product_design "[[product-design.<project>]]"
anvil set milestone <id> system_design "[[system-design.<project>]]"
```

`system_design` is the governing spine edge — issues scoped under it inherit that design as box grounding. Make an absent link an **explicit decision**, not a silent omission. Either attach the governing design, or affirm to the user that none governs this slice, before leaving the slot empty.

## Phase 4b — Contract coverage

**REQUIRED REFERENCE:** Use skills/writing-milestone/references/contract-coverage.md

## Phase 5 — Validate

```bash
anvil show milestone <id> --validate
```

Fix any schema errors reported. Re-run until clean. Validate now also enforces body shape: the four required headings in order, no `## Success criteria` section, and (for `kind: scoped`) non-empty `acceptance`.

## Hand-off

**REQUIRED SUB-SKILL:** Use `writing-issue` for the first issue under this milestone.

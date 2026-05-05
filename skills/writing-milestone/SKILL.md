---
name: writing-milestone
description: "Use when a product- or system-design exists and a discrete shippable bundle of work needs scoping into a milestone. Triggers: \"scope a milestone\", \"what's the next milestone\", \"M1\", \"M2\", \"define M3\". Not for system design (anvil:writing-system-design) or individual issues (anvil:writing-issue)."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-milestone
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-04-30
  updated: 2026-04-30
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
- User wants to carve the next shippable increment (M1, M2, etc.).
- Before any issues are written for that increment.

## When not to use

- No design doc exists yet → use `anvil:writing-product-design` or `anvil:writing-system-design` first.
- Work item level (a task, bug, feature) → `anvil:writing-issue`.
- Editing existing milestone frontmatter only (date bump, status flip) — that's a direct `anvil set` call, not this workflow.

---

## Phase 1 — Read the design doc

Find the active project, then read the design doc.

```bash
anvil project current
```

The design doc lives at `05-projects/<project>/product-design.md` or `05-projects/<project>/system-design.md` inside the vault. Read the file at that path directly — designs are not yet typed artifacts in the CLI.

Confirm with the user which design doc to derive from (product or system, or both).

**Gate:** user confirms which design doc drives scope.

---

## Phase 2 — Shape the milestone body

Draft the following before calling the CLI:

- **title** — one line; verb-noun ("Ship X", "Validate Y", "Deliver Z").
- **acceptance** — testable conditions for "done"; each must be checkable without ambiguity.

**Gate:** user confirms title and acceptance criteria.

---

## Phase 3 — Create

```bash
anvil create milestone \
  --title "<title>" \
  --json
```

Capture `id` and `path` from the JSON output. Then direct-edit the body sections (objectives, success criteria, non-goals) into the file the CLI created at `path`.

---

## Phase 4 — Link to design docs

```bash
anvil set milestone <id> product_design "[[product-design.<project>]]"
anvil set milestone <id> system_design "[[system-design.<project>]]"
```

Both calls land in dedicated typed slots. If a system-design doesn't yet exist, omit the second call.

---

## Phase 5 — Validate

```bash
anvil validate "<path-from-phase-3>"
```

> **CLI gap:** `anvil show milestone <id> --validate` parity (plan-only today). See spec gap #1. Fallback: `anvil validate <path>`.

Fix any schema errors reported. Re-run until clean.

---

## Hand-off

**REQUIRED SUB-SKILL:** Use `anvil:writing-issue`.

Next: `writing-issue` for the first issue under this milestone.

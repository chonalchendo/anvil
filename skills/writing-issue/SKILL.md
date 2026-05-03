# prettier-ignore
---
name: writing-issue
description: "Use when a problem worth tracking surfaces — from an inbox thought, a raw idea, or a fully-formed request. Triggers: \"open an issue for X\", \"track this as an issue\", \"issue under M1\", \"should we build X\", \"I've been thinking about Y\", \"is this worth doing\", \"promote inbox item to issue\". Not for raw capture (anvil:capturing-inbox), milestone creation (anvil:writing-milestone), or solution design (anvil:writing-plan)."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-issue
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-04-30
  updated: 2026-04-30
  tags: [type/skill, activity/issue]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Issue

Workflow for creating an issue artifact via the `anvil` CLI. Issues sit one level below milestones: product-design → milestones → **issues** → plans.

## When this skill runs

- A milestone exists for the target project.
- A problem worth tracking has surfaced (inbox item, brainstorm output, direct request).
- Before any solution design or plan for that problem.

## When not to use

- No milestone exists → use `anvil:writing-milestone` first.
- You need to design the solution → `anvil:writing-plan` after the issue is approved.
- Editing existing issue frontmatter only (status flip, tag) — a direct `anvil set` call, not this workflow.

---

## Phase 1 — Context

Read the milestone to understand scope and success criteria.

```bash
anvil show milestone <m-id>
```

Then read any design docs referenced in the milestone (product-design, system-design). These bound the issue's problem statement.

---

## Phase 2 — Shape the issue body

Draft the following before calling the CLI. **No solution design here** — that is `anvil:writing-plan`'s job.

- **Problem statement** — what is broken or missing, and why it matters.
- **Acceptance criteria** — testable conditions for "done"; each must be checkable without ambiguity.
- **Severity** — `low` / `medium` / `high` / `critical`. Drives triage queries.
- **Links** — to the milestone, design docs, related issues.

**Gate:** confirm problem statement, acceptance criteria, and severity with the user before creating.

---

## Phase 3 — Create

**A. From inbox (promotion):**

```bash
anvil set inbox <inbox-id> suggested_type issue
anvil inbox promote <inbox-id>
# output: "issue <new-issue-id>"
```

> **CLI gap:** `anvil inbox promote <id> --as <type>` (single-step). Today: two-step set + promote. See spec gap #3.

**B. Greenfield:**

```bash
anvil create issue --title "<title>" --json
```

Capture `id` and `path` from the JSON output.

---

## Phase 4 — Link upward and set severity

```bash
anvil set issue <issue-id> milestone "[[milestone.<project>.<slug>]]"
anvil set issue <issue-id> severity <low|medium|high|critical>
```

Both writes land in typed slots on the issue; structural edges always go through `set`, not `link`. (Use `link` only for associative pointers — those land in `related[]`.)

---

## Phase 5 — Author body

Direct-edit the body section of the file the CLI created (at `path` from Phase 3). Write the problem statement, acceptance criteria, non-goals, and links drafted in Phase 2. Edit the body only — do not hand-author frontmatter.

---

## Phase 6 — Validate

```bash
anvil validate
```

> **CLI gap:** `anvil show issue <id> --validate` parity (plan-only today). See spec gap #1. `anvil validate` validates the whole vault; no per-file path argument.

Fix any schema errors reported. Re-run until clean.

---

## Hand-off

Next: `writing-plan` once the issue is approved. **REQUIRED SUB-SKILL:** Use `anvil:writing-plan`.

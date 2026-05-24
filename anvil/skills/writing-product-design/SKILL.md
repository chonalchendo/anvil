---
name: writing-product-design
description: "Use when starting a NEW project — vision, users, success, scope, milestones. Greenfield only. Not for system design (writing-system-design) or individual issues (writing-issue)."
license: MIT
allowed-tools: [Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-product-design
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-04-27
  updated: 2026-05-24
  tags: [type/skill, activity/product-design]
  diataxis: how-to
  authored_via: extracting-skill-from-session
  confidence: low
  status: in-use
---

# Writing Product Design

A workflow for authoring a project's product-design artifact — the top of Anvil's design-driven hierarchy. Greenfield only.

**Frontmatter is the universal spine** (`type, title, description, created, updated, status, project, tags, aliases, related`). Every output of this skill is body prose under named sections; the schema rejects anything else (`additionalProperties: false`). Schema: `schemas/product-design.schema.json`.

## When to use

- Starting a new project; need the vision artifact before milestones, plans, or code.
- User signals defining the product (not how to build it).

## When not to use

- Architecture / implementation → `writing-system-design`.
- One milestone → `defining-milestone`.
- A discrete work item → `creating-issue`.
- Light revisions to an existing PD → direct edit, not a re-author.
- Brownfield carving — different activity; this skill does not handle it.

## Output path

Canonical destination: `~/anvil-vault/05-projects/<project>/product-design.md`. Vault-only — never committed to the project's source repo. Surface this at Phase 1 so the user can flag any can't-commit-anywhere constraint up front.

## The phases

Eight phases, each with an explicit user gate. Don't skip them — gates are the load-bearing part. Each is an iteration loop; expect 1–2 reframes per phase. The quick-reference table below is the index; load the procedure when you start the walk.

**REQUIRED REFERENCE:** Use skills/writing-product-design/references/phases.md

The reference owns the per-phase procedure (drafting instructions, prompts, gates) for all eight phases through serialize-and-save.

## Quick reference

| Phase | What | Output | Gate |
|---|---|---|---|
| 1 Frame | Project scope, slug, path | — | Trivial |
| 2 Problem & users | Why it matters / Who it's for | Body | User confirms |
| 3 What we're building | One-line shape + convictions | Body | **Load-bearing** |
| 3.5 Approach | Fat-marker sketch (3–7) | Body | Altitude check |
| 4 Goals / success / constraints / out-of-scope | Four sections | Body | **Load-bearing** |
| 4.5 Risks & rabbit holes | 3–7 bullets | Body | User confirms |
| 5 Milestones | Wikilinks + summaries | Body + `related` | User confirms |
| 6 Serialize & save | Universal frontmatter + validate | Frontmatter | Cold read |

## Common mistakes

- **Stuffing prose into frontmatter.** Schema is `additionalProperties: false`; only universals + `related` are accepted. Goals, metrics, constraints, risks, milestones, target users — all body sections.
- **Drafting from a source doc.** Greenfield: there is no source. If you find yourself reading "lines X–Y of file Y", stop — that's brownfield carving.
- **Conflating *what* with *how*.** Implementation strategy, packaging, subprocess choices belong in `system-design.md`.
- **Generic success metrics.** "Users are happy" isn't a metric. Blend quantitative and qualitative; tie qualitative to a measurement plan.
- **Skipping the past-pain prompt in Phase 4.** Old-tool failure modes are the most concrete metrics.
- **Voice drift.** AI-generic prose fails the cold read. Match project voice; audit for hedging and corporate-speak.
- **Treating gates as one-shot approvals.** Each is an iteration loop. Reframes after a draft = gate working, not failing.

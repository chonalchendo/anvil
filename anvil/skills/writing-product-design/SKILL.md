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
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Product Design

A workflow for authoring a project's product-design artifact — the top of Anvil's design-driven hierarchy. Greenfield only.

**Frontmatter is the universal spine** (`type, title, description, created, updated, status, project, tags, aliases, related, external_links`). Every output of this skill is body prose under named sections; the schema rejects anything else (`additionalProperties: false`).

## When to use

- Starting a new project; need the vision artifact before milestones or code.
- User signals defining the product (not how to build it).

## When not to use

- Architecture / implementation → `writing-system-design`.
- One milestone → `defining-milestone`.
- A discrete work item → `creating-issue`.
- Light revisions to an existing PD → direct edit, not a re-author.
- Brownfield carving — different activity; this skill does not handle it.

## Saving

Save with `anvil create product-design --project <slug> --title … --body-file <file>`; read it back with `anvil show product-design <slug> --body`. The design lives in the vault, never in the project's source repo. Surface this at Phase 1 so the user can flag any can't-commit-anywhere constraint up front.

## The phases

Six phases plus two half-phases, each with an explicit user gate — gates are the load-bearing part, not optional checkpoints. Each is an iteration loop; expect 1–2 reframes per phase. Phases 3 and 4 are load-bearing: Phase 5 derives milestones from them. The reader is the human. Workers do not load this doc.

The per-phase procedure — drafting instructions, voice checks, gate criteria — lives in the reference below. The quick-reference table is the phase index; load the reference before drafting Phase 1.

**REQUIRED REFERENCE:** Use `convention.design` §Design from first principles (`anvil show convention convention.design --body`) before shaping. Refuse a draft whose components precede its needs.

**REQUIRED REFERENCE:** Use skills/writing-product-design/references/phases.md

## Quick reference

| Phase | What | Output | Gate |
|---|---|---|---|
| 1 Frame | Project scope, slug, save route | — | Trivial |
| 2 Problem & users | Why it matters / Who it's for | Body | User confirms |
| 3 What we're building | One-line shape + convictions | Body | **Load-bearing** |
| 3.5 Approach | Fat-marker sketch (3–7) | Body | Altitude check |
| 4 Goals and measures / constraints / out-of-scope | Three sections | Body | **Load-bearing** |
| 4.5 Risks & rabbit holes | 3–7 bullets | Body | User confirms |
| 5 Milestones | Open candidates: why now + components | Body + `related` | User confirms |
| 6 Serialize & save | `## TL;DR` first, frontmatter, validate | Body + frontmatter | Cold read |

## Common mistakes

- **Stuffing prose into frontmatter.** Schema is `additionalProperties: false`; only universals + `related` are accepted. Goals, measures, constraints, risks, milestones, target users — all body sections.
- **Drafting from a source doc.** Greenfield: there is no source. If you find yourself reading "lines X–Y of file Y", stop — that's brownfield carving.
- **Conflating *what* with *how*.** Implementation strategy, packaging, subprocess choices belong in `system-design.md`.
- **Goals without measures.** "Users are happy" is not a measure. Blend quantitative and qualitative; tie each qualitative measure to how you check it.
- **Skipping the past-pain prompt in Phase 4.** Old-tool failure modes are the most concrete measures.
- **Writing for workers.** No worker instructions in the doc: no pointers to repo docs or convention files, no "agents must" rules. Those belong in conventions.
- **Bare milestone one-liners.** Each open candidate needs a `Why now:` line and a `Components:` line.
- **Voice drift.** AI-generic prose fails the cold read. Match project voice; audit for hedging and corporate-speak.
- **Treating gates as one-shot approvals.** Each is an iteration loop. Reframes after a draft = gate working, not failing.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.

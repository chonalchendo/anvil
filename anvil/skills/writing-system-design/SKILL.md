---
name: writing-system-design
description: "Use when authoring system design — architecture, components, data flow, invariants. Requires existing product-design. Not for product vision (writing-product-design) or issues (writing-issue)."
license: MIT
allowed-tools: [Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-system-design
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-04-29
  updated: 2026-05-24
  tags: [type/skill, activity/system-design]
  diataxis: how-to
  authored_via: hand-authored
  confidence: low
  status: in-use
---

# Writing System Design

A workflow for authoring a project's system-design artifact — the architectural counterpart to `product-design`. The product design says *what* we're building and *why*; the system design says *what shape* it has, *what's load-bearing*, and *what must always be true*. Both are vault-only.

## When to use

- A `product-design` exists and the project needs an architectural shape before milestones, plans, or code.
- User says "let's system-design X", "what's the architecture", or anything that signals shape (not vision, not implementation tasks).
- Bootstrapping the second artifact in the vault hierarchy.

## When not to use

- No product-design yet → `writing-product-design` first.
- Vision, users, scope → `writing-product-design`.
- One milestone in detail → `defining-milestone`.
- Implementation tasks → `creating-issue` or `planning`.
- Documenting a *single* architectural choice (e.g., "JWT vs sessions") → that's an ADR, use `decision-making`.

## Output path

Canonical destination: `~/anvil-vault/05-projects/{project}/system-design.md`. Vault-only — never committed to the project's source repo. The `system-design` frontmatter schema is spelled out inline in the Phase 11 hand-check (`references/phases.md`); run `anvil validate <path>` to confirm conformance against your project's schema.

Surface this path at Phase 1 so the user can flag any constraint up front.

## The phases

Eleven phases, each with an explicit user gate — don't skip the gates. Phases 1, 4, and 7 are load-bearing: Phase 1 enforces the product-design dependency, Phase 4 derives components from product-design milestones, Phase 7 produces the invariants that downstream planning and review check against.

The per-phase procedure — drafting instructions, mermaid templates, gate criteria, voice checks — lives in the reference below. The quick-reference table is the phase index; load the reference before drafting Phase 1.

**REQUIRED REFERENCE:** Use skills/writing-system-design/references/phases.md

## Prior learnings (after Phase 1, before Phase 4)

Once Phase 1 fixes the slug and confirms the product-design dependency, dispatch `anvil-learnings-researcher` via the Agent tool's `subagent_type` to surface what the vault already knows about this architecture before you derive components. Build the `<work-context>`:

```text
<work-context>
work: <the architectural shape in one sentence>
domain: <domain/ tag(s) the design touches>
activity: activity/system-design
artifacts: [[product-design.<project>]]
</work-context>
Return the findings that genuinely bear on this work, highest-precision first.
```

Fold non-stale, high-confidence findings into components (Phase 4), invariants (Phase 7), and risks (Phase 10) as you draft, and record the surfaced set in the design's rationale (Phase 9) so the reasoning is auditable. `Findings: none` → note it and move on. A `stale?: yes` finding is a signal to weigh against present evidence, not a directive.

## Quick reference

| Phase | What | Gate |
|---|---|---|
| 1 Frame | Slug, product-design dependency, path | **Load-bearing** |
| 2 Architectural overview | One-sentence shape + body | User confirms |
| 3 Tech stack | `tech_stack` frontmatter | User confirms |
| 4 Components & responsibilities | 3–8 components, milestone mapping | **Load-bearing** |
| 5 Data flow | Mermaid sequence diagram + body | User confirms |
| 6 Boundaries | Mermaid context diagram + body | User confirms |
| 7 Key invariants | 3–7 declarative absolutes | **Load-bearing** |
| 8 Authorized decisions | ADR wikilinks (or TODOs) | User confirms |
| 9 Why this shape | Rationale (≤80 lines) | User reads cold |
| 10 Risks | 3–7 bullets | User confirms |
| 11 Serialize & save | Frontmatter, hand-check, write | User reads cold |

## Common mistakes

- **Skipping Phase 1's product-design check.** Without the product design, Phase 4 has no anchor and components drift toward implementation taste rather than product fit. Stop and hand off.
- **Soft invariants in Phase 7.** "We try to..." is not an invariant. If the user shrugs at a candidate, strip it.
- **Tech stack without ADRs.** Phase 3 should hand off to `decision-making` for any non-trivial choice. ADR-less tech stacks rot first.
- **Mermaid as decoration.** Phases 5 and 6 require diagrams as core content. A system design without a context diagram is incomplete.
- **AI-generic Why-this-shape prose.** Cite the user's own words; reference the product-design and ADRs; don't generate filler.
- **Conflating system design with planning.** Components are responsibilities, not work items. If a section reads like a task list, it belongs in `planning`.

---
name: writing-system-design
description: "Use when authoring system design — architecture, components, runtime flow, invariants. Requires existing product-design. Not for product vision (writing-product-design) or issues (writing-issue)."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
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

A workflow for authoring a project's system-design artifact — the architectural counterpart to `product-design`. The product design says *what* we're building and *why*; the system design says *what shape* it has, *what's load-bearing*, and *what must always be true*. It records the **target state only**: no "today" statements, no tech-stack inventory, no decision deliberation. Current state lives in code and in component designs' `## Interfaces` rows. Product design and system design are vault-only.

The system design is explanation plus reference (Diátaxis): Solution strategy explains; Constraints, Components and System invariants are reference.

## When to use

- A `product-design` exists and the project needs an architectural shape before milestones or code.
- User says "let's system-design X", "what's the architecture", or anything that signals shape (not vision, not implementation tasks).
- Bootstrapping the second artifact in the vault hierarchy.

## When not to use

- No product-design yet → `writing-product-design` first.
- Vision, users, scope → `writing-product-design`.
- One milestone in detail → `writing-milestone`.
- Implementation tasks → `writing-issue`.
- Documenting a *single* architectural choice (e.g., "JWT vs sessions") → that's a decision, record it with `anvil create decision --title "<the choice>" --topic <topic> --description "<one line>" --tags domain/<d>,activity/system-design --body-file <f> --json` (`<f>` holds `## Context`, `## Decision`, `## Rationale`, `## Consequences`, `## Links`; add `--allow-new-facet <facet>` for a new tag value).

## Output path

Read the product design with `anvil show product-design <project> --body`. Save with `anvil create system-design --project <project> --title "<title>" --description "<one line>" --body-file <file>`. Vault-only — never committed to the project's source repo. `anvil create` validates the frontmatter and body on write.

Surface the save command at Phase 1 so the user can flag any constraint up front.

## The phases

Eleven phases, each with an explicit user gate — don't skip the gates. Phases 1, 4, and 7 are load-bearing: Phase 1 enforces the product-design dependency, Phase 4 derives components from product-design goals (the candidate-to-component map lives in the product design's Milestones list, not here), Phase 7 produces the invariants that downstream planning and review check against.

The per-phase procedure — drafting instructions, mermaid templates, gate criteria, voice checks — lives in the reference below. The quick-reference table is the phase index; load the reference before drafting Phase 1.

**Frame the fork, then recommend.** At a *genuine* architectural fork — a choice that shapes structure the human will later have to steer — make it legible in a few lines *before* recommending: name the options plainly, state the tension, surface the rejected alternative *and why it fails*, and give the one fact that discriminates. Then recommend a single direction — don't hand back a menu. A default, not a template: stay silent on trivial choices, never manufacture tension to fill slots, and keep it brief — legible means clearer, not longer.

**REQUIRED REFERENCE:** Use `convention.design` §Design from first principles (`anvil show convention design --body`) before shaping. Refuse a draft whose components precede its needs.

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

Fold non-stale, high-confidence findings into components (Phase 4), invariants (Phase 7), and open questions (Phase 9) as you draft, and cite a shaping learning by wikilink at the row it shapes. `Findings: none` → note it and move on. A `stale?: yes` finding is a signal to weigh against present evidence, not a directive.

## Required sections

The body has these sections, in order: `## TL;DR`, `## Context and scope`, `## Non-goals`, `## Constraints and quality goals` (tech choices live here), `## Components` (table: component, responsibility, component-design link), `## Runtime flow` (target only; mark each step shipped or target), `## System invariants`, `## Decisions` (links), `## Open questions`. Optional, after Open questions: `## Solution strategy` (10 lines or fewer, plus decision links) and `## Risks`.

## Quick reference

| Phase | What | Gate |
|---|---|---|
| 1 Frame | Slug, product-design dependency, path | **Load-bearing** |
| 2 Context and scope | TL;DR, context, scope, non-goals | User confirms |
| 3 Constraints and quality goals | Tech choices and quality bars as constraints | User confirms |
| 4 Components | 3-8 components table, component design yes/no | **Load-bearing** |
| 5 Runtime flow | Target-only mermaid sequence diagram | User confirms |
| 6 Boundaries | Mermaid context diagram, in Context and scope | User confirms |
| 7 System invariants | 3-7 cross-component absolutes | **Load-bearing** |
| 8 Decisions | Decision wikilinks (or TODOs) | User confirms |
| 9 Solution strategy, open questions | Strategy (10 lines max), open questions | User reads cold |
| 10 Risks (optional) | 3-7 bullets | User confirms |
| 11 Serialize & save | Save, activate, link decisions | User reads cold |

## Common mistakes

- **Skipping Phase 1's product-design check.** Without the product design, Phase 4 has no anchor and components drift toward implementation taste rather than product fit. Stop and hand off.
- **Soft invariants in Phase 7.** "We try to..." is not an invariant. If the user shrugs at a candidate, strip it.
- **"Today" statements.** State the target only. Current state decays; it lives in code and component designs.
- **Duplicated invariants.** A system invariant is cross-component. Link a component rule; never copy it.
- **Unrecorded tech choices.** Record a non-trivial choice as a decision (`anvil create decision`) and link it under Decisions.
- **Mermaid as decoration.** Phases 5 and 6 require diagrams as core content. A system design without a context diagram is incomplete.
- **AI-generic Solution strategy prose.** Cite the user's own words; reference the product-design and decisions; don't generate filler.
- **Conflating system design with planning.** Components are responsibilities, not work items. If a section reads like a task list, it belongs in a milestone or issue.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.

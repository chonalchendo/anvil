---
type: product-design
title: "Anvil: product design"
project: anvil
created: 2026-04-27
updated: 2026-04-27
status: active
tags: [domain/dev-methodology, type/product-design]
target_users:
  - "Solo developers using AI to grow as engineers, not just ship faster"
  - "Small teams that want AI-assisted work without sprint ceremony"
problem_statement: "AI dev tools either pile on ceremony or take the wheel. Neither makes the user a stronger engineer."
success_metrics:
  - "Daily users touch the CLI ≤2 times/day; everything else is conversation"
  - "Users report Anvil makes them stronger engineers (qualitative survey on first 10 v0.1 users)"
  - "Brand-new project bootstrap (idea → first plan) takes ≤30 minutes of user-active time"
  - "Skills auto-fire correctly on intent ≥80% of the time without explicit invocation"
  - "Token cost per active session stays bounded as bundled skills grow — no mantle-style compiled-context bloat"
  - "Anvil itself is buildable using Anvil — the methodology bootstraps its own development"
out_of_scope:
  - "Project management (Linear/Jira/GitHub Issues — Anvil augments via lifecycle hooks, doesn't replace)"
  - "Orchestrator framework (Claude-Flow, Conductor — Anvil produces context they consume)"
  - "Docs hallucination fix (use Context7)"
  - "Skill marketplace, registry, or recommendations"
  - "Permission system / corporate access control"
  - "Per-task skill filtering (compile copies all enabled skills; progressive disclosure handles relevance)"
  - "Magic — it's a disciplined way to work that compiles to four formats"
milestones:
  - "[[milestone.anvil.m1-v0.1-minimal-usable-anvil]]"
  - "[[milestone.anvil.m2-v0.2-codex-parity-and-concurrent-waves]]"
  - "[[milestone.anvil.m3-v0.3-educational-gate-and-workspaces]]"
  - "[[milestone.anvil.m4-v0.4-iterate-from-real-signal]]"
related: []
revisions:
  - { date: 2026-04-27, change: "Initial draft, carved from docs/design.md" }
---

## What we're building

Anvil is a methodology for AI-assisted development packaged as auto-loading SKILL.md files with a thin Python orchestrator and a personal knowledge vault.

Three layers, each doing one thing. Skills are the methodology — auto-firing markdown the agent loads from conversational triggers, not commands the user has to type. The orchestrator is a small Python CLI for what genuinely needs a process: subprocess management, persistent state across sessions, telemetry. The vault is curated knowledge — learnings, decisions, skills — that travels with the user across projects.

The product is opinionated about what counts as a feature. Cost discipline is structural — lazy loading, cache breakpoints, model profiles — not a dashboard the user has to read. Educational gates offer themselves but never block; the user has final agency every time. Less surface area beats more capability.

Skills are hypotheses about recurring patterns, validated by use. They come from successful sessions, accumulated learnings, or focused research — not from invented patterns. Confidence accrues through reuse, not authoring effort. Skills that don't earn their reuse get retired.

The user touches the CLI twice a day at most. Everything else is conversation.

## Who it's for

Solo developers and small teams using AI assistants who want to grow as engineers. They believe in TDD, simplicity, and deliberate practice — and that some things are worth doing yourself even when the agent could do them.

Not for users who want sprint ceremonies, story points, or stakeholder syncs. BMAD and Spec Kit fit that better. Also not for users who want full autopilot — Anvil insists you stay in the driver's seat.

## Why it matters

Two failure modes dominate AI dev tooling. Sprint frameworks like BMAD and Spec Kit pile on ceremony solo developers don't need. Autopilot tools cede craft entirely: review the diff, ship, repeat. The first wastes time. The second atrophies judgement.

Anvil's bet: be stubborn about the design and vision, flexible about the implementation. A design-driven artifact hierarchy — product-design → milestones → plans → sweeps → issues → inbox — keeps every low-level task traceable to a higher-level intent. Skills handle the flexible part: reusable workflows that auto-fire on conversational triggers, not commands the user has to remember. The shape stays disciplined; the work itself stays adaptive.

Educational gating is the other half. AI should make the user a stronger engineer, not just a faster shipper. Mantle, the predecessor, reached v0.23.0 with a sophisticated workflow but accumulated 30+ slash commands; the gate was always in the design but never landed under the ceremony fatigue. Anvil starts smaller so the gate has room to fit without becoming one more demand.

## What success looks like

**Daily users touch the CLI ≤2 times/day.** The CLI is for orchestration, not for navigating the methodology. More than twice a day means the conversation surface has gaps and the methodology is leaking into ceremony.

**Users report Anvil makes them stronger engineers.** Measured via informal survey of the first ten v0.1 users. Qualitative, hand-wavy — but the commitment to ask is the point. If users feel faster but not stronger, Anvil is failing its educational gate thesis.

**Brand-new project bootstrap takes ≤30 minutes of user-active time.** From `anvil init` to a first plan ready to execute, including the design-driven hierarchy walk: product-design → milestone → plan. Longer than that and the design-side methodology is too heavy for small projects.

**Skills auto-fire correctly on intent ≥80% of the time.** Measured by counting explicit `Use skill X` overrides in real sessions. Below 80% means descriptions need work; auto-firing is the unit of UX leverage.

**Token cost per active session stays bounded.** Mantle ate tokens via compiled-context injection on every command — the workflow itself became expensive. Anvil compiles once, lazy-loads skill bodies, and respects cache breakpoints. Bundled skill count can grow without daily-driver cost growing in lockstep.

**Anvil itself is buildable using Anvil.** The methodology bootstraps its own development — including this product-design and the `writing-product-design` skill that came out of the same session. If we have to step outside Anvil to ship Anvil, it's not yet the right shape.

## What's deliberately out of scope

**Project management.** Linear, Jira, and GitHub Issues already do this. Anvil augments them via lifecycle hooks rather than competing — replacing them is surface the audience already pays elsewhere for.

**Orchestrator framework.** Claude-Flow, Conductor, and similar handle agent loops and graph execution. Anvil produces the context they consume; building a competing one would compromise both.

**Docs hallucination fix.** Context7 solves this. Anvil benefits from it indirectly and doesn't need its own version.

**Skill marketplace, registry, or recommendations.** Skills come from git repos; GitHub search is the discovery layer. Centralized marketplaces invite ranking games and lock-in pressure that work against an open standard.

**Permission system / corporate access control.** Different teams use different skill sources, and that's fine. Access controls turn a methodology into infrastructure.

**Per-task skill filtering.** `anvil compile` copies all enabled skills; progressive disclosure (descriptions in the system prompt, bodies lazy-loaded) handles relevance at the right layer. Predicting which subset a task needs adds complexity for a problem the harness already solves.

**Magic.** Anvil is a disciplined way to work that happens to compile to four formats. If a decision starts to feel like magic — opaque, unfalsifiable — it's wrong.

## Milestones

This product is delivered through these structural milestones:

- [[milestone.anvil.m1-v0.1-minimal-usable-anvil]] — smallest shippable Anvil: ClaudeCode adapter, sequential build, 11 bundled skills, JSON Schemas, CI validation
- [[milestone.anvil.m2-v0.2-codex-parity-and-concurrent-waves]] — Codex adapter, concurrent wave execution with worktree isolation, brownfield adoption, vault skill source
- [[milestone.anvil.m3-v0.3-educational-gate-and-workspaces]] — learning-shaping skill + gate detection, cross-repo workspace concept, external-skill porting pattern
- [[milestone.anvil.m4-v0.4-iterate-from-real-signal]] — refinements driven by observed use; further adapters and skill ports as need arises

Full milestone artifacts (one file per milestone) are deferred to a later iteration of the bootstrap, when `anvil:defining-milestone` exists. Wikilinks above are deliberately unresolved until then.

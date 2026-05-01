---
title: "Anvil system design — skill-based execution"
tags: [domain/dev-tools, type/system-design-shard]
---

## Skill-based execution

Skills follow Anthropic's SKILL.md open standard — directory per skill with `SKILL.md`, optional `references/`, `scripts/`, `assets/`. Progressive disclosure: ~100-token metadata always loaded, body lazy-loaded, bundled resources on demand.

**Three sides:**
- **Meta** — skills that produce other skills (`writing-skills`, `extracting-skill-from-session`, `synthesizing-knowledge-skill`, `researching-domain`).
- **Design** — produce structural artifacts (`writing-product-design`, `writing-system-design`, `defining-milestone`, `decision-making`).
- **Execution** — produce operational artifacts (`creating-issue`, `creating-plan`, `implementing-a-plan`, `capturing-inbox`, `human-review`, `capturing-learnings`, `re-entry`, `pausing-work`).

**Core skills slated for v0.1 dogfooding** (a separate research pass defines each):
- `writing-skills` (meta — knowledge skill style guide)
- `extracting-skill-from-session` (meta — already created)
- `capturing-inbox`
- `creating-issue`
- `creating-plan`
- `defining-milestone`
- `implementing-a-plan`
- `write-verification-skill` (meta — produces project-specific verification skills)

**Design-side ordering enforced** — `writing-system-design` won't fire without a product design; `defining-milestone` won't fire without one of the design docs; `creating-plan` requires a milestone. The dependency chain is checked at the skill's first phase, not at the orchestrator level.

**Mermaid embedding.** Plans, system designs, and milestone roadmaps embed mermaid diagrams inline (wave graphs, gantts, dependency graphs). Diagrams are first-class artifact content, not appendices — diffable, render natively in Obsidian and GitHub, survive plugin churn.

Authoring rules — body length, ALL-CAPS triggers, namespace handoff, description budget, `# prettier-ignore` directive — live in [`skill-authoring.md`](../skill-authoring.md). This section captures only how the orchestrator consumes them.

**Auto-discovery.** On `anvil build`, the installer materializes a *filtered* skill set into each spawn's state dir: every skill named in the task's `skills_to_load` plus the always-on core (a small designated subset of bundled skills loaded into every spawn — concretely the orchestration entry points and any skill the build orchestrator invokes implicitly; the list lives in orchestrator config, not in plan frontmatter). The agent CLI's native skill loader picks them up by file presence. No manifest, no registry file (per invariant); the selector decides what's *in* the state dir, the loader decides what to surface.

**CI vs. orchestrator.** Body length, ALL-CAPS, namespace-handoff, negative-trigger, and aggregate description-budget checks run in CI against `skills/`. The orchestrator assumes its inputs are valid SKILL.md files and does not re-validate at runtime — validation that fires at spawn time is too late.

**Source registry.** v0.1 scans only the project's `skills/` directory. v0.2 adds the vault's `40-skills/` and external pack directories under `~/.anvil/packs/`.

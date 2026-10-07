# Writing System Design — Phase procedure

Loaded on demand from `writing-system-design/SKILL.md`. Each phase has an explicit user gate — don't skip them. Phases 1, 4, and 7 are load-bearing. State the target only: no "today" statements.

### Phase 1 — Frame (LOAD-BEARING)

- Confirm the slug from the existing product-design.
- Read it with `anvil show product-design {slug} --body`. **If it doesn't exist, stop.** Hand off to `writing-product-design`.
- Confirm the save command: `anvil create system-design --project {slug} --title … --description "<one line>" --body-file <f>`.

**Gate (load-bearing):** product-design exists and is read.

### Phase 2 — Context and scope

Draft `## TL;DR` (the shape in one or two sentences: "X is a three-layer system: skills, orchestrator, vault."), `## Context and scope` (what the system is, who and what it talks to), and `## Non-goals` (what this design deliberately excludes).

**Gate:** user confirms the shape and the non-goals.

### Phase 3 — Constraints and quality goals

Draft `## Constraints and quality goals`: language, framework, storage, deployment, plus quality bars (latency, privacy, cost). Tech choices are constraints; there is no Tech stack section. Link a decision for each non-trivial choice.

Record an unmade choice as a decision (see Phase 8), or mark it `TODO: record via anvil create decision`.

**Gate:** user confirms each constraint.

### Phase 4 — Components (LOAD-BEARING)

Draft `## Components` as a table: component, responsibility, component-design link.

- 3-8 components (more is a smell; fold related responsibilities).
- Every product-design goal maps to at least one component. The candidate-to-component map lives in the product design's Milestones list, not here.
- Link a component design (`writing-component-design`) only if the component has an interface others build against, state or invariants beyond this design's, or spans more than one milestone. Write it when the milestone building it starts.

**Gate (load-bearing):** no product-design goal is orphaned.

### Phase 5 — Runtime flow

Draft `## Runtime flow` with a **mermaid sequence diagram** of the critical path. Show the target flow only. Mark each step `shipped` or `target`.

```mermaid
sequenceDiagram
    participant User
    participant CLI
    participant Orchestrator
    User->>CLI: command (shipped)
    CLI->>Orchestrator: parsed spec (shipped)
    Orchestrator-->>User: result (target)
```

(Replace with the project's actual flow — do not ship the placeholder.)

**Gate:** user confirms the diagram captures the critical path.

### Phase 6 — Boundaries

Add a **mermaid context diagram** to `## Context and scope`: every external system the project talks to (CLIs, databases, file locations, hooks).

```mermaid
graph LR
    Anvil[anvil] --> ClaudeCLI[claude-code]
    Anvil --> Vault[Obsidian vault]
```

(Replace with the project's actual boundaries.)

**Gate:** user confirms no boundary is missing.

### Phase 7 — System invariants (LOAD-BEARING)

Draft `## System invariants`: 3-7 statements that must always be true. Planning checks against them; review verifies them.

- One level only. A system invariant is cross-component. Link a component rule; never copy it.
- Declarative and absolute. "We try to..." is not an invariant.

Examples (from anvil itself):
- "Each agent CLI subprocess gets an isolated `CLAUDE_CONFIG_DIR` / `CODEX_HOME`."
- "Telemetry is local-only without explicit opt-in."

**Gate (load-bearing):** user signs off on each. If the user shrugs, strip it.

### Phase 8 — Decisions

Draft `## Decisions`: wikilinks to decisions that authorized the choices above (Phase 11 copies them into frontmatter `authorized_by`). Create a missing one with `anvil create decision --title "<the choice>" --topic <topic> --description "<one line>" --tags domain/<d>,activity/system-design --body-file <f> --json`. `<f>` holds `## Context`, `## Decision`, `## Rationale`, `## Consequences`, `## Links`. Add `--allow-new-facet <facet>` for a new tag value. Unresolved links are acceptable: flag them `TODO: record via anvil create decision`.

**Gate:** list confirmed, or TODO list accepted.

### Phase 9 — Solution strategy and open questions

Draft `## Open questions`: unresolved items, each with an owner or the decision that would close it.

Optionally draft `## Solution strategy`: 10 lines or fewer, plus decision links. Cross-reference the product-design and decisions; don't restate.

**Voice check.** Audit for hedging, abstract framing, corporate-speak. Cite the user's own words where possible.

**Gate:** user reads cold; voice matches the project.

### Phase 10 — Risks (optional)

Draft `## Risks` only if load-bearing assumptions could fail. 3-7 bullets, each naming what could go wrong and what would signal it.

**Gate:** list confirmed, or section skipped.

### Phase 11 — Serialize & save

1. Check the body against SKILL.md §Required sections, in order. No Tech stack section; no "today" statements. Mermaid diagrams render.
2. Save with `anvil create system-design --project {project} --title "<title>" --description "<one line>" --body-file <file>`. It writes `status: draft`, validates on write, and must pass clean.
3. Activate: `anvil set system-design {project} status active`.
4. For each Phase 8 decision: `anvil set system-design {project} authorized_by --add "[[decision.{topic}.NNNN-{slug}]]"`.

**Gate:** user reads the artifact cold. If anything's off, fix and re-show.

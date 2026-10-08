---
name: writing-component-design
description: "Use when authoring or updating a component design. Triggers: 'write the X component design', 'design the internals of X', 'what does/does not X own', 'what is the interface of X'. Modes: author, update."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-component-design
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-06-02
  updated: 2026-10-07
  tags: [type/skill, activity/component-design]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Component Design

Workflow for creating or updating a component design: the per-component reference document. It has an interface-and-ownership core (required) plus optional internal design. Component designs are plural per project (one per component-family) and carry a registry-validated `kind`. A component design is reference (Diataxis): facts a reader looks up, no explanation or history.

## Mode selection

**Author mode** - no component design exists for this component yet; you distil its interface and boundaries from the codebase and design docs.

**Update mode** - a component design exists; you sharpen an entry or add a missing one.

Decide before Phase 1. If uncertain, run `anvil list component-design`.

## Component design skeleton (both modes)

`create` and `validate` reject a body missing any core heading, in order: `## Does`, `## Does not`, `## Interfaces`, `## Invariants`, `## Verification`.

**REQUIRED REFERENCE:** Use skills/writing-component-design/references/body-shape.md for the heading skeleton and per-heading content.

**Optional sections** (add only when they earn their place):

- `## Code design` - the delta only: how *this* component is shaped. Link `[[convention.<lang>]]`; never restate house-wide rules.
- `## Decisions` - links to decisions, not their prose.
- `## Open questions` - unresolved items.

**Forbidden:** a Purpose section, Risks, Flow, Decision tree, and system invariants restated from the system design. Record history in a decision, a learning, or the issue; an append-only `## Precedents` log is allowed but no reader loads it.

**Discriminating test for `## Code design`:** a rule belongs in a `[[convention.X]]` iff it would be copied verbatim into another project's component design. A rule specific to this component's architecture stays here.

**REQUIRED REFERENCE:** Use skills/writing-component-design/references/verification-strategy.md before writing ## Verification.

---

## Author mode

### Phase 1 - Discover layout

Read the project's CLAUDE.md (or AGENTS.md) for the vault root and project slug. Then:

```bash
anvil list component-design            # confirm none exists for this component
anvil component-design kinds list      # see registered kinds
anvil component-design kinds add <name> --desc "<one line>"   # only if none fits
```

### Phase 2 - Read the boundary

Identify the boundary from at least two of: the system design (`anvil show system-design <project>`), the codebase (package boundary, public surface, ownership comments), and issues that touched the boundary.

Write the core just in time, when the milestone building the component starts. Add optional sections only when the component has a convention to link, linked decisions, or open questions. When the parent system design exists, link it: `anvil set component-design <id> system_design "[[system-design.<project>]]"`.

### Phase 3 - Create

```bash
anvil create component-design \
  --title "<Component> component design" \
  --project <slug> \
  --kind <registered-kind> \
  --description "<one sentence - the component's primary responsibility>" \
  --body-file <body.md>
```

Compose `<body.md>` from the skeleton (`anvil create component-design --show-template` prints the headings). `create` checks the core only on a supplied body, so a bodiless create writes an unchecked empty skeleton.

**Gate:** run `anvil validate`, fix schema errors, then promote: `anvil transition component-design <id> active`.

---

## Update mode

### Phase 1 - Locate

```bash
anvil list component-design --json
anvil show component-design <id>
```

### Phase 2 - Classify and apply

Make the minimal edit in the component design file:

- **Sharpen an entry** - edit an ambiguous `Does`, `Does not` or `Invariants` line in place; add no redundant line.
- **New does-not or invariant** - append it. Record the cause (the issue or PR) in a decision, a learning, or the issue, not in the component design.
- **Interface change** - edit the operation's input, output, errors or conditions.
- **Code design delta** - add `## Code design` if absent. A rule that would copy verbatim into another project goes to a `[[convention.<lang>]]` via `writing-convention`; its wire-the-rail phase links the convention here.
- **Verification strategy** - add the Direct or Indirect check a recent issue had to invent.

When the component retires, archive or delete its component design.

### Phase 3 - Validate

```bash
anvil validate
anvil set component-design <id> updated <today-iso>
```

---

## Non-goals

- Routing (linking an issue to its component design) — use `anvil link` directly.
- Enforcing optional sections — `create` checks only the required core headings; this skill gates the optional ones.
- Lifecycle tags and command verification — out of scope for v0.1.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.

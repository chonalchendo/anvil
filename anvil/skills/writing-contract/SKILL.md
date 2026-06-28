---
name: writing-contract
description: "Use when authoring a component contract or appending a precedent. Triggers: 'write the X contract', 'record this boundary violation or code-design rule for X', 'what does/does not X own'. Modes: author (new) and update (append/sharpen)."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-contract
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-06-02
  updated: 2026-06-10
  tags: [type/skill, activity/contract]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Contract

Workflow for creating or updating a component contract — the per-component guardrail document — boundary (`does / does not`) plus component-specific code design — registered in the project vault. Contracts are plural per project (one per component-family) and carry a registry-validated `kind`.

## Mode selection

**Author mode** — no contract exists for this component yet; you are distilling its boundaries from the codebase and design docs.

**Update mode** — a contract exists; you are appending a precedent (a violated or clarified boundary) or sharpening an existing `does not` entry.

Decide before Phase 1. If uncertain, run `anvil list contract` to check whether a contract already exists.

## Contract skeleton (both modes)

Every contract body follows this skeleton:

```
## Does

- <component> owns <responsibility>.
- <component> is the single source of truth for <X>.

## Does not

- <component> does not <boundary that surprised someone or needs emphasis>.
- <component> does not own <Y> — that belongs to <other component>.

## Code design

- <component-specific design delta: how *this* component is shaped — file/package layout, a pattern to follow or avoid, an entry-seam rule>.
- House-wide language/tool style: `[[convention.<lang>]]` (canonical cross-project spec) — link, never restate.

## Decision tree

When in doubt: <brief heuristic for the hardest boundary call>.

## Precedents

> <iso-date> · issue/PR <id>: <one-sentence description of the boundary violation or clarification that produced this precedent>.
```

The `## Precedents` section is append-only. Never rewrite a precedent; add a new one.

`## Code design` holds *component-specific design deltas* (how this component is shaped) **+** `[[convention.<lang>]]` links to the house-wide style — never restated house-wide rules.

**The discriminating test** — a rule belongs in a `[[convention.X]]`, not the contract, iff it would be copy-pasted verbatim into another project's contract. If it is specific to *this* component's architecture, it stays in the contract. So: "use module-alias imports" is house-wide → it lives in `convention.python` and the contract just links it; "config is bound once at the `--env` entry seam" is this component's shape → it stays in the contract. The section is **optional but always considered** — omit only when the component has neither a design delta nor a governing convention to link.

---

## Author mode

### Phase 1 — Discover layout

Read the project's CLAUDE.md (or AGENTS.md) to learn the vault root and project slug. Then:

```bash
anvil list contract            # confirm no contract exists for this component
anvil contract kinds list      # see registered kinds; register a new one if needed
```

If no matching kind exists, register it before creating the contract:

```bash
anvil contract kinds add <name> --desc "<one-line description>"
```

### Phase 2 — Read the design boundary

Identify the component's boundary from at least two of:

- The system-design doc (`anvil show system-design <project>` or the file directly).
- The codebase — grep for the component's package/module boundary, public surface, and any existing comments that name ownership.
- Existing issues or precedents that touched the boundary.

Draft the `## Does` and `## Does not` lists before writing the file. The `## Decision tree` entry is one sentence capturing the hardest boundary call — skip it if no non-obvious case has surfaced yet.

For `## Code design`, apply the guess heuristic above and extract those rules now.

### Phase 3 — Create the contract

```bash
anvil create contract \
  --title "<Component> contract" \
  --project <slug> \
  --kind <registered-kind> \
  --description "<one sentence — the component's primary responsibility>"
```

Then open the created file and write the body using the contract skeleton above.

**Gate:** validate before promoting to `active`.

```bash
anvil validate
```

Fix any schema errors. Promote once the boundary is honest:

```bash
anvil set contract <id> status active
```

---

## Update mode

### Phase 1 — Locate the contract

```bash
anvil list contract --json      # find the contract id
anvil show contract <id>        # read current body
```

### Phase 2 — Classify the update

- **New precedent** — a boundary was violated or clarified by a real issue or PR. Append to `## Precedents`.
- **Sharpen a does-not** — an existing `does not` entry is ambiguous or incomplete. Edit the entry in-place; do not add a redundant entry.
- **New does-not** — a boundary omission was found. Append to `## Does not`. If it was discovered via an issue/PR, also add a `## Precedents` entry.
- **Code design rule** — a pattern surfaced during implementation. Apply the discriminating test: a *component-specific* delta goes in `## Code design` (add the section if absent); a rule that would copy-paste verbatim into another project belongs in a `[[convention.<lang>]]` (author via `writing-convention`, then link it here).

### Phase 3 — Apply the update

Open the contract file directly and make the minimal edit. Precedent format:

```
> <iso-date> · issue/PR <id>: <one sentence — what happened and what the boundary clarification is>.
```

Use today's date (ISO 8601). Reference the causing issue or PR by id — do not leave the precedent unanchored.

### Phase 4 — Validate and commit

```bash
anvil validate
anvil set contract <id> updated <today-iso>
```

---

## Non-goals

- Routing (linking an issue to its contract) — use `anvil link` directly.
- Enforcing the body shape at the CLI level — the schema keeps body prose-flexible; this skill guides the shape.
- Lifecycle tags and command verification — out of scope for v0.1.

---
name: writing-component-design
description: "Use when authoring a component design or appending a precedent. Triggers: 'write the X component design', 'design the internals of X', 'what does/does not X own', 'record a boundary violation for X'. Modes: author, update."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-component-design
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-06-02
  updated: 2026-06-10
  tags: [type/skill, activity/component-design]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Component Design

Workflow for creating or updating a component design — the per-component document — a required boundary half (`does / does not`, verification, precedents) plus an optional design half (interfaces, shape, invariants) — registered in the project vault. Component designs are plural per project (one per component-family) and carry a registry-validated `kind`.

## Mode selection

**Author mode** — no component design exists for this component yet; you are distilling its boundaries from the codebase and design docs.

**Update mode** — a component design exists; you are appending a precedent (a violated or clarified boundary) or sharpening an existing `does not` entry.

Decide before Phase 1. If uncertain, run `anvil list component-design` to check whether one already exists.

## Component design skeleton (both modes)

Every component design body carries the **boundary half** (required — `create` and `validate` reject a body missing `## Purpose`, `## Does`, `## Does not`, `## Verification` with `### Direct` / `### Indirect`, or `## Precedents`). The **design half** (optional) follows it.

```
## Purpose

<one or two sentences: what this component is for and who builds against it>.

## Does

- <component> owns <responsibility>.
- <component> is the single source of truth for <X>.

## Does not

- <component> does not <boundary that surprised someone or needs emphasis>.
- <component> does not own <Y> — that belongs to <other component>.

## Code design

- <component-specific design delta: how *this* component is shaped — file/package layout, a pattern to follow or avoid, an entry-seam rule>.
- House-wide language/tool style: `[[convention.<lang>]]` (canonical cross-project spec) — link, never restate.

## Verification

How a change under this component design is proven to work — the strategy an issue's `## Verification` predicates are drawn from, not invented per issue.

### Direct
- <in-tree checks for this component: the unit / e2e / regression suites and how to run them>.

### Indirect (live)
- <how you exercise a change for real under this boundary, driven through the built/installed/served artifact: e.g. ping the endpoint and assert the response / run the CLI verb / query the downstream table>.

## Decision tree

When in doubt: <brief heuristic for the hardest boundary call>.

## Precedents

> <iso-date> · issue/PR <id>: <one-sentence description of the boundary violation or clarification that produced this precedent>.

<!-- design half, all optional: fill per the gate in Author mode -->
## Interfaces
## Shape
## Flow
## Invariants
## Decisions
## Risks
```

The `## Precedents` section is append-only. Never rewrite a precedent; add a new one.

`## Code design` holds *component-specific design deltas* (how this component is shaped) **+** `[[convention.<lang>]]` links to the house-wide style — never restated house-wide rules.

**The discriminating test** — a rule belongs in a `[[convention.X]]`, not the component design, iff it would be copy-pasted verbatim into another project's component design. If it is specific to *this* component's architecture, it stays in the component design. So: "use module-alias imports" is house-wide → it lives in `convention.python` and the component design just links it; "config is bound once at the `--env` entry seam" is this component's shape → it stays in the component design. The section is **optional but always considered** — omit only when the component has neither a design delta nor a governing convention to link.

`## Verification` holds the component's **testing strategy** (Direct + Indirect). Verification is keyed by what the component *is*, not its language: an API is verified by hitting the endpoint in Python or Go, while one language verifies a CLI, a pipeline, and an API three different ways — so it lives here, not in the language convention (style only). The **Indirect (live)** part is the live check `completing-issue`'s Iron Law gates on; record any system topology it must respect (e.g. the prod registry is unreachable from dev → a vs-prod check is a prod-time step). An issue draws its predicates from these parts (`writing-issue` loads the component design): the component design names the strategy, the issue writes the command.

**Deriving the strategy — read the *targets* off the component design, ground the *approach* in three sources:**

The *what-to-verify* is read off the component design — don't invent it:

- **Direct** = the `## Does not` invariants + each `## Precedents` entry as a regression test + the component's failure mode (data-integrity → assert a downstream value; response shape → assert the typed model; idempotency → run twice, assert stable).
- **Indirect** = the component's real entry point (HTTP route / CLI verb / landed table) + the most-downstream **non-proxy** observable that proves the change worked.

The *how-to-verify* — the strategy keyed to what the component *is* (you verify an API vs an ingest job vs an infra/deploy path three different ways) — is grounded in **three sources, not the repo alone**: the **repo** (its invariants, real entry points, existing suites — above), your **training data** (the recognised approach for the component type — contract/endpoint tests for an API boundary, golden round-trip / data-quality assertions for ingest, smoke-after-deploy + idempotency for infra), and **online research** (corroborate the approach against current sources, taking only recognised industry experts, not arbitrary blogs). When the approach for a component type isn't already settled, dispatch an `anvil-researcher` subagent (`subagent_type: anvil-researcher` — topic and deliverable shape as fill-ins) before naming the strategy.

Test discipline and *style* (test-first, given-when-then, framework idiom) are **not** re-grounded here — they are inherited from the language convention (`[[convention.<lang>]]`), which research-grounds the style once. This skill grounds the verification *strategy*; the convention grounds the test *style*.

---

## Author mode

### Phase 1 — Discover layout

Read the project's CLAUDE.md (or AGENTS.md) to learn the vault root and project slug. Then:

```bash
anvil list component-design   # confirm none exists for this component
anvil component-design kinds list      # see registered kinds; register a new one if needed
```

If no matching kind exists, register it before creating the component design:

```bash
anvil component-design kinds add <name> --desc "<one-line description>"
```

### Phase 2 — Read the design boundary

Identify the component's boundary from at least two of:

- The system-design doc (`anvil show system-design <project>` or the file directly).
- The codebase — grep for the component's package/module boundary, public surface, and any existing comments that name ownership.
- Existing issues or precedents that touched the boundary.

**Design-half gate.** Fill the design half (`## Interfaces`, `## Shape`, `## Flow`, `## Invariants`, `## Decisions`, `## Risks`) only if the component (1) has an interface others build against, (2) has state or invariants beyond its parent system design's, or (3) spans more than one milestone or a handful of issues. Otherwise write the boundary half alone. Write it just in time, when the milestone building the component starts — not up front with the system design. When the parent system design exists, link it: `anvil set component-design <id> system_design "[[system-design.<project>]]"`.

Draft the `## Purpose`, `## Does` and `## Does not` sections before writing the file. The `## Decision tree` entry is one sentence capturing the hardest boundary call — skip it if no non-obvious case has surfaced yet.

For `## Code design`, apply the guess heuristic above and extract those rules now. For `## Verification`, name the component's Direct and Indirect strategy from what the component *is* — how its tests run, and how a change is exercised live through the real artifact.

### Phase 3 — Create the component design

```bash
anvil create component-design \
  --title "<Component> component design" \
  --project <slug> \
  --kind <registered-kind> \
  --description "<one sentence — the component's primary responsibility>" \
  --body-file <body.md>
```

Compose `<body.md>` from the skeleton above first (`anvil create component-design --show-template` prints the boundary headings); `create` checks the boundary half only on a supplied body, so a bodiless create writes an unchecked empty skeleton.

**Gate:** validate before promoting to `active`.

```bash
anvil validate
```

Fix any schema errors. Promote once the boundary is honest:

```bash
anvil set component-design <id> status active
```

---

## Update mode

### Phase 1 — Locate the component design

```bash
anvil list component-design --json      # find the component design id
anvil show component-design <id>        # read current body
```

### Phase 2 — Classify the update

- **New precedent** — a boundary was violated or clarified by a real issue or PR. Append to `## Precedents`.
- **Sharpen a does-not** — an existing `does not` entry is ambiguous or incomplete. Edit the entry in-place; do not add a redundant entry.
- **New does-not** — a boundary omission was found. Append to `## Does not`. If it was discovered via an issue/PR, also add a `## Precedents` entry.
- **Code design rule** — a pattern surfaced during implementation. Apply the discriminating test: a *component-specific* delta goes in `## Code design` (add the section if absent); a rule that would copy-paste verbatim into another project belongs in a `[[convention.<lang>]]` (author via `writing-convention` — its wire-the-rail phase links the new convention from every component design it governs, this one included; the frontmatter edge is idempotent, so check the component design's `## Code design` for an existing link line before adding one).
- **Verification strategy** — a Direct or Indirect check the component needs surfaced (a regression suite to name, a live-exercise step a recent issue had to invent). Add to `## Verification` (add the section if absent) so the next issue draws it instead of re-deriving it.

### Phase 3 — Apply the update

Open the component design file directly and make the minimal edit. Precedent format:

```
> <iso-date> · issue/PR <id>: <one sentence — what happened and what the boundary clarification is>.
```

Use today's date (ISO 8601). Reference the causing issue or PR by id — do not leave the precedent unanchored.

### Phase 4 — Validate and commit

```bash
anvil validate
anvil set component-design <id> updated <today-iso>
```

---

## Non-goals

- Routing (linking an issue to its component design) — use `anvil link` directly.
- Enforcing the design half — `create` checks only the boundary-half headings; this skill gates the design half.
- Lifecycle tags and command verification — out of scope for v0.1.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.

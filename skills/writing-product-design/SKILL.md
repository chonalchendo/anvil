---
name: writing-product-design
description: Use when starting a NEW project — vision, users, success, scope, milestones. Greenfield only (not brownfield carving). Not for system design (anvil:writing-system-design), one milestone (anvil:defining-milestone), tasks (anvil:creating-issue).
license: MIT
allowed-tools: [Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-product-design
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-04-27
  updated: 2026-05-12
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

- Architecture / implementation → `anvil:writing-system-design`.
- One milestone → `anvil:defining-milestone`.
- A discrete work item → `anvil:creating-issue`.
- Light revisions to an existing PD → direct edit, not a re-author.
- Brownfield carving — different activity; this skill does not handle it.

## Output path

Canonical destination: `~/anvil-vault/05-projects/<project>/product-design.md`. Vault-only — never committed to the project's source repo. Surface this at Phase 1 so the user can flag any can't-commit-anywhere constraint up front.

## The phases

Each phase has an explicit user gate. Don't skip them — gates are the load-bearing part. Each is an iteration loop; expect 1–2 reframes per phase.

### Phase 1 — Frame

Confirm scope: project slug, what counts as the product (one or several), destination path.

**Gate:** scope, slug, path confirmed.

### Phase 2 — Problem & users

Elicit from conversation; there is no source doc.

Draft body sections:
- **Why it matters** — leads with the one-sentence problem (2–4 short paragraphs).
- **Who it's for** — 1–3 user descriptions with implicit negative space (who it's *not* for; 2–3 short paragraphs).

**Voice check before drafting.** Sample the project's voice if any prose exists; audit drafts for AI-generic hedging, abstract framing ("In today's landscape..."), corporate-speak. Direct, declarative, specific. Concrete details, not analogies.

**Gate:** distillation reflects intent. Reframes expected.

### Phase 3 — What we're building (LOAD-BEARING)

Draft the **What we're building** body section:
- Lead with one-line product shape ("X is a Y for Z, packaged as…").
- Follow with user-facing convictions — what the user *feels*, not *how it's built*.
- End with one-line user-touch summary ("the user does X at most; everything else is Y").

**Keep *what* separate from *how*.** Implementation strategy, packaging, subprocess choices belong in `system-design.md`. Strip them.

**Gate (load-bearing):** premise confirmed. Phase 5 derives milestones from this — often takes 2–3 passes.

### Phase 3.5 — Approach (fat-marker sketch)

Draft the **Approach** body section: a fat-marker sketch of the solution shape. Gives Phase 5 something to derive from without breaching what-vs-how.

> **Broad strokes only — no architecture, no tech choices, no APIs.** If a bullet names a database, framework, library, or API, push it to `system-design.md`.

3–7 bullets at whiteboard altitude. Right altitude examples:

- "User runs one command per phase; everything else is automated."
- "Skills auto-load by file presence — no registry."
- "Telemetry is opt-in, local-only, queryable from the CLI."

Wrong altitude (push to system-design): "Use SQLite via modernc.org driver", "Cobra command tree with three subcommands".

**Gate:** altitude is right — too detailed → strip; too vague → expand.

### Phase 4 — Goals, success, constraints, out-of-scope (LOAD-BEARING)

All four are body sections (prose under headings); no frontmatter arrays.

**Past-pain → metrics prompt (critical).** Ask explicitly: *what should success measurably not be? What failure modes from past tools do you want a metric guarding against?* Old-tool failure modes are the strongest source of concrete measurable success criteria.

Draft body sections:

- **Goals** — outcome-shaped (≥1). Distinct from metrics. Examples:
  - "Users feel the tool is on their side" — goal (qualitative)
  - "≥80% of plans auto-fire on first try" — metric (measurable)

  If a candidate has a number in it, it's probably a metric. If it describes a felt experience, it's a goal. Both required.

- **Constraints & appetite** (Shape Up). Constraints are usually fixed-time; scope is the variable. Appetite is the explicit time box (`small-batch` 1–2 weeks / `big-batch` 4–6 weeks / explicit duration). 2–5 constraint bullets mixing capacity, deadline, dependency. "v0.1 ships in 6 weeks" is a constraint; "no UI" is out-of-scope.

- **What success looks like** — 3–5 success metrics blending quantitative ("≤30 min to first plan") and qualitative ("users report stronger engineers"). For qualitative, name how it'll be measured (informal survey, telemetry signal). Commitment to measurement is the point.

- **What's deliberately out of scope** — 5–7 items shaped as `<topic> — <why this is out, not just "we won't do it">`. Negative space prevents scope creep; "why" turns a no into a no with reasons.

**Gate (load-bearing):** explicit sign-off on goals, metrics, constraints, appetite, out-of-scope. **Time-box:** if any list doesn't land in two iterations, write `TODO: validate after two weeks of real use` placeholders.

### Phase 4.5 — Risks, rabbit holes, open questions

Draft the **Risks, rabbit holes, open questions** body section. Brief — single gate, ≤5 minutes. Placeholders allowed.

Prompt: What could derail this? What rabbit hole are you afraid of? What's still genuinely open?

3–7 bullets. Examples:
- "Subprocess streaming buffer overflow on long tool-result lines"
- "Companion-pack drift if Superpowers reshapes its skills"
- "Open: should skills be content-addressed or path-based?"

**Gate:** list confirmed. Naming a risk often reveals it's actually a constraint or out-of-scope item — accept the reframe.

### Phase 5 — Initial milestones

Draft the **Milestones** body section: titles as wikilinks `[[milestone.<project>.<slug>]]` plus one-line summaries.

Structural links: add each wikilink to the artifact's `related` frontmatter array (the universal link slot). The milestone's child→parent link is `product_design` on the milestone side.

**REQUIRED SUB-SKILL:** `anvil:writing-milestone` (a.k.a. `defining-milestone`).

If unavailable in v0.1, collect titles + summaries inline; wikilinks stay unresolved until the sub-skill exists.

**Gate:** breakdown confirmed.

### Phase 6 — Serialize & save

1. Flip frontmatter `status: draft` → `active`. Bump `updated` to today.
2. Hand-check against `schemas/product-design.schema.json`:
   - Required frontmatter: `type, title, description, created, status, project`.
   - Optional frontmatter: `updated, tags, aliases, related`.
   - **No other frontmatter fields** — schema is `additionalProperties: false`. If you wrote `goals:`, `risks:`, `milestones:`, `target_users:`, `revisions:` as frontmatter arrays, move them to body sections.
   - Body has these sections in order: What we're building / Who it's for / Why it matters / Approach / Goals / Constraints & appetite / What success looks like / What's deliberately out of scope / Risks, rabbit holes, open questions / Milestones.
   - Wikilinks under Milestones (and mirrored in `related`) are well-formed `[[milestone.<project>.<slug>]]`.
3. Run `anvil validate <path>` — must pass clean.
4. Write to `~/anvil-vault/05-projects/<project>/product-design.md`.

**Gate:** user reads the artifact cold. Capture the project's vision? Fix and re-show if not.

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

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
  updated: 2026-04-27
  tags: [type/skill, activity/product-design]
  diataxis: how-to
  authored_via: extracting-skill-from-session
  confidence: low
  status: in-use
---

# Writing Product Design

A six-phase workflow for authoring a project's product-design artifact — the top of Anvil's design-driven hierarchy (product-design → milestones → plans → sweeps → issues → inbox). Greenfield only: the user is starting a new project, not carving from existing docs.

## When to use

- Starting a new project and needing the vision artifact before milestones, plans, or code.
- User says "let's product-design X", "what's X for", or anything that signals defining the product (not how to build it).
- Bootstrapping the artifact hierarchy for a vault project.

## When not to use

- Architecture, implementation choices → `anvil:writing-system-design`.
- Authoring a single milestone in detail → `anvil:defining-milestone`.
- Capturing a discrete work item → `anvil:creating-issue`.
- Light revisions to an existing product-design (date bump, append revisions entry, add a new milestone wikilink) — that's a frontmatter edit, not a re-author.
- Brownfield carving (lifting product-design content out of an existing design doc) — different activity. Today this skill does not handle that case.

## Output path

Canonical destination: `~/anvil-vault/05-projects/<project>/product-design.md`. Vault-only — never committed to the project's source repo. The schema lives in `docs/design.md` lines ~647-696 (TODO: update this pointer to `anvil/schemas/product-design.schema.json` once schemas land).

Surface this path at Phase 1, not Phase 6, so the user can flag any can't-commit-anywhere constraint up front.

## The six phases

Each phase has an explicit user gate. Don't skip the gates — they're the load-bearing part. Treat each as a real iteration loop; expect 1-2 reframes per phase as the user finds the right framing.

### Phase 1 — Frame

Confirm scope with the user:
- What's the project? Pick a slug.
- What counts as the product — one product or several?
- Confirm the destination path: `~/anvil-vault/05-projects/<slug>/product-design.md`.

**Gate:** user confirms scope, slug, and path. Trivial gate, but don't skip it — wrong scope here propagates.

### Phase 2 — Problem & users

Elicit from **conversation**, not from any source document. There is no source doc — the user's intent is the source.

Distill to:
- `problem_statement` — one sentence.
- `target_users` — 1-3 entries, each with implicit negative space (who it's *not* for).

Draft body prose for "Who it's for" (2-3 short paragraphs) and "Why it matters" (2-4 short paragraphs).

**Voice check before drafting.** Ask the user for the project's voice, or sample existing prose if any exists. Audit drafts for AI-generic hedging, abstract framing ("In today's landscape..."), and corporate-speak. Direct, declarative, specific. Concrete details, not analogies.

**Gate:** user confirms the distillation reflects intent. Expect reframes — the user often finds the right framing only after seeing a draft. Iterate.

### Phase 3 — What we're building (LOAD-BEARING)

Draft the "What we're building" body section:
- Lead with one-line product shape ("X is a Y for Z, packaged as...").
- Follow with the user-facing convictions — what the user *feels*, not *how it's built*.
- End with one-line user-touch summary ("the user does X at most; everything else is Y").

**Keep what separate from how.** If a sentence describes implementation strategy, packaging, subprocess choices, or compilation pipelines, it belongs in `system-design.md`. Strip it out.

**Gate (load-bearing):** user confirms the premise. Phase 5 derives milestones from this — don't rush. If the user reframes the premise, accept the iteration; this phase often takes 2-3 passes.

### Phase 4 — Success & out-of-scope (LOAD-BEARING)

Both fields are net-new authoring — no source doc to lift from.

**Past-pain → metrics prompt (critical).** Ask the user explicitly: *what should success measurably not be? What failure modes from past tools do you want a metric guarding against?* Old-tool failure modes are the strongest source of concrete measurable success criteria. Don't lose them to politeness.

Author 3-5 `success_metrics`:
- Blend quantitative ("≤30 minutes to first plan") and qualitative ("users report stronger engineers").
- For qualitative metrics, name how it'll be measured (informal survey, feedback loop, signal in usage telemetry) — even if hand-wavy. The commitment to measurement is the point.

Author 5-7 `out_of_scope` items:
- Each as `<topic> — <why this is out, not just "we won't do it">`. The negative space prevents scope creep; "why" turns a no into a no with reasons.

Draft body sections "What success looks like" and "What's deliberately out of scope" with 1-2 sentence justifications per item.

**Gate (load-bearing):** user signs off **explicitly** on both lists. **Time-box:** if metrics don't land in two iterations, write placeholders (`"TODO: validate after two weeks of real use"`) and accept that v0.1 product-design is itself a draft. Don't stall here.

### Phase 5 — Initial milestones

Break the product into structural milestones from the user's roadmap intuition. Collect:
- Milestone titles as wikilinks: `[[milestone.<project>.<slug>]]`. Unresolved is fine; malformed is a bug.
- One-line summaries.

**REQUIRED SUB-SKILL:** Use `anvil:defining-milestone`

If `anvil:defining-milestone` is not yet available (v0.1 may not have it), collect milestone titles + one-line summaries inline and stop there. Wikilinks stay unresolved until the sub-skill exists; that's accepted v0.1 behavior, not a blocker.

**Gate:** user confirms the breakdown. Common reframes: split a milestone, merge two, drop one entirely.

### Phase 6 — Serialize & save

1. Populate frontmatter from body. Replace any remaining placeholders.
2. Flip `status: draft` → `status: active`.
3. Append a `revisions:` entry: `{ date: <today>, change: "Initial draft" }`.
4. Hand-check against the schema in `docs/design.md` lines ~647-696. (TODO: update this pointer to `anvil/schemas/product-design.schema.json` once schemas land.) Verify:
   - All schema-required frontmatter fields present (`type`, `title`, `project`, `created`, `updated`, `status`, `tags`, `target_users`, `problem_statement`, `success_metrics`, `out_of_scope`, `milestones`, `related`, `revisions`).
   - Body has six sections in schema order: What we're building / Who it's for / Why it matters / What success looks like / What's deliberately out of scope / Milestones.
   - Wikilinks in `milestones:` are well-formed `[[milestone.<project>.<slug>]]`.
   - `revisions:` has at least one entry with today's date.
5. Write to `~/anvil-vault/05-projects/<project>/product-design.md`.

**Gate:** user reads the artifact cold. Does it capture the project's vision? If anything's off, fix and re-show.

## Quick reference

| Phase | What | Gate |
|---|---|---|
| 1 Frame | Project scope, slug, destination path | Trivial |
| 2 Problem & users | `problem_statement`, `target_users`, body prose | User confirms |
| 3 What we're building | Body section | **Load-bearing** |
| 4 Success & out-of-scope | `success_metrics`, `out_of_scope`, both bodies | **Load-bearing** |
| 5 Milestones | Wikilinks + summaries | User confirms |
| 6 Serialize & save | Frontmatter, hand-check, write | User reads cold |

## Common mistakes

- **Drafting from a source doc.** Greenfield: there is no source. Elicit from conversation. If you find yourself reading "lines X-Y of file Y", stop — that's brownfield carving (a different activity).
- **Conflating what with how.** Phase 3 captures *what we're building*. Implementation strategy, packaging choices, and subprocess decisions belong in `system-design.md`. Strip them.
- **Generic success metrics.** "Users are happy" is not a metric. Blend quantitative and qualitative; tie qualitative to a measurement plan.
- **Skipping the past-pain prompt in Phase 4.** Old-tool failure modes are the most concrete metrics — they're the user's lived experience compressed into a measurable bar. Don't lose them to politeness.
- **Voice drift.** AI-generic prose ("In today's landscape...", "the modern paradigm...") fails the cold-read test. Match the project's voice before drafting; audit for hedging and corporate-speak.
- **Treating gates as one-shot approvals.** Each gate is an iteration loop. The user often reframes after seeing a draft — that's the gate working, not failing.
- **Skipping the destination-path confirmation at Phase 1.** Surfacing it late can force a rework if the user can't commit to a particular location.

---
name: writing-milestone
description: "Use when scoping a shippable bundle of work into a milestone (product/system design must already exist). Triggers: 'scope a milestone', 'what's the next milestone', 'M1', 'M2', 'define M3'."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-milestone
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-04-30
  updated: 2026-06-04
  tags: [type/skill, activity/milestone]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Milestone

Workflow for creating a milestone artifact via the `anvil` CLI. Milestones sit one level below the design docs in Anvil's hierarchy: product-design → **milestones** → plans → issues.

## When this skill runs

- A product-design or system-design exists for the project.
- User wants to carve the next shippable increment (M1, M2, etc.).
- Before any issues are written for that increment.

## When not to use

- No design doc exists yet → use `writing-product-design` or `writing-system-design` first.
- Work item level (a task, bug, feature) → `writing-issue`.
- Editing existing milestone frontmatter only (date bump, status flip) — that's a direct `anvil set` call, not this workflow.

---

## Phase 1 — Read the design doc

Find the active project, then read the design doc.

```bash
anvil project current
```

The design doc lives at `05-projects/<project>/product-design.md` or `05-projects/<project>/system-design.md` inside the vault. Read the file at that path directly — designs are not yet typed artifacts in the CLI.

Confirm with the user which design doc to derive from (product or system, or both).

**Gate:** user confirms which design doc drives scope.

---

## Phase 2 — Shape the milestone body

Draft the following before calling the CLI:

- **title** — one line; verb-noun ("Ship X", "Validate Y", "Deliver Z").
- **goal** — one sentence, ≤120 chars: the terminal predicate (what "done" means for this milestone), mirroring how issues carry a `goal`. Required by the schema; `anvil create milestone` fails without it.
- **kind** — `scoped` (the default — discrete shippable bundle with acceptance criteria) or `bucket` (rolling-findings tracker; `acceptance` stays `[]`). Pick `bucket` only for friction-collection milestones; everything else is `scoped`.
- **acceptance** — testable conditions for "done"; each must be a runnable predicate (a command that exits 0/1, or an observation a reader can re-check without ambiguity), never prose that merely looks testable. Required substance for `kind: scoped`.

### Finish-line gate (scoped only)

A scoped milestone must have a **witnessable finish line** — a point a future agent can run and see is reached. Two ways it silently fails to, both of which you **must refuse** here rather than carry into Phase 3:

- **State-phrased goal.** The `goal` names an ongoing condition ("docs *stay* accurate", "the CLI *remains* fast") instead of an event. A persisting state never closes, so the milestone never ends. Refuse it: rewrite the goal as a terminal predicate ("docs match the shipped flags as of <sha>"), or — if the work genuinely is open-ended collection — flip to a bucket (below).
- **Silent empty acceptance.** `acceptance` is left `[]` on a `kind: scoped` milestone. That is the bucket anti-pattern smuggled into the wrong kind: a scoped milestone with no closeable AC. Refuse it — do not proceed with empty acceptance on a scoped milestone.

The author resolves a refusal one of two ways, deliberately:

1. **Write the finish line** — supply at least one runnable-predicate acceptance criterion (and an event-phrased goal). This is the default.
2. **Explicit bucket affirmation** — the work really is a rolling-findings tracker with no end. Affirm it out loud, then flip `kind` to `bucket` in Phase 3; `acceptance: []` is then legal *because the choice was made, not defaulted into*.

Buckets stay legal. The gate only forbids the *silent* path — a scoped milestone that can never end without anyone having chosen that.

**Gate:** user confirms title, goal, kind, and acceptance criteria — and, for a scoped milestone, that the goal is event-phrased and acceptance carries at least one runnable predicate; for a bucket, that the open-ended kind was explicitly affirmed.

---

## Phase 3 — Create

```bash
anvil create milestone \
  --title "<title>" \
  --description "<one-line preview>" \
  --goal "<terminal predicate>" \
  --json
```

Capture `id` and `path` from the JSON output. The artifact ships with `kind: scoped` by default. If this is a bucket milestone, flip it now:

```bash
anvil set milestone <id> kind bucket
```

Then direct-edit the body sections (objectives, success criteria, non-goals) into the file the CLI created at `path`.

---

## Phase 4 — Link to design docs

```bash
anvil set milestone <id> product_design "[[product-design.<project>]]"
anvil set milestone <id> system_design "[[system-design.<project>]]"
```

Both calls land in dedicated typed slots. If a system-design doesn't yet exist, omit the second call.

---

## Phase 4b — Contract coverage

Name the component families this milestone owns (read them off the acceptance criteria), then see which already have a governing contract:

```bash
anvil list contract --json
```

A contract **accretes from building**, so it is authored at the start of the milestone that owns the family (`docs/system-design.md`, contract cadence). For an **extremely obvious** uncovered family — one this milestone plainly owns and builds against — surface the gap and, on the user's confirmation, fire `writing-contract` (author mode) for it. Skip silently when every family the milestone owns is already covered, or when the gap is ambiguous — never author speculatively. This is the authoring end of the cadence; `writing-issue` Phase 4b stays link-only.

**REQUIRED SUB-SKILL (on confirmed gap only):** Use `writing-contract`.

---

## Phase 5 — Validate

```bash
anvil show milestone <id> --validate
```

Fix any schema errors reported. Re-run until clean.

---

## Hand-off

**REQUIRED SUB-SKILL:** Use `writing-issue`.

Next: `writing-issue` for the first issue under this milestone.

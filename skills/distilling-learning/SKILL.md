---
name: distilling-learning
description: "Use when the user explicitly says \"let's distill\" / \"wrap this up into a learning\" / \"extract what we learned from <source>\". Source-agnostic: a thread + sessions, a completed plan, a transcript, or an ad-hoc reflection. Produces N tagged learnings (not 1) keyed by diataxis + confidence + four-facet tags so future agents can retrieve them via `anvil list learning --diataxis ... --tags ... --confidence ...`. Not for active research (stay in the thread); not for one-off thoughts (use anvil:capturing-inbox); not for summaries that aren't durable claims."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: distilling-learning
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-05-01
  updated: 2026-05-01
  tags: [type/skill, activity/distillation]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Distilling Learning

Workflow for crystallizing thinking into retrievable knowledge artifacts. Distillation sits in the **knowledge pipeline**, parallel to the build pipeline. Threads are the workspace; **learnings are the durable output**.

The terminal contract is retrieval: every learning must be reachable later via `anvil list learning --diataxis X --tags domain/Y,activity/Z --confidence W`. If a draft can't be retrieved that way, the tags are wrong.

## When this skill runs

- The user explicitly commits to distilling: "let's distill", "wrap this up into a learning", "extract what we learned from <source>".
- A source artifact exists and is named: a thread, a completed plan, a transcript, or a free reflection.

## When not to use

- The source isn't crystallized yet (still actively researching) → keep working in the thread.
- The output would just restate the source — distillation requires a durable claim or piece of know-how, not a summary.
- The user wants to capture a one-off thought without taxonomy commitment → `anvil:capturing-inbox`.

---

## Phase 1 — Identify the source

Confirm one of the following with the user, then read the relevant files:

| Source kind | What to read |
|---|---|
| Thread + sessions | The thread file plus every session linked via `related: [[thread.<id>]]` |
| Completed plan | The plan file plus build artifacts it produced |
| Transcript | The transcript file |
| Reflection | The conversation context only |

```bash
anvil show thread <id>          # if thread source
anvil list session --tag thread/<id>   # if thread source, find linked sessions
```

---

## Phase 2 — Decide cardinality

N learnings per pass, not 1. Each learning crystallizes **one** claim or **one** piece of know-how. Two related claims become two learnings — never bundle.

Draft a list of candidate learnings. For each, pick:

- **Title** — the claim, phrased as a noun phrase (`"FK locks block writes during backfill"`, not `"how to handle locks"`).
- **`diataxis`** — `tutorial | how-to | reference | explanation`. Default `explanation` for claims; `how-to` for procedural know-how; `reference` for catalogued options; `tutorial` only for end-to-end teaching material.
- **`confidence`** — `low | medium | high`. Default `low`. Promote to `medium` only if backed by a primary source the user has read; `high` only if also independently verified (replicated, peer-reviewed, run in production).

**Gate:** present the draft list (titles + `diataxis` + `confidence`) to the user. User prunes/edits before any file is written.

---

## Phase 3 — Tag each learning

Tags are **mechanism**, not vocabulary. Use the four-facet system; the actual values come from `_meta/glossary.md`.

```bash
anvil glossary tags --prefix domain/      # see existing domain tags
anvil glossary tags --prefix activity/    # see existing activity tags
anvil glossary tags --prefix pattern/     # see existing pattern tags
```

For each learning, propose tags drawn from existing glossary values first. Only invent a new tag if no existing one fits — and only after the user approves it. New tags must:

- Be lowercase ASCII, hyphens only (no spaces, no underscores, no caps).
- Have shape `<facet>/<name>` where facet is one of `domain | activity | pattern`. (`type/` is auto-managed; `status/` is forbidden — status is a frontmatter field.)
- Be added via `anvil glossary add tag <facet>/<name> --desc "..."` BEFORE you write the learning, otherwise `anvil validate` will reject the learning.

Always include `type/learning` in the tag list. Never include a `status/*` tag.

---

## Phase 4 — Create + populate each approved learning

For each approved learning, in order:

```bash
anvil create learning --title "<claim>" --json
# capture {id, path}
anvil set learning <id> diataxis <tutorial|how-to|reference|explanation>
anvil set learning <id> confidence <low|medium|high>
```

Then direct-edit the file body. The body **MUST** contain three H2 sections in this exact order:

```markdown
## TL;DR

One paragraph. The claim, why it matters.

## Evidence

Sources, session quotes, plan outcomes, transcript references that ground the claim.
Use wikilinks: `[[session.<id>]]`, `[[plan.<id>]]`, etc.

## Caveats

What would change `confidence`. What's still unknown. Limits of applicability.
```

Progressive disclosure: future agents query `anvil list learning ...`, see frontmatter + (eventually) the TL;DR; only drill into Evidence + Caveats when needed.

Then set tags. Tags live in YAML frontmatter only — never as body `#hashtags` (Obsidian splits those on space and corrupts multi-word values). Edit the `tags:` list directly:

```yaml
tags:
  - type/learning
  - domain/postgres
  - activity/debugging
```

Backlink the source:

```bash
anvil link learning <id> <source-type> <source-id>
```

---

## Phase 5 — Glossary additions

If new tags were approved in phase 3, append them now (one per new tag):

```bash
anvil glossary add tag domain/<new-name> --desc "<one-line description>"
```

Same commit as the learning files.

---

## Phase 6 — Source aftermath

| Source kind | Aftermath |
|---|---|
| Thread | Ask user: `closed | paused | stay open`. If `closed` and active, run `anvil thread deactivate`. Apply via `anvil set thread <id> status <state>`. |
| Plan | None. |
| Transcript | None. |
| Reflection | None. |

Session folder hygiene (raw → distilled) is **not** this skill's concern — it belongs to vault GC.

---

## Phase 7 — Validate

```bash
anvil validate
```

This checks:
- Schema correctness on every learning frontmatter.
- Body contains `## TL;DR / ## Evidence / ## Caveats` in order.
- Tags are lowercase-ASCII-hyphen.
- Every non-`type/` tag is present in the glossary.
- `type/learning` tag is present and matches.

Fix any failures and re-run until clean.

---

## Verify retrieval

End-to-end: confirm the learnings are queryable on the axes you tagged them with.

```bash
anvil list learning --tags domain/<X>,activity/<Y> --diataxis <Z> --confidence <W>
```

Each new learning should appear in at least one such query. If a learning isn't reachable, the tags are wrong — fix and re-validate.

---

## Hand-off

The pipeline ends here for the source. Possible next moves the user may signal:

- Continue the thread (don't close it; new sessions will keep stacking).
- Open a new thread on a follow-up subquestion → `anvil:opening-thread`.
- Surface project work the distillation revealed → `anvil:writing-issue`.
- Extract a methodology lesson into a skill → `anvil:extracting-skill-from-session` (orthogonal track).

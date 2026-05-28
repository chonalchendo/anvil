---
name: recording-learning
description: "Use when a concrete verified claim crystallises mid-session and must be persisted now. Triggers: 'record this learning', 'this is a finding', 'save this insight'. Not for fleeting thoughts (capturing-inbox) or bulk extraction (distilling-learning)."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: recording-learning
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-05-28
  updated: 2026-05-28
  tags: [type/skill, activity/distillation]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Recording a Learning

Workflow for committing one concrete, verified claim to the vault mid-session. Distinct from `distilling-learning`, which runs post-session over a named source artifact. Use this when the claim is ready now and waiting until session end would risk losing it.

The terminal contract is retrieval: the learning must be reachable later via `anvil list learning --diataxis X --tags domain/Y,activity/Z`. If it can't be retrieved that way, the tags are wrong.

## When this skill runs

- A concrete claim has crystallised during active work: a verified diagnosis, an observed contract, a "this works / this doesn't" result.
- The user wants it persisted now, not deferred to post-session distillation.
- The claim is durable and specific enough to be phrased as a noun phrase.

## When not to use

- The thought is still half-formed or unverified → `capturing-inbox`.
- Multiple claims from a finished source (thread, transcript, plan) → `distilling-learning`.
- The "learning" is really a task or follow-up → `writing-issue`.

---

## Phase 1 — Shape the claim

Take the user's statement and sharpen it into one durable claim:

- **Title**: noun phrase, states the claim directly (`"FK locks block writes during backfill"`, not `"how to handle locks"`).
- **`diataxis`**: `explanation` for factual claims; `how-to` for procedural know-how; `reference` for catalogued options. Default `explanation`.
- **`confidence`**: `low | medium | high`. Default `low`. Promote to `medium` only if the user has read a primary source; `high` only if also independently verified.

Present the shaped title + diataxis + confidence to the user. Wait for confirmation or correction before writing any file.

---

## Phase 2 — Tag the learning

Tags are mechanism, not vocabulary. Use the four-facet system; values come from the vault glossary.

```bash
anvil tags list --source used --prefix domain/
anvil tags list --source used --prefix activity/
anvil tags list --source used --prefix pattern/
```

Propose tags drawn from existing values. Only invent a new tag if no existing one fits, and only after user approval. New tags must:

- Be lowercase ASCII, hyphens only.
- Have shape `<facet>/<name>` where facet is `domain | activity | pattern`.
- Be introduced via `--allow-new-facet=<facet>` on the `create` call.

Never include a `status/*` tag.

---

## Phase 3 — Create and populate

```bash
anvil create learning --title "<claim>" \
  --tags domain/<x>,activity/<x> \
  [--allow-new-facet=domain --allow-new-facet=activity] \
  --json
# capture {id, path}
anvil set learning <id> diataxis <tutorial|how-to|reference|explanation>
anvil set learning <id> confidence <low|medium|high>
```

Direct-edit the file body. The body MUST contain three H2 sections in this order:

```markdown
## TL;DR

One paragraph. The claim, why it matters.

## Evidence

What produced this claim: session observation, command output, source read.
Use wikilinks: [[session.<id>]], [[issue.<id>]], etc.

## Caveats

What would change confidence. What is still unknown. Limits of applicability.
```

Tags are seeded on `anvil create` (via `--tags` and `--allow-new-facet`). Do not hand-edit the `tags:` frontmatter block.

If a source artifact exists in the vault, backlink it:

```bash
anvil link learning <id> <source-type> <source-id>
```

---

## Phase 4 — Glossary additions

If new tags were approved in Phase 2, register them now:

```bash
anvil tags add domain/<new-name> --desc "<one-line description>"
```

---

## Phase 5 — Validate and confirm retrieval

```bash
anvil validate
anvil list learning --tags domain/<X>,activity/<Y> --diataxis <Z>
```

The new learning must appear in the retrieval query. If it doesn't, the tags are wrong — fix and re-validate.

Acknowledge with: `Recorded → <path>`. Do not propose follow-up work unless the user signals it.

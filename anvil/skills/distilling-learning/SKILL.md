---
name: distilling-learning
description: "Use to persist one or more learnings: 'let's distill', 'extract what we learned from <source>', or a mid-session claim — 'record this learning', 'save this finding', 'this works/doesn't'. Not active research or fleeting thoughts (capturing-inbox)."
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
- A single concrete claim crystallised mid-session and the user wants it persisted now: a verified diagnosis, an observed contract, a "this works / this doesn't" result. Triggers: "record this learning", "save this finding". The source is the live session itself.

## When not to use

- The source isn't crystallized yet (still actively researching) → keep working in the thread.
- The output would just restate the source — distillation requires a durable claim or piece of know-how, not a summary.
- A fleeting, unverified thought without taxonomy commitment → `capturing-inbox`.

---

## Autonomous mode (trigger-invoked, attended)

When an **attended** end-of-run trigger invokes this skill — `completing-issue`, `dispatching-issue-fleet`, `driving-build-loop`, `responding-to-pr-review` — the operator is present but must not be prompted per run. Resolve this skill's three human gates from the **compounding bar** instead of asking, and distil only what clears it.

**The bar — high-value only, default to silence.** Most runs produce *no* learning; distilling is the rare exception, not a closing ritual. The bar is one question: **can you name the specific future failure this prevents?** — the concrete wrong path a later agent takes *without* it (repeats a dead end, ships a regression, burns a cycle rediscovering a non-obvious gotcha or a hard-won working approach). If you cannot name that failure in one sentence, it does not clear the bar — stay silent.

A marginal learning is **negative value, not neutral**. It dilutes retrieval: the next agent querying learnings gets noise mixed with signal and reasons from worse context. That active downside — not mere clutter — is why the bar is high; when in doubt, do not distil. `refreshing-learnings` is a backstop for what slips through, not a licence to lower the bar.

Does **not** clear the bar (never distil these):

- A restatement of what happened, or of what the code, git history, or the issue already records.
- A one-off fact true only of this issue, with no reuse beyond it.
- General knowledge the model already holds (a language idiom, a documented API).
- A claim an existing learning already covers — sharpen that one (Phase 4), never add a near-duplicate.

Gate substitutes (do not confirm any of these with the operator):

- **Phase 2 draft-list →** pick title / `diataxis` / `confidence` from the rubric yourself. `confidence` stays `low` (unverified), so the capture reads as provisional.
- **Phase 4 dedup →** run the dedup search and let it decide: a material match is sharpened, not duplicated; only a distinct claim is created.
- **Phase 3 / 6 tags & glossary →** reuse existing vocabulary only, and reach for an existing `pattern/*` value so the capture stays cross-domain-retrievable. Never invent a new facet or glossary term autonomously — if only a novel tag would fit, distil without it and leave the vocabulary to a human.

**Attended-only.** An unattended / dispatched trigger (fleet worker, headless orchestrator) does **not** enter this mode — no human is present to see or override what lands, so it stages candidates for later human harvest, unchanged ([[learning.unattended-loops-make-unattended-mistakes-independent-verifi]]). Explicit operator invocation ("let's distil") runs the interactive gates below as written.

---

## Phase 1 — Identify the source

A named external artifact is **optional**. When the claim crystallised in-session, the source is the live conversation (the **Reflection** row) — no thread, plan, or transcript is required. Otherwise confirm one of the following with the user, then read the relevant files:

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

Each learning crystallizes **one** claim or **one** piece of know-how. Two related claims become two learnings — never bundle.

Draft a list of candidate learnings. For each, pick:

- **Title** — the claim, phrased as a noun phrase (`"FK locks block writes during backfill"`, not `"how to handle locks"`).
- **`diataxis`** — `tutorial | how-to | reference | explanation`. Default `explanation` for claims; `how-to` for procedural know-how; `reference` for catalogued options; `tutorial` only for end-to-end teaching material.
- **`confidence`** — `low | medium | high`. Default `low`. Promote to `medium` only if backed by a primary source the user has read; `high` only if also independently verified (replicated, peer-reviewed, run in production).

**Gate:** present the draft list (titles + `diataxis` + `confidence`) to the user. User prunes/edits before any file is written.

---

## Phase 3 — Tag each learning

Tags are **mechanism**, not vocabulary. Use the four-facet system; the actual values come from `_meta/glossary.md`.

```bash
anvil tags list --source used --prefix domain/      # values currently in vault
anvil tags list --source used --prefix activity/    # values currently in vault
anvil tags list --source used --prefix pattern/     # values currently in vault
```

**The `pattern/*` facet.** `domain/*` tags the subject, `activity/*` the verb — neither reaches a *reusable implementation pattern* (a write-through invariant, an atomic-transition rule) that recurs across unrelated subjects. Tag such a learning `pattern/*` so an agent in a different `domain/*` finds it; a learning bound to one subject takes none. Draw the value from the seeded vocabulary (`anvil tags list --source defined --prefix pattern/`), reusing before inventing.

For each learning, propose tags drawn from existing values first. Only invent a new tag if no existing one fits — and only after the user approves it. New tags must:

- Be lowercase ASCII, hyphens only (no spaces, no underscores, no caps).
- Have shape `<facet>/<name>` where facet is one of `domain | activity | pattern`. (`status/` is forbidden — status is a frontmatter field.)
- Be introduced by passing `--allow-new-facet=<facet>` on the `create` call. (Glossary seeding via `anvil tags add <facet>/<name> --desc "..."` is a Phase 5 follow-up; it does not bypass the novelty gate on its own.)

Never include a `status/*` tag.

---

## Phase 4 — Dedup check

Before creating any learning, search the vault for existing learnings that cover the same claim. This prevents near-duplicate accrual that would need manual dedup later.

For each draft learning, run:

```bash
anvil list learning --search "<key terms from the title / claim>"
```

Pick one to three distinctive nouns or phrases from the draft title. If results appear:

- Read the top match (`anvil show learning <id>`).
- If it materially covers the same claim: propose **sharpening the existing learning** (edit its TL;DR / Evidence / Caveats) rather than creating a new one.
- If it is related but distinct: proceed to create; note the existing learning in the new one's Evidence section and link via `anvil link`.
- If no close match: proceed to create.

**Gate:** surface the candidates + the decision; the user decides whether to merge/sharpen the existing learning or proceed. Never auto-block or auto-merge.

---

## Phase 5 — Create + populate each approved learning

For each approved learning, in order:

```bash
anvil create learning --title "<claim>" \
  --tags domain/<x>,activity/<x> \
  [--allow-new-facet=domain --allow-new-facet=activity] \
  --json
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

Tags are seeded on `anvil create` above (passing `--tags` and, for novel values, `--allow-new-facet`). Don't hand-edit the `tags:` frontmatter block — that bypasses the novelty gate. Body edits only.

Backlink the source:

```bash
anvil link learning <id> <source-type> <source-id>
```

---

## Phase 6 — Glossary additions

If new tags were approved in phase 3, append them now (one per new tag):

```bash
anvil tags add domain/<new-name> --desc "<one-line description>"
```

If the learning introduces a term that deserves a canonical definition — a new concept the vault vocabulary should carry — add it to the glossary. Author-gate this: only add when the term and gloss are confirmed, not auto-extracted.

```bash
anvil glossary add <term> --desc "<one-line definition>"
# idempotent; use --update to overwrite an existing definition
```

Same commit as the learning files.

---

## Phase 7 — Source aftermath

| Source kind | Aftermath |
|---|---|
| Thread | Ask user: `closed | paused | stay open`. If `closed` and active, run `anvil thread deactivate`. Apply via `anvil set thread <id> status <state>`. |
| Plan | None. |
| Transcript | None. |
| Reflection | None. |

Session folder hygiene (raw → distilled) is **not** this skill's concern — it belongs to vault GC.

---

## Phase 8 — Validate

```bash
anvil validate
```

This checks:
- Schema correctness on every learning frontmatter.
- Body contains `## TL;DR / ## Evidence / ## Caveats` in order.
- Tags are lowercase-ASCII-hyphen.
- Required `domain/<x>` and `activity/<x>` facets present (per schema).

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
- Open a new thread on a follow-up subquestion → `opening-thread`.
- Surface project work the distillation revealed → `writing-issue`.
- Extract a methodology lesson into a skill → `extracting-skill-from-session` (orthogonal track).

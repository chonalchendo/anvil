---
name: extracting-skill-from-session
description: Use when a workflow just ran end-to-end and the user wants it captured as a reusable skill, or says "extract a skill" or "turn this into a skill". Do NOT use for knowledge or refresh — try anvil:researching-domain or anvil:synthesizing-knowledge-skill.
license: MIT
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: extracting-skill-from-session
  vault_type: skill
  skill_type: workflow
  side: meta
  authored_via: hand-authored
  confidence: low
  status: in-use
---

# Extracting Skill From Session

A working in-context workflow is *evidence*. This skill turns evidence into a reusable workflow SKILL.md: the user just did the activity, iterated until it worked, and now wants the load-bearing pattern crystallized — without the session-specific noise around it.

## EXTRACT ONLY FROM A WORKFLOW THAT WAS RUN END-TO-END AND WORKED

Skills are hypotheses about recurring patterns, validated by use. Extracting from a half-completed or speculative session produces a skill that codifies what was *attempted* rather than what *worked* — a worse hypothesis than no skill at all, because future invocations will follow it confidently down the wrong path.

If the activity has not actually completed successfully, stop and tell the user:

> The workflow I'd be extracting hasn't run end-to-end successfully yet. Skills capture what worked, not what was planned. Let's finish the activity first, then come back to extract.
>
> Wait for the user's response.

There is no "draft now, validate later" exception. A draft anchored to a non-working run misleads every invocation until it is rewritten — at which point the draft has been net-negative.

## When this is the wrong skill

The agent fires this skill from the description's trigger contract. If the user's actual intent is shaped differently, name a better skill in plain prose and stop.

| User intent | Right skill |
|---|---|
| Capture *principles* the user has been applying repeatedly | `anvil:synthesizing-knowledge-skill` (if learnings already exist in the vault) |
| Bootstrap a knowledge skill on a topic researched but not yet practiced | `anvil:researching-domain` |
| Refresh an existing knowledge skill with new vault learnings | `anvil:synthesizing-knowledge-skill` |
| Crystallize a workflow that just ran successfully | this skill |

The shape distinction is real. Workflow skills encode a sequence with phases that depend on each other; the agent must read the body in full because the description deliberately omits the steps. Knowledge skills encode principles, patterns, and gotchas applied as judgment; the description is *pushy* because under-triggering is the failure mode.

If the workflow shape is unclear, ask:

> Are you describing a sequence of phases that depend on each other (workflow), or a set of principles and gotchas you keep reaching for (knowledge)?
>
> Wait for the user's response.

## Phases

### 1. Confirm the run was successful and the activity recurs

Two questions, separately. The combined answers determine whether to proceed.

> Did the workflow finish end-to-end successfully? If a colleague picked up mid-flight, where would the handoff land?
>
> Wait for the user's response.

> Will you do this again — at least quarterly, ideally more often? Skills earn their cost only by being reused.
>
> Wait for the user's response.

Branch on the answers:

| Ran end-to-end? | Recurring? | Action |
|---|---|---|
| No | (either) | Iron Law refusal — return to the gate above. Stop. |
| Yes | No | Propose capturing the pattern as a vault learning under `~/anvil-vault/20-learnings/` instead. Name it, offer to draft it. Do not produce a skill. Stop. |
| Yes | Yes | Proceed to phase 2. |

### 2. Distill the load-bearing pattern

Work through the source material with the user. The source can be the just-concluded conversation, a saved transcript file the user points to, or the user's recall of an activity completed in a prior session. The shape of the workflow is what matters; the freshness of the evidence does not. Where recall is fuzzy, ask targeted questions to fill the gaps before continuing.

Separate the source into three distinct things:

- **The pattern that worked** — the sequence of phases, ordering constraints, validation gates between phases. This becomes the body.
- **Course corrections the user had to insist on** — moments where the agent took a wrong turn and the user redirected. Each correction is a load-bearing instruction in disguise: future invocations will repeat the same mistake unless the skill names it explicitly.
- **Session-specific noise** — file paths, project names, one-time decisions, ad-hoc improvisations. These get stripped.

Produce a phase outline first. Each phase has a name, the agent's action, the validation that confirms the phase succeeded, and (where applicable) the user gate that follows. Show the outline to the user before drafting any SKILL.md content:

> Here's the phase outline I extracted. Are these the load-bearing phases? Anything I labelled noise that should stay, or anything I kept that's actually session-specific?
>
> Wait for the user's response.

### 3. Draft the trigger contract

The description is a *trigger contract*: its job is discrimination, firing on the right contexts and not on adjacent ones. Workflow descriptions are **triggers-only** — never summarize what the skill does. Descriptions that summarize a workflow cause the agent to follow the description and skip the body, missing required steps.

Constraints:

- Third person. Lead with `Use when…` or a verb-first phrase.
- ≤250 characters practical (Claude Code listing truncation); 1024 chars hard.
- At least one literal trigger phrase (`mentions "X"`) for explicit-invocation paths.
- Negative triggers naming plausibly-overlapping sibling skills (always namespace-qualified: `anvil:other-skill`).
- No XML angle brackets anywhere.

Show the draft trigger contract to the user before writing the body:

> Here's the trigger contract I'd register. Reads correctly? Anything missing from the positive triggers, any sibling skill I should add to the negative list?
>
> Wait for the user's response.

### 4. Author the SKILL.md

**REQUIRED SUB-SKILL:** Use superpowers:writing-skills

That sub-skill owns the universal form: frontmatter spec, naming, descriptions vs workflow summaries, code-vs-flowchart usage, the TDD-for-skills loop, anti-patterns. Hand it the phase outline from step 2 and the trigger contract from step 3, and let it produce the SKILL.md.

Anvil layers three overlays on top that `superpowers:writing-skills` does not enforce. Apply each:

- **Anvil metadata block** under `metadata:` in the frontmatter:
  ```yaml
  metadata:
    vault_id: <skill-name>
    vault_type: skill
    skill_type: workflow
    side: user
    authored_via: extracting-skill-from-session
    confidence: low
    status: in-use
    created: <today>
    tags: [type/skill, ...]
  ```

- **Body length limits**: target ≤200 lines, hard cap ≤500. Beyond 200 lines, push reference content to `references/`. CI fails the build at 500.

- **Namespace-qualified handoffs**: when the produced skill references another skill, write `**REQUIRED SUB-SKILL:** Use anvil:<name>` (or `superpowers:<name>`, etc.) rather than the bare name. The namespace prefix is mandatory; CI warns when it is missing. `superpowers:writing-skills` already teaches the `**REQUIRED SUB-SKILL:**` form; Anvil's contribution is the namespace discipline.

**Confidence progression.** A fresh extraction ships at `confidence: low`. One successful session is necessary but not sufficient evidence — a single run can't tell you whether you captured the right pattern, the wrong pattern phrased generically, or a pattern that won't recur. Confidence is bumped manually as evidence accumulates:

- `low` → `medium`: the skill has fired on three or more real activities and produced useful results each time.
- `medium` → `high`: the skill has been refreshed via `anvil:synthesizing-knowledge-skill` (or hand-refresh) incorporating accumulated learnings from `~/anvil-vault/20-learnings/`.

Agents weight skill content by confidence during synthesis. Keeping the field honest matters.

### 5. Save to the vault

Confirm the destination path with the user before writing:

> About to save the skill to `~/anvil-vault/40-skills/<skill-name>/SKILL.md`. OK to write?
>
> Wait for the user's response.

Write the SKILL.md to that path. If the user is contributing the skill back to Anvil's bundled methodology rather than keeping it personal, the destination is the project's `skills/<skill-name>/SKILL.md` instead and `side: user` becomes `side: meta` or `side: execution` — surface this only if the user signals it.

### 6. Validate before declaring done

Run Anvil's CI checks against the new skill (`quick_validate.py`, body-length, namespace-handoff, ALL-CAPS proliferation, `# prettier-ignore` presence, negative-trigger presence). Surface every warning to the user. Validation failures must be fixed before the skill is considered shipped — silent registration breakage from frontmatter drift is the documented #2 failure mode at scale.

The skill is provisional from this point. The confidence-progression criteria above are the gate for bumping it; reuse is the only thing that earns the bump.

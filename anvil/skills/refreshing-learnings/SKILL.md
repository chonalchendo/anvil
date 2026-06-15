---
name: refreshing-learnings
description: "Use to age existing learnings against the current codebase — keep / update / consolidate / replace / delete — and drive status draft→verified→stale→retracted. Triggers: 'refresh learnings', 'are these still true', 'age the learnings'. Not for creating a new learning (distilling-learning)."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: refreshing-learnings
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-06-15
  updated: 2026-06-15
  tags: [type/skill, activity/refresh]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Refreshing Learnings

Learnings decay: code moves, a claim stops holding, two notes converge. This skill ages the existing corpus so retrieval stays trustworthy — the read-side counterpart to [[distilling-learning]], which only ever creates.

The deterministic drift signal runs without you (`anvil refresh learnings`, wired into vault-hygiene): a learning whose `related:` wikilink targets a moved or deleted artifact is auto-transitioned to `stale`. This skill drives the judgement calls that the signal can't: is a still-resolving learning actually true, should two be merged, does one need a rewrite.

## When this skill runs

- The user wants to audit learnings against current reality: "refresh learnings", "are these still true", "age the learnings".
- After a large refactor/deletion wave, to catch claims the code outran.

## When not to use

- Persisting a *new* claim or finding → `distilling-learning`.
- A fleeting thought with no taxonomy commitment → `capturing-inbox`.

---

## Phase 1 — Run the deterministic pass

```bash
anvil refresh learnings --json    # auto-stales learnings with a missing related target
```

Read its `transitioned[]`: each entry names a learning and the `related` targets that vanished. These are already `stale` on disk — your job is the verdict (Phase 3), not re-detecting them.

## Phase 2 — Surface the live corpus

```bash
anvil list learning --json        # status, tags, confidence per learning
```

Prioritise `draft` and `verified` learnings (those the deterministic pass left untouched). For each, read its `## TL;DR` / `## Evidence` and check the claim against the **current** codebase — the files, commands, or behaviours it cites.

## Phase 3 — Decide per learning

| Verdict | When | Action |
|---|---|---|
| **keep** | Still true and well-scoped | `anvil set learning <id> status verified` (promote a confirmed draft) |
| **update** | Core claim holds, details drifted | Edit the body; bump `updated`; re-`verified` |
| **consolidate** | Two+ learnings say one thing | Merge into the strongest; `retracted` the rest with a `related` pointer to the survivor |
| **replace** | Superseded by a newer claim | `retracted`; distil the replacement via `distilling-learning` |
| **delete** | Never load-bearing; noise | Remove the file (vault hygiene) |
| **stale** | Claim no longer holds, no replacement yet | `anvil set learning <id> status stale` |

Status is driven by the generic field edit — there is no `transition` verb for learnings:

```bash
anvil set learning <id> status <verified|stale|retracted>
```

Gate the destructive verdicts (consolidate / replace / delete) on the user before acting.

## Phase 4 — Validate

```bash
anvil validate
```

Fix any frontmatter/body failures and re-run until clean.

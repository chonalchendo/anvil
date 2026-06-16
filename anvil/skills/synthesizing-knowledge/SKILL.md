---
name: synthesizing-knowledge
description: "Use when vault learnings should roll up into a reusable skill update. Triggers: 'synthesize learnings', 'update skill from learnings', 'refresh skill body'. Not for a single new learning (distilling-learning) or research bootstrapping (researching)."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: synthesizing-knowledge
  vault_type: skill
  skill_type: workflow
  side: meta
  authored_via: hand-authored
  confidence: low
  status: in-use
  created: 2026-06-16
  updated: 2026-06-16
  tags: [type/skill, activity/synthesis]
---

# Synthesizing Knowledge

Takes a cluster of related vault learnings and rolls them up into a generalized, reusable artifact — either an updated knowledge skill body or a new skill draft. This is the **learnings → generalization** tier: where the compounding curve steepens.

A generalization earns its place only when the cluster passes a minimum-evidence bar. One or two learnings are coincidence; three or more verified learnings with the same tag cluster are a pattern.

## When this skill runs

- The user has a named tag cluster or topic and wants to know if a skill (new or existing) should be updated.
- An existing knowledge skill is stale — accumulated vault learnings have outrun it.
- A pattern has emerged across enough learnings that a new skill hypothesis is warranted.

## When not to use

- Fewer than three verified learnings in the cluster → distil more first (`distilling-learning`).
- The target output is capturing a *new* single observation → `distilling-learning`.
- The topic has no vault learnings at all — bootstrapping from external research → `researching`.
- The user wants to age/prune *existing* learnings for freshness → `refreshing-learnings`.

---

## Phase 1 — Identify the cluster

Ground the retrieval in canonical tags before querying. Synonyms return nothing.

```bash
anvil tags list --source used --prefix domain/
anvil tags list --source used --prefix activity/
```

Pick the canonical `domain/<x>` and `activity/<y>` values that fit the topic, then query each confidence tier separately (`--confidence` is exact-match; `--confidence high,medium` matches nothing):

```bash
anvil list learning --tags domain/<x>,activity/<y> --confidence high --json
anvil list learning --tags domain/<x>,activity/<y> --confidence medium --json
anvil list learning --tags domain/<x>,activity/<y> --confidence low --json
```

Collect the result sets. The link-graph is the highest-precision secondary signal — any learning whose `related:` wikilinks point at the same files the skill will touch should be included even if it doesn't share every tag. Read TL;DRs of all candidates; drill into Evidence + Caveats only for high-confidence or contested ones.

**Gate:** if the combined verified (`high` + `medium`) count is fewer than three, surface the gap:

> Only N verified learnings found in this cluster. Synthesis at this point will produce a low-evidence draft. Continue anyway, or distil more first?
>
> Wait for the user's response.

## Phase 2 — Diff new learnings against the existing skill

If a target skill already exists:

```bash
anvil show skill <name>
```

Map each learning against the skill body:

- **Covered and still accurate** → no change needed for that section.
- **Covered but contradicted** → flag as a conflict; surface evidence + caveats to user.
- **Novel — not covered** → candidate for a new section or updated bullet.
- **Covered and now stale** → candidate for removal or softening.

For a net-new skill there is no body to diff; proceed directly to Phase 3.

Present the diff summary to the user before writing:

> Diff summary: N covered-accurate, M novel, K conflicted, J stale. Proposed changes: [list]. Proceed?
>
> Wait for the user's response.

## Phase 3 — Propose the synthesis

Draft the updated (or new) skill body section by section. Constraints:

- Knowledge skill bodies use **reference-with-principles** shape: Philosophy → Patterns → Gotchas / Anti-patterns.
- Target ≤200 lines total (frontmatter included); hard cap 500.
- Each claim must trace to at least one learning (cite by `[[learning.<id>]]` in a comment or inline).
- Conflicted claims are marked `[CONFLICT: learning.<id> vs learning.<id>]` — do not resolve silently; surface to user.

Show the full draft to the user:

> Here is the proposed skill body. Accept, iterate, or abort?
>
> Wait for the user's response.

## Phase 4 — Write the artifact

**For an existing skill:**

Edit the skill body in place. If the skill lives in Anvil's bundled set (`anvil/skills/<name>/SKILL.md`), the change will be embedded in the next binary. If it lives in the user vault (`~/anvil-vault/40-skills/<name>/SKILL.md`), edit there.

Update `metadata.updated` to today and bump `metadata.authored_via` to `synthesizing-knowledge`.

**For a new skill:**

Confirm the destination:

> About to write to `~/anvil-vault/40-skills/<name>/SKILL.md`. OK?
>
> Wait for the user's response.

The new skill ships at `confidence: low` regardless of the source learnings' confidence — generalization is a hypothesis, not a proof. Set `status: in-use`; a populated `source_learnings` already distinguishes it from a research-bootstrapped (`from-research-only`) draft.

Populate `metadata.source_learnings` with the learning IDs that drove the synthesis:

```yaml
metadata:
  source_learnings: [learning.<id1>, learning.<id2>, ...]
```

## Phase 5 — Validate

```bash
anvil validate skill <name>
```

Walk the checklist from `extracting-skill-from-session` Phase 6 (frontmatter keys, name regex, description ≤250 chars, body line count, one Iron Law max, sibling negative triggers). Fix failures; re-run until clean.

## Phase 6 — Back-link learnings

Each learning that contributed to the synthesis should point at the skill:

```bash
anvil link learning <id> skill <skill-name>
```

This closes the loop: the skill is now reachable via the learnings that produced it, and the learnings are traceable through it.

## Hand-off

Possible next moves:

- Run the skill on real work and distil gaps as new learnings → `distilling-learning`.
- The skill has now accumulated three or more real-use cycles → bump `confidence: medium` manually.
- A topic was missing canonical tags → `anvil tags add domain/<name> --desc "..."`.
- Stale learnings surfaced during the diff → `refreshing-learnings`.
